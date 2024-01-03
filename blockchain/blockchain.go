package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/functionland/go-fula/announcements"
	"github.com/functionland/go-fula/common"
	"github.com/functionland/go-fula/ping"
	wifi "github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

var apiError struct {
	Message     string `json:"message"`
	Description string `json:"description"`
}

const (
	FxBlockchainProtocolID = "/fx.land/blockchain/0.0.1"
	actionAuth             = "auth"
	devRelay               = "/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"
)

var (
	_ Blockchain = (*FxBlockchain)(nil)

	log = logging.Logger("fula/blockchain")
)

type Config struct {
	StoreDir string `yaml:"storeDir"`
	// other fields
}

type (
	FxBlockchain struct {
		*options
		h  host.Host
		s  *http.Server
		c  *http.Client //libp2p client
		ch *http.Client //normal http client

		authorizedPeers     map[peer.ID]struct{}
		authorizedPeersLock sync.RWMutex

		bufPool   *sync.Pool
		reqPool   *sync.Pool
		keyStorer KeyStorer

		p *ping.FxPing
		a *announcements.FxAnnouncements

		members     map[peer.ID]common.MemberStatus
		membersLock sync.RWMutex

		lastFetchTime    time.Time
		fetchInterval    time.Duration
		fetchCheckTicker *time.Ticker
		fetchCheckStop   chan struct{}

		stopFetchUsersAfterJoinChan chan struct{}
		cachedAccount               string
		isAccountCached             bool
	}
	authorizationRequest struct {
		Subject peer.ID `json:"id"`
		Allow   bool    `json:"allow"`
	}
)

func NewFxBlockchain(h host.Host, p *ping.FxPing, a *announcements.FxAnnouncements, keyStorer KeyStorer, o ...Option) (*FxBlockchain, error) {
	opts, err := newOptions(o...)
	if err != nil {
		return nil, err
	}
	bl := &FxBlockchain{
		options: opts,
		h:       h,
		p:       p,
		a:       a,
		s:       &http.Server{},
		c: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					pid, err := peer.Decode(strings.TrimSuffix(addr, ".invalid:80"))
					if err != nil {
						return nil, err
					}
					return gostream.Dial(ctx, h, pid, FxBlockchainProtocolID)
				},
			},
		},
		ch: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
			},
		},
		authorizedPeers: make(map[peer.ID]struct{}),
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
		keyStorer:      keyStorer,
		lastFetchTime:  time.Now(),
		fetchInterval:  opts.fetchFrequency * time.Hour,
		fetchCheckStop: make(chan struct{}),
	}
	if bl.authorizer != "" {
		if err := bl.SetAuth(context.Background(), h.ID(), bl.authorizer, true); err != nil {
			return nil, err
		}
	}
	bl.startFetchCheck()
	return bl, nil
}

func (bl *FxBlockchain) startFetchCheck() {
	bl.fetchCheckTicker = time.NewTicker(1 * time.Hour) // check every hour, adjust as needed

	if bl.wg != nil {
		// Increment the WaitGroup counter before starting the goroutine
		log.Debug("called wg.Add in blockchain startFetchCheck")
		bl.wg.Add(1)
	}

	go func() {
		if bl.wg != nil {
			log.Debug("called wg.Done in startFetchCheck ticker")
			defer bl.wg.Done() // Decrement the counter when the goroutine completes
		}
		defer log.Debug("startFetchCheck ticker go routine is ending")

		for {
			select {
			case <-bl.fetchCheckTicker.C:
				if time.Since(bl.lastFetchTime) >= bl.fetchInterval {
					bl.FetchUsersAndPopulateSets(context.Background(), bl.topicName, false)
					bl.lastFetchTime = time.Now() // update last fetch time
				}
			case <-bl.fetchCheckStop:
				bl.fetchCheckTicker.Stop()
				return
			}
		}
	}()
}

