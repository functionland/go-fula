package ping

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	FxPingProtocolID = "/fx.land/ping/0.0.1"

	actionPing = "ping"
)

var (
	_ Ping = (*FxPing)(nil)

	log = logging.Logger("fula/ping")
)

type (
	FxPing struct {
		*options
		h host.Host
		s *http.Server
		c *http.Client //libp2p client

		bufPool *sync.Pool
		reqPool *sync.Pool

		pingedPeers     map[peer.ID]int
		pingedPeersLock sync.RWMutex

		started bool
		mu      sync.RWMutex
	}
)

func NewFxPing(h host.Host, o ...Option) (*FxPing, error) {
	opts, err := newOptions(o...)
	if err != nil {
		return nil, err
	}
	pn := &FxPing{
		options: opts,
		h:       h,
		s:       &http.Server{},
		c: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					pid, err := peer.Decode(strings.TrimSuffix(addr, ".invalid:80"))
					if err != nil {
						return nil, err
					}
					return gostream.Dial(ctx, h, pid, FxPingProtocolID)
				},
			},
		},
		pingedPeers: make(map[peer.ID]int),
		bufPool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
		reqPool: &sync.Pool{
			New: func() interface{} {
				return new(http.Request)
			},
		},
	}
	return pn, nil
}

func (pn *FxPing) Start(ctx context.Context) error {
	pn.mu.Lock()
	if pn.started {
		pn.mu.Unlock()
		return errors.New("ping already started")
	}

	listen, err := gostream.Listen(pn.h, FxPingProtocolID)
	if err != nil {
		return err
	}
	pn.s.Handler = http.HandlerFunc(pn.serve)
	log.Debug("called wg.Add in ping start")
	pn.wg.Add(1)
	go func() {
		log.Debug("called wg.Done in Start ping")
		defer pn.wg.Done()
		defer log.Debug("Start ping go routine is ending")
		pn.s.Serve(listen)
	}()
	pn.started = true
	pn.mu.Unlock()

	return nil
}

