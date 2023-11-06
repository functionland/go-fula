package ping

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

		pingedPeers     map[peer.ID]struct{}
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
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					pid, err := peer.Decode(strings.TrimSuffix(addr, ".invalid:80"))
					if err != nil {
						return nil, err
					}
					return gostream.Dial(ctx, h, pid, FxPingProtocolID)
				},
			},
		},
		pingedPeers: make(map[peer.ID]struct{}),
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
	defer pn.mu.Unlock()
	if pn.started {
		return errors.New("ping already started")
	}
	listen, err := gostream.Listen(pn.h, FxPingProtocolID)
	if err != nil {
		return err
	}
	pn.s.Handler = http.HandlerFunc(pn.serve)
	go func() { pn.s.Serve(listen) }()
	pn.started = true
	return nil
}

func (pn *FxPing) serve(w http.ResponseWriter, r *http.Request) {
	from, err := peer.Decode(r.RemoteAddr)
	if err != nil {
		log.Debugw("cannot parse remote addr as peer ID", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	action := path.Base(r.URL.Path)
	if !pn.authorized(from, action) {
		log.Debugw("rejected duplicate request", "from", from, "action", action)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	switch action {
	case actionPing:
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
	} else if r.ContentLength < 0 {
		// ContentLength < 0 indicates that the size is unknown and potentially too large as well
		log.Error("request body size is unknown and could be too large")
		http.Error(w, "Length Required", http.StatusLengthRequired)
		return
	}

	// Decode directly into the response object to avoid extra allocations
	var req PingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Errorf("cannot parse request body: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Use a buffer from the pool to write the JSON response and set Content-Length header
	buf := pn.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer pn.bufPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(req); err != nil {
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
		// Note: It's too late to change the HTTP status code or headers if an error occurs at this point.
		return
	}
}

func (pn *FxPing) authorized(pid peer.ID, action string) bool {
	switch action {
	case actionPing:
		pn.pingedPeersLock.RLock()
		_, ok := pn.pingedPeers[pid]
		pn.pingedPeersLock.RUnlock()
		var pinged = ok
		if !pinged {
			pn.pingedPeersLock.Lock()
			pn.pingedPeers[pid] = struct{}{}
			log.Infow("Authorizing peers for ", "pid", pid)
			pn.pingedPeersLock.Unlock()
		}
		return !pinged
	default:
		return false
	}
}

func (pn *FxPing) Ping(ctx context.Context, to peer.ID, count int) (int, int, error) {
	if count <= 0 {
		return 0, 0, errors.New("count must be a positive integer")
	}

	if pn.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.ping")
	}

	var totalDuration time.Duration
	var successCount int

	for i := 0; i < count; i++ {
		// Setup a timeout for the ping operation
		pingCtx, cancel := context.WithTimeout(ctx, time.Duration(pn.timeout)*time.Second) // Adjust the timeout as needed
		defer cancel()

		startTime := time.Now()

		// Create a random-sized body within the 56 to 256 bytes range
		bodySize := rand.Intn(200) + 56 // to get a size between 56 and 256 bytes
		requestBody := make([]byte, bodySize)
		_, _ = rand.Read(requestBody) // generate a random body

		req, err := http.NewRequestWithContext(pingCtx, http.MethodPost, "http://"+to.String()+".invalid/"+actionPing, bytes.NewReader(requestBody))
		if err != nil {
			log.Errorf("failed to create request: %v", err)
			continue // Skip to the next iteration on error
		}

		resp, err := pn.c.Do(req)
		if err != nil {
			log.Errorf("request failed: %v", err)
			continue // Skip to the next iteration on error
		}
		defer resp.Body.Close()

		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("failed to read response body: %v", err)
			continue // Skip to the next iteration on error
		}

		if len(responseBody) != len(requestBody) {
			log.Errorf("response size (%d bytes) does not match request size (%d bytes)", len(responseBody), len(requestBody))
			continue // Skip to the next iteration on error
		}

		if resp.StatusCode != http.StatusOK {
			log.Errorf("unexpected response status: %d", resp.StatusCode)
			continue // Skip to the next iteration on error
		}

		// Increment successful attempt count
		successCount++

		// Calculate the time it took for the request
		totalDuration += time.Since(startTime)
	}

	// Calculate the average duration only if there were successful attempts to avoid division by zero
	var averageDuration int
	if successCount > 0 {
		averageDuration = int(totalDuration.Milliseconds() / int64(successCount))
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
	pn.pingedPeers = make(map[peer.ID]struct{})
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