func (bl *FxBlockchain) Start(ctx context.Context) error {
	listen, err := gostream.Listen(bl.h, FxBlockchainProtocolID)
	if err != nil {
		return err
	}
	bl.s.Handler = http.HandlerFunc(bl.serve)
	if bl.wg != nil {
		log.Debug("called wg.Add in blockchain start")
		bl.wg.Add(1)
	}
	go func() {
		if bl.wg != nil {
			log.Debug("called wg.Done in Start blockchain")
			defer bl.wg.Done()
		}
		defer log.Debug("Start blockchain go routine is ending")
		bl.s.Serve(listen)
	}()
	return nil
}

func (bl *FxBlockchain) putBuf(buf *bytes.Buffer) {
	buf.Reset()
	bl.bufPool.Put(buf)
}
func (bl *FxBlockchain) putReq(req *http.Request) {
	*req = http.Request{}
	bl.reqPool.Put(req)
}

func prependProtocol(addr string) string {
	if strings.HasPrefix(addr, "localhost") || strings.HasPrefix(addr, "127.0.0.1") || strings.HasPrefix(addr, "192.168.") || strings.HasPrefix(addr, "10.") {
		return "http://" + addr
	}
	return "https://" + addr
}

func (bl *FxBlockchain) callBlockchain(ctx context.Context, method string, action string, p interface{}) ([]byte, int, error) {
	endpoint := prependProtocol(bl.blockchainEndPoint)
	addr := endpoint + "/" + strings.Replace(action, "-", "/", -1)

	// Use the bufPool and reqPool to reuse bytes.Buffer and http.Request objects
	buf := bl.bufPool.Get().(*bytes.Buffer)
	req := bl.reqPool.Get().(*http.Request)
	defer func() {
		bl.putBuf(buf)
		bl.putReq(req)
	}()

	preparedRequest := bl.PlugSeedIfNeeded(ctx, action, p)
	if err := json.NewEncoder(buf).Encode(preparedRequest); err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, addr, buf)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := bl.ch.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var bufRes bytes.Buffer
	_, err = io.Copy(&bufRes, resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	b := bufRes.Bytes()

	return b, resp.StatusCode, nil
}

func (bl *FxBlockchain) PlugSeedIfNeeded(ctx context.Context, action string, req interface{}) interface{} {
	switch action {
	case actionSeeded, actionAccountExists, actionPoolCreate, actionPoolJoin, actionPoolCancelJoin, actionPoolVote, actionPoolLeave, actionManifestUpload, actionManifestStore, actionManifestRemove, actionManifestRemoveStorer, actionManifestRemoveStored, actionManifestBatchStore:
		seed, err := bl.keyStorer.LoadKey(ctx)
		if err != nil {
			log.Errorw("seed is empty", "err", err)
			seed = ""
		}
		log.Debugf("seed is %s", seed)
		log.Debugf("request is %v", req)

		// Make sure we are dealing with a pointer to a struct
		val := reflect.ValueOf(req)
		if val.Kind() != reflect.Ptr || val.Elem().Kind() != reflect.Struct {
			log.Error("req is not a pointer to a struct")
			log.Errorf("Invalid req type: %T", req)
			return req
		}

		// Create a new struct based on the req's type and then set the Seed field
		reqVal := val.Elem()
		seededReqType := reflect.StructOf([]reflect.StructField{
			{
				Name: "Seed",
				Type: reflect.TypeOf(""),
				Tag:  `json:"seed"`,
			},
		})
		seededReqVal := reflect.New(seededReqType).Elem()
		seededReqVal.FieldByName("Seed").SetString(seed)

		// Create a new struct that is a combination of the request struct and the Seed field
		combinedReqType := reflect.StructOf(append(reflect.VisibleFields(reqVal.Type()), seededReqVal.Type().Field(0)))
		combinedReq := reflect.New(combinedReqType).Elem()

		// Copy the request struct fields to the new combined struct
		for i := 0; i < reqVal.NumField(); i++ {
			combinedReq.Field(i).Set(reqVal.Field(i))
		}
		// Set the Seed field
		combinedReq.FieldByName("Seed").SetString(seed)

		log.Debugf("seeded request is %v", combinedReq.Interface())
		return combinedReq.Interface()

	default:
		return req
	}
}