func (pn *FxPing) serve(w http.ResponseWriter, r *http.Request) {
	from, err := peer.Decode(r.RemoteAddr)
	if err != nil {
		log.Errorw("cannot parse remote addr as peer ID", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	action := path.Base(r.URL.Path)
	if !pn.authorized(from, action) {
		log.Errorw("rejected duplicate request", "from", from, "action", action)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	switch action {
	case actionPing:
		log.Debugf("Received ping request on server from %s", from.String())
		pn.handlePing(from, w, r)
	default:
		http.Error(w, "", http.StatusNotFound)
	}
}

func (pn *FxPing) handlePing(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("from", from)

	// Check if the Content-Length exceeds 1 KB (1 * 1024 bytes)
	if r.ContentLength > 1*1024 {
		log.Errorf("request body too large: %v bytes", r.ContentLength)
		http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
		return
	} else if r.ContentLength <= 0 {
		log.Error("request body size is unknown and could be too large")
		http.Error(w, "Length Required", http.StatusLengthRequired)
		return
	}

	// Decode the JSON request into the PingRequest struct
	var req PingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf("cannot parse request body: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Base64 decode the data
	decodedData, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		log.Errorf("failed to decode base64 data: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Re-encode the data to base64 for the response
	encodedData := base64.StdEncoding.EncodeToString(decodedData)

	// Create the PingResponse with the same ID and the re-encoded data
	resp := PingResponse{
		ID:   req.ID,
		Data: encodedData,
	}

	// Use a buffer from the pool to write the JSON response
	buf := pn.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer pn.bufPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(resp); err != nil {
		log.Errorf("failed to encode response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set the appropriate headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))

	// Write header once all headers are set
	w.WriteHeader(http.StatusOK)

	// Write the buffer to the response writer
	if _, err := buf.WriteTo(w); err != nil {
		log.Errorf("failed to write response: %v", err)
		return
	}
}

func (pn *FxPing) authorized(pid peer.ID, action string) bool {
	switch action {
	case actionPing:
		pn.pingedPeersLock.Lock()
		defer pn.pingedPeersLock.Unlock()

		// Get the current count of pings from this peer
		count, ok := pn.pingedPeers[pid]

		if !ok {
			// If the peer hasn't pinged before, initialize the count
			pn.pingedPeers[pid] = 1
			log.Infow("Authorizing peer for first ping", "pid", pid)
			return true
		} else {
			// If the peer has pinged before, increment the count
			pn.pingedPeers[pid] = count + 1
			// Check if the count exceeds the limit
			if count >= pn.count {
				log.Errorw("Rejecting ping request; count limit exceeded", "pid", pid, "count", count, "limit", pn.count)
				return false
			}
			log.Infow("Authorizing peer for additional ping", "pid", pid, "count", count)
			return true
		}

	default:
		return false
	}
}

func (pn *FxPing) Ping(ctx context.Context, to peer.ID) (int, int, error) {
	if pn.count <= 0 {
		log.Errorf("count must be a positive integer instead of %d", pn.count)
		return 0, 0, nil
	}

	if pn.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.ping")
	}

	var totalDuration time.Duration
	var successCount int

	for i := 0; i < pn.count; i++ {
		// Generate a unique ID for the ping request
		id := fmt.Sprintf("%s-%d", to, time.Now().UnixNano())

		// Setup a timeout for the ping operation
		pingCtx, cancel := context.WithTimeout(ctx, time.Duration(pn.timeout)*time.Second) // Adjust the timeout as needed
		defer cancel()

		startTime := time.Now()

		// Create a random-sized body within the 56 to 256 bytes range
		bodySize := rand.Intn(200) + 56 // to get a size between 56 and 256 bytes
		binaryData := make([]byte, bodySize)
		_, _ = rand.Read(binaryData) // generate a random body
		encodedData := base64.StdEncoding.EncodeToString(binaryData)

		pingRequest := PingRequest{
			ID:   id,
			Data: encodedData,
		}
		requestBody, err := json.Marshal(pingRequest)
		if err != nil {
			log.Errorf("failed to marshal request: %v for id: %d", err, id)
			continue
		}

		req, err := http.NewRequestWithContext(pingCtx, http.MethodPost, "http://"+to.String()+".invalid/"+actionPing, bytes.NewReader(requestBody))
		if err != nil {
			log.Errorf("failed to create request: %v for id: %d", err, id)
			continue // Skip to the next iteration on error
		}

		resp, err := pn.c.Do(req)
		if err != nil {
			log.Errorf("request failed: %v for id: %d", err, id)
			continue // Skip to the next iteration on error
		}
		defer resp.Body.Close()

		// Read and log the raw response body
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("failed to read response body: %v for id: %s", err, id)
			continue
		}

		var pingResponse PingResponse
		err = json.Unmarshal(responseBody, &pingResponse)
		if err != nil {
			log.Errorf("failed to decode response: %v for id: %s", err, id)
			continue
		}

		// Check if the response's ID matches the request's ID
		if pingResponse.ID != id {
			log.Errorf("response ID (%s) does not match request ID (%s)", pingResponse.ID, id)
			continue
		}

		// Base64 decode the data from the response
		decodedData, err := base64.StdEncoding.DecodeString(pingResponse.Data)
		if err != nil {
			log.Errorf("failed to decode base64 data from response: %v for id: %s", err, id)
			continue
		}

		// Check if the size of the received data matches the sent data
		if len(decodedData) != len(binaryData) {
			log.Errorf("response data size (%d bytes) does not match sent data size (%d bytes) for id: %s", len(decodedData), len(binaryData), id)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Errorf("unexpected response status: %d for id: %s", resp.StatusCode, id)
			continue
		}

		// Increment successful attempt count
		successCount++

		// Calculate the time it took for the request
		totalDuration += time.Since(startTime)
	}

	// Calculate the average duration only if there were successful attempts to avoid division by zero
	var averageDuration int
	if successCount > 0 {
		avgDurationFloat := float64(totalDuration.Milliseconds()) / float64(successCount)
		averageDuration = int(avgDurationFloat)
	}

	return averageDuration, successCount, nil
}

// Get the status of Ping server
func (pn *FxPing) Status() bool {
	pn.mu.RLock()
	defer pn.mu.RUnlock()
	return pn.started
}

// clearPingedPeers safely empties the pingedPeers map
func (pn *FxPing) clearPingedPeers() {
	pn.pingedPeersLock.Lock()
	defer pn.pingedPeersLock.Unlock()

	// Reinitialize the map to clear it
	pn.pingedPeers = make(map[peer.ID]int)
}

func (pn *FxPing) StopServer(ctx context.Context) error {
	pn.mu.Lock()
	defer pn.mu.Unlock()
	// Clear the pingedPeers map
	pn.started = false
	pn.clearPingedPeers()
	return pn.s.Shutdown(ctx)
}

func (pn *FxPing) StopClient(ctx context.Context) {
	pn.c.CloseIdleConnections()
}

func (pn *FxPing) Shutdown(ctx context.Context) error {
	pn.mu.Lock()
	defer pn.mu.Unlock()
	// Clear the pingedPeers map
	pn.clearPingedPeers()
	pn.c.CloseIdleConnections()
	pn.started = false
	return pn.s.Shutdown(ctx)
}