func (bl *FxBlockchain) serve(w http.ResponseWriter, r *http.Request) {

	from, err := peer.Decode(r.RemoteAddr)
	if err != nil {
		log.Errorw("cannot parse remote addr as peer ID: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	action := path.Base(r.URL.Path)
	if !bl.authorized(from, action) {
		log.Errorw("rejected unauthorized request", "from", from, "action", action)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	// Define a map of functions with the same signature as handleAction
	actionMap := map[string]func(peer.ID, http.ResponseWriter, *http.Request){
		actionSeeded: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionSeeded, from, w, r)
		},
		actionAccountExists: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionAccountExists, from, w, r)
		},
		actionAssetsBalance: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionAssetsBalance, from, w, r)
		},
		actionTransferToGoerli: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionTransferToGoerli, from, w, r)
		},
		actionTransferToMumbai: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionTransferToMumbai, from, w, r)
		},
		actionPoolCreate: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			//TODO: We should check if from owns the blox
			bl.handleAction(http.MethodPost, actionPoolCreate, from, w, r)
		},
		actionPoolJoin: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionPoolJoin, from, w, r)
		},
		actionPoolCancelJoin: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionPoolCancelJoin, from, w, r)
		},
		actionPoolRequests: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodGet, actionPoolRequests, from, w, r)
		},
		actionPoolList: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodGet, actionPoolList, from, w, r)
		},
		actionPoolVote: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionPoolVote, from, w, r)
		},
		actionPoolLeave: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionPoolLeave, from, w, r)
		},
		actionManifestUpload: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionManifestUpload, from, w, r)
		},
		actionManifestStore: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionManifestStore, from, w, r)
		},
		actionManifestAvailable: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionManifestAvailable, from, w, r)
		},
		actionManifestRemove: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionManifestRemove, from, w, r)
		},
		actionManifestRemoveStorer: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionManifestRemoveStorer, from, w, r)
		},
		actionManifestRemoveStored: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionManifestRemoveStored, from, w, r)
		},
		actionAuth: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAuthorization(from, w, r)
		},
		actionBloxFreeSpace: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleBloxFreeSpace(from, w, r)
		},
		actionWifiRemoveall: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleWifiRemoveall(r.Context(), from, w, r)
		},
		actionReboot: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleReboot(r.Context(), from, w, r)
		},
		actionPartition: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handlePartition(r.Context(), from, w, r)
		},
		actionDeleteFulaConfig: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleDeleteFulaConfig(r.Context(), from, w, r)
		},
		actionDeleteWifi: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleDeleteWifi(r.Context(), from, w, r)
		},
		actionDisconnectWifi: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleDisconnectWifi(r.Context(), from, w, r)
		},
		actionGetAccount: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.HandleGetAccount(r.Context(), from, w, r)
		},
		actionEraseBlData: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleEraseBlData(r.Context(), from, w, r)
		},
	}

	// Look up the function in the map and call it
	handleActionFunc, ok := actionMap[action]
	if !ok {
		http.Error(w, "", http.StatusNotFound)
		return
	}
	handleActionFunc(from, w, r)
}

func (bl *FxBlockchain) handleAction(method string, action string, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", action, "from", from)
	req := reflect.New(requestTypes[action]).Interface()
	res := reflect.New(responseTypes[action]).Interface()

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("cannot parse request body: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	//TODO: Ensure it is optimized for long-running calls
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*time.Duration(bl.timeout))
	defer cancel()
	response, statusCode, err := bl.callBlockchain(ctx, method, action, req)
	if err != nil {
		log.Error("failed to call blockchain: %v", err)
		w.WriteHeader(statusCode)
		// Try to parse the error and format it as JSON
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(response, &errMsg); jsonErr != nil {
			// If the response isn't JSON or can't be parsed, use a generic message
			errMsg = map[string]interface{}{
				"message":     "An error occurred",
				"description": err.Error(),
			}
		}
		json.NewEncoder(w).Encode(errMsg)
		return
	}
	// If status code is not 200, attempt to format the response as JSON
	if statusCode != http.StatusOK {
		w.WriteHeader(statusCode)
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(response, &errMsg); jsonErr == nil {
			// If it's already a JSON, write it as is
			w.Write(response)
		} else {
			// If it's not JSON, wrap the response in the expected format
			errMsg = map[string]interface{}{
				"message":     "Error",
				"description": string(response),
			}
			json.NewEncoder(w).Encode(errMsg)
		}
		return
	}
	w.WriteHeader(http.StatusAccepted)
	err1 := json.Unmarshal(response, &res)
	if err1 != nil {
		log.Error("failed to format response: %v", err1)
	}

	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Error("failed to write response: %v", err)
	}
}

func (bl *FxBlockchain) handleAuthorization(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionAuth, "from", from)
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorw("failed to read request body", "err", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var a authorizationRequest
	if err := json.Unmarshal(b, &a); err != nil {
		log.Debugw("cannot parse request body", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	bl.authorizedPeersLock.Lock()
	if a.Allow {
		bl.authorizedPeers[a.Subject] = struct{}{}
	} else {
		delete(bl.authorizedPeers, a.Subject)
	}
	bl.authorizedPeersLock.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (bl *FxBlockchain) handleBloxFreeSpace(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionBloxFreeSpace, "from", from)
	out, err := wifi.GetBloxFreeSpace()
	if err != nil {
		log.Error("failed to getBloxFreeSpace: %v", err)
		out = wifi.BloxFreeSpaceResponse{
			DeviceCount:    0,
			Size:           0,
			Used:           0,
			Avail:          0,
			UsedPercentage: 0,
		}
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) handleEraseBlData(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionEraseBlData, "from", from)
	out := wifi.EraseBlData(ctx)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) handleWifiRemoveall(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionWifiRemoveall, "from", from)
	out := wifi.WifiRemoveall(ctx)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) handleReboot(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionReboot, "from", from)
	out := wifi.Reboot(ctx)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) handlePartition(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionPartition, "from", from)
	out := wifi.Partition(ctx)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) handleDeleteFulaConfig(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionDeleteFulaConfig, "from", from)
	out := wifi.DeleteFulaConfig(ctx)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) handleDeleteWifi(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionDeleteWifi, "from", from)

	// Parse the JSON body of the request into the DeleteWifiRequest struct
	var req wifi.DeleteWifiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("failed to decode request: %v", err)
		http.Error(w, "failed to decode request", http.StatusBadRequest)
		return
	}
	log.Debugw("handleDeleteWifi received", "req", req)

	out := wifi.DeleteWifi(ctx, req)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}
func (bl *FxBlockchain) handleDisconnectWifi(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionDisconnectWifi, "from", from)

	// Parse the JSON body of the request into the DeleteWifiRequest struct
	var req wifi.DeleteWifiRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("failed to decode request: %v", err)
		http.Error(w, "failed to decode request", http.StatusBadRequest)
		return
	}
	log.Debugw("handleDisconnectWifi received", "req", req)

	out := wifi.DisconnectNamedWifi(ctx, req)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) SetAuth(ctx context.Context, on peer.ID, subject peer.ID, allow bool) error {
	// Check if auth is for local host; if so, handle it locally.
	if on == bl.h.ID() {
		bl.authorizedPeersLock.Lock()
		if allow {
			bl.authorizedPeers[subject] = struct{}{}
		} else {
			delete(bl.authorizedPeers, subject)
		}
		bl.authorizedPeersLock.Unlock()
		return nil
	}
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}
	r := authorizationRequest{Subject: subject, Allow: allow}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+on.String()+".invalid/"+actionAuth, &buf)
	if err != nil {
		return err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return err
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return nil
	}
}

func (bl *FxBlockchain) authorized(pid peer.ID, action string) bool {
	if bl.authorizer == "" {
		// If no authorizer is set allow all.
		return true
	}
	switch action {
	case actionBloxFreeSpace, actionAssetsBalance, actionEraseBlData, actionWifiRemoveall, actionReboot, actionPartition, actionDeleteWifi, actionDisconnectWifi, actionDeleteFulaConfig, actionGetAccount, actionSeeded, actionAccountExists, actionPoolCreate, actionPoolJoin, actionPoolCancelJoin, actionPoolRequests, actionPoolList, actionPoolVote, actionPoolLeave, actionManifestUpload, actionManifestStore, actionManifestAvailable, actionManifestRemove, actionManifestRemoveStorer, actionManifestRemoveStored:
		bl.authorizedPeersLock.RLock()
		_, ok := bl.authorizedPeers[pid]
		bl.authorizedPeersLock.RUnlock()
		return ok
	case actionAuth:
		return pid == bl.authorizer
	default:
		return false
	}
}

func (bl *FxBlockchain) Shutdown(ctx context.Context) error {
	bl.c.CloseIdleConnections()
	bl.ch.CloseIdleConnections()
	close(bl.fetchCheckStop)
	return bl.s.Shutdown(ctx)
}

func (bl *FxBlockchain) IsMembersEmpty() bool {
	bl.membersLock.RLock()         // Use RLock for read-only access
	defer bl.membersLock.RUnlock() // Defer the unlock operation
	return len(bl.members) == 0
}

// contains is a helper function to check if the slice contains a string
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func (bl *FxBlockchain) cleanUnwantedPeers(keepPeers []peer.ID) {
	// Convert the keepPeers slice to a map for efficient existence checks
	keepMap := make(map[peer.ID]bool)
	for _, p := range keepPeers {
		keepMap[p] = true
	}

	// Retrieve all peers from the AddrBook
	allPeers := bl.h.Peerstore().PeersWithAddrs()

	// Iterate over all peers and clear addresses for those not in keepMap
	for _, peerID := range allPeers {
		if _, found := keepMap[peerID]; !found {
			bl.h.Peerstore().ClearAddrs(peerID)
		}
	}
}
func (bl *FxBlockchain) FetchUsersAndPopulateSets(ctx context.Context, topicString string, initiate bool) error {
	// Initialize the map if it's nil
	if bl.members == nil {
		bl.members = make(map[peer.ID]common.MemberStatus)
	}

	// Store repeated method calls as variables to avoid redundant calls
	localPeerID := bl.h.ID()
	localPeerIDStr := localPeerID.String()

	log.Debug("FetchUsersAndPopulateSets is called for ", "topicString: ", topicString, " ,initiate: ", initiate)
	// Update last fetch time on successful fetch
	var keepPeers []peer.ID
	bl.lastFetchTime = time.Now()

	// Convert topic from string to int
	topic, err := strconv.Atoi(topicString)
	if err != nil {
		// Handle the error if the conversion fails
		return fmt.Errorf("invalid topic, not an integer: %s", err)
	}
	if topic <= 0 {
		log.Info("Not a member of any pool at the moment")
		return nil
	}

	// Minimize lock scope, declare a function to handle locked operations
	updateMembers := func(pid peer.ID, status common.MemberStatus, addrs []multiaddr.Multiaddr) error {
		bl.membersLock.Lock()
		defer bl.membersLock.Unlock()
		bl.members[pid] = status
		if len(addrs) > 0 {
			bl.h.Peerstore().AddAddrs(pid, addrs, peerstore.ConnectedAddrTTL)
		}
		return nil
	}

	if initiate {
		//If members list is empty we should check what peerIDs we already voted on and update to avoid re-voting
		isMembersEmpty := bl.IsMembersEmpty()
		if isMembersEmpty {
			log.Debugw("Members list is empty", "peer", localPeerIDStr)
			// Call the bl.PoolRequests and get the list of requests
			req := PoolRequestsRequest{
				PoolID: topic, // assuming 'topic' is your pool id
			}
			responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", actionPoolRequests, req)
			if err != nil {
				return fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
			}

			// Check if the status code is OK; if not, handle it as an error
			if statusCode != http.StatusOK {
				var errMsg map[string]interface{}
				if jsonErr := json.Unmarshal(responseBody, &errMsg); jsonErr == nil {
					return fmt.Errorf("unexpected response status: %d, message: %s, description: %s",
						statusCode, errMsg["message"], errMsg["description"])
				} else {
					return fmt.Errorf("unexpected response status: %d, body: %s", statusCode, string(responseBody))
				}
			}

			var poolRequestsResponse PoolRequestsResponse
			// Unmarshal the response body into the poolRequestsResponse struct
			if err := json.Unmarshal(responseBody, &poolRequestsResponse); err != nil {
				return fmt.Errorf("failed to unmarshal poolRequests response: %w", err)
			}

			// For each one check if the voted field in the response contains the localPeerIDStr and if so it means we already voted
			// Move it to members with status Unknown.
			log.Debugw("Empty members for ", "peer", localPeerID, "Received response from blockchain", poolRequestsResponse.PoolRequests)
			for _, request := range poolRequestsResponse.PoolRequests {
				if contains(request.Voted, localPeerIDStr) {
					pid, err := peer.Decode(request.PeerID)
					if err != nil {
						return err
					}

					// Create a slice to hold the multiaddresses for the peer
					var addrs []multiaddr.Multiaddr

					// Loop through the static relays and convert them to multiaddr
					for _, relay := range bl.relays {
						ma, err := multiaddr.NewMultiaddr(relay + "/p2p-circuit/p2p/" + pid.String())
						if err != nil {
							return err
						}
						addrs = append(addrs, ma)
					}

					// Add the relay addresses to the peerstore for the peer ID
					err = updateMembers(pid, common.Unknown, addrs)
					if err != nil {
						return err
					}
					keepPeers = append(keepPeers, pid)
				}
			}
		}

		log.Debugw("stored members after empty member list", "peer", localPeerID, "members", bl.members)
	}

	//Get the list of both join requests and joined members for the pool
	// Create a struct for the POST req
	req := PoolUserListRequest{
		PoolID:        topic,
		RequestPoolID: topic,
	}

	// Call the existing function to make the request
	responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", actionPoolUserList, req)
	if err != nil {
		// Handle the error from callBlockchain, including the status code for more context
		return fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
	}

	// Check if the status code is OK; if not, handle it as an error
	if statusCode != http.StatusOK {
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(responseBody, &errMsg); jsonErr == nil {
			// If the responseBody is JSON, use it in the error message
			return fmt.Errorf("unexpected response status: %d, message: %s, description: %s",
				statusCode, errMsg["message"], errMsg["description"])
		} else {
			// If the responseBody is not JSON, return it as a plain text error message
			return fmt.Errorf("unexpected response status: %d, body: %s", statusCode, string(responseBody))
		}
	}
	var response PoolUserListResponse

	// Unmarshal the response body into the response struct
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return err
	}

	// Now iterate through the users and populate the member map
	log.Debugw("Now iterate through the users and populate the member map", "peer", localPeerID, "response", response.Users)

	for _, user := range response.Users {
		pid, err := peer.Decode(user.PeerID)
		if err != nil {
			log.Errorw("Could not debug PeerID in response.Users", "user.PeerID", user.PeerID, "err", err)
		}
		account := user.Account

		if initiate {
			keepPeers = append(keepPeers, pid)
			//Check if self status is in pool request, start ping server and announce join request
			if user.PeerID == localPeerIDStr {
				log.Debugw("Found self peerID", user.PeerID)
				if user.RequestPoolID != nil {
					if !bl.p.Status() {
						log.Debugw("Found self peerID and running Ping Server now", "peer", user.PeerID)
						err = bl.p.Start(ctx)
						if err != nil {
							log.Errorw("Error when starting the Ping Server", "PeerID", user.PeerID, "err", err)
						} else {
							log.Debugw("Found self peerID and ran Ping Server and announcing pooljoinrequest now", "peer", user.PeerID)
							if bl.wg != nil {
								log.Debug("Called wg.Add in somewhere before AnnounceJoinPoolRequestPeriodically")
								bl.wg.Add(1)
							}
							go func() {
								if bl.wg != nil {
									log.Debug("called wg.Done in somewhere before AnnounceJoinPoolRequestPeriodically")
									defer bl.wg.Done() // Decrement the counter when the goroutine completes
								}
								defer log.Debug("somewhere before AnnounceJoinPoolRequestPeriodically go routine is ending")
								bl.a.AnnounceJoinPoolRequestPeriodically(ctx)
							}()
						}
					} else {
						log.Debugw("Ping Server is already running for self peerID", user.PeerID)
					}
				} else {
					log.Debugw("PeerID is already a member of the pool", user.PeerID)
				}
			}
		}

		// Determine the status based on pool_id and request_pool_id
		bl.membersLock.RLock()
		existingStatus, exists := bl.members[pid]
		localPeerStatus, localPeerExists := bl.members[localPeerID]
		bl.membersLock.RUnlock()

		var status common.MemberStatus
		if user.PoolID != nil && *user.PoolID == topic {
			status = common.Approved
		} else if user.RequestPoolID != nil && *user.RequestPoolID == topic {
			status = common.Pending
		} else {
			// Skip users that do not match the topic criteria
			continue
		}

		//if initiate {
		//Vote for any peer that has not voted already
		log.Debugw("check if vote needs to be casted", "from", localPeerID, "status", localPeerStatus, "for", pid, "status", existingStatus)
		if exists && existingStatus == common.Pending && localPeerExists && localPeerStatus == common.Approved {
			log.Debugw("Voting for peers", "pool", topicString, "from", localPeerID, "for", pid)
			err = bl.HandlePoolJoinRequest(ctx, pid, account, topicString, false)
			if err == nil {
				status = common.Unknown
			} else {
				log.Errorw("Error happened while voting", "pool", topicString, "from", localPeerID, "for", pid)
			}
		}
		//}

		if exists {
			log.Debugw("peer already exists in members", "h.ID", localPeerID, "pid", pid, "existingStatus", existingStatus, "status", status)
			if existingStatus != status && (existingStatus != common.Approved) {
				// If the user is already pending and now approved, update to ApprovedOrPending and no need to update the addrs
				if err := updateMembers(pid, status, []multiaddr.Multiaddr{}); err != nil {
					return err
				}
			} else {
				// If the user status is the same as before, there's no need to update
				log.Debugw("member exists but is not approved so no need to change status", "h.ID", localPeerID, "pid", pid, "Status", status, "existingStatus", existingStatus)
			}
		} else {
			log.Debugw("member does not exists", "h.ID", bl.h.ID(), "pid", pid)
			// If the user does not exist in the map, add them
			// Create a slice to hold the multiaddresses for the peer
			var addrs []multiaddr.Multiaddr

			// Loop through the static relays and convert them to multiaddr
			for _, relay := range bl.relays {
				fullAddr := relay + "/p2p-circuit/p2p/" + pid.String()
				log.Debugw("full relay address", "peer", bl.h.ID(), "for", pid, "fullAddr", fullAddr)
				ma, err := multiaddr.NewMultiaddr(fullAddr)
				if err != nil {
					return err
				}
				addrs = append(addrs, ma)
			}

			// Add the relay addresses to the peerstore for the peer ID

			if err := updateMembers(pid, status, addrs); err != nil {
				return err
			}
			log.Debugw("Added peer to peerstore", "h.ID", localPeerID, "pid", pid, "addrs", addrs)
		}
		if pid != localPeerID {
			//bl.h.Connect(ctx, peer.AddrInfo{ID: pid, Addrs: addrs})
			peerAddr := bl.h.Peerstore().PeerInfo(pid)
			log.Debugw("Connecting to other peer", "from", bl.h.ID(), "to", pid, "with address", peerAddr)
			err := bl.h.Connect(ctx, peerAddr)
			if err != nil {
				log.Debugw("Not Connected to peer", "from", bl.h.ID(), "to", pid, "err", err)
			} else {
				log.Debugw("OK Connected to peer", "from", bl.h.ID(), "to", pid)
			}

		}
	}

	log.Debugw("peerstore for ", "id", bl.h.ID(), "peers", bl.h.Peerstore().Peers())
	if initiate {
		bl.cleanUnwantedPeers(keepPeers)
	}

	return nil
}

func (bl *FxBlockchain) GetMemberStatus(id peer.ID) (common.MemberStatus, bool) {
	bl.membersLock.RLock()
	defer bl.membersLock.RUnlock()
	status, exists := bl.members[id]
	if !exists {
		// If the peer.ID doesn't exist in the members map, we treat it as an error case.
		return common.MemberStatus(0), false
	}
	return status, true
}

func (bl *FxBlockchain) GetMembers() map[peer.ID]common.MemberStatus {
	bl.membersLock.RLock()
	defer bl.membersLock.RUnlock()

	copy := make(map[peer.ID]common.MemberStatus)
	for k, v := range bl.members {
		copy[k] = v
	}
	return copy
}
