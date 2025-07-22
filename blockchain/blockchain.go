package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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
	ipfsClusterClientApi "github.com/ipfs-cluster/ipfs-cluster/api"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var apiError struct {
	Message     string `json:"message"`
	Description string `json:"description"`
}

const (
	FxBlockchainProtocolID = "/fx.land/blockchain/0.0.1"
	actionAuth             = "auth"
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

		fetchMutex sync.Mutex
		isFetching bool
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
		fetchInterval:  opts.fetchFrequency,
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
	internal := 2 * time.Minute
	bl.fetchCheckTicker = time.NewTicker(internal) // check every hour, adjust as needed

	if bl.wg != nil {
		// Increment the WaitGroup counter before starting the goroutine
		log.Debug("called wg.Add in blockchain startFetchCheck")
		bl.wg.Add(1)
	}

	// Periodic fetch is no longer needed with EVM chain integration
	// Pool membership is determined during startup and doesn't change frequently
	go func() {
		if bl.wg != nil {
			log.Debug("called wg.Done in startFetchCheck ticker")
			defer bl.wg.Done()
		}
		defer log.Debug("startFetchCheck ticker go routine is ending")

		for {
			select {
			case <-bl.fetchCheckTicker.C:
				// No-op: periodic fetching disabled for production optimization
				log.Debugw("Periodic fetch skipped - using EVM chain integration")
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

// checkHealth checks the health of the blockchain by querying the /health endpoint.
// It returns an error if the blockchain is currently syncing.
func (bl *FxBlockchain) checkHealth(ctx context.Context) error {
	// Removed Health check as it is not needed
	return nil
}

func (bl *FxBlockchain) callBlockchain(ctx context.Context, method string, action string, p interface{}) ([]byte, int, error) {
	// Check blockchain health before proceeding
	if err := bl.checkHealth(ctx); err != nil {
		return nil, http.StatusFailedDependency, err // Use 424 as the status code for a syncing blockchain
	}

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

// ChainConfig represents the configuration for an EVM chain
type ChainConfig struct {
	Name      string
	ChainID   int64
	RPC       string
	BackupRPC string
	Contract  string
}

// GetChainConfigs returns the available chain configurations
func GetChainConfigs() map[string]ChainConfig {
	return map[string]ChainConfig{
		"base": {
			Name:      "base",
			ChainID:   8453,
			RPC:       "https://base-rpc.publicnode.com",
			BackupRPC: "https://1rpc.io/base",
			Contract:  "0xf293A6902662DcB09E310254A5e418cb28D71b6b",
		},
		"skale": {
			Name:     "skale",
			ChainID:  2046399126,
			RPC:      "https://mainnet.skalenodes.com/v1/elated-tan-skat",
			Contract: "0xf293A6902662DcB09E310254A5e418cb28D71b6b",
		},
	}
}

// callEVMChain makes calls to EVM-compatible chains (Base/Skale) using JSON-RPC
func (bl *FxBlockchain) callEVMChain(ctx context.Context, chainName string, method string, params []interface{}) ([]byte, int, error) {
	chainConfigs := GetChainConfigs()
	chainConfig, exists := chainConfigs[chainName]
	if !exists {
		return nil, http.StatusBadRequest, fmt.Errorf("unsupported chain: %s", chainName)
	}

	// Prepare JSON-RPC request
	rpcRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}

	// Use the bufPool and reqPool to reuse bytes.Buffer and http.Request objects
	buf := bl.bufPool.Get().(*bytes.Buffer)
	req := bl.reqPool.Get().(*http.Request)
	defer func() {
		bl.putBuf(buf)
		bl.putReq(req)
	}()

	if err := json.NewEncoder(buf).Encode(rpcRequest); err != nil {
		return nil, 0, fmt.Errorf("failed to encode JSON-RPC request: %w", err)
	}

	// Try primary RPC first
	rpcURL := chainConfig.RPC
	httpReq, err := http.NewRequestWithContext(ctx, "POST", rpcURL, buf)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := bl.ch.Do(httpReq)
	if err != nil && chainConfig.BackupRPC != "" {
		// Try backup RPC if primary fails
		log.Debugw("Primary RPC failed, trying backup", "chain", chainName, "error", err)
		buf.Reset()
		if err := json.NewEncoder(buf).Encode(rpcRequest); err != nil {
			return nil, 0, fmt.Errorf("failed to encode JSON-RPC request for backup: %w", err)
		}

		httpReq, err = http.NewRequestWithContext(ctx, "POST", chainConfig.BackupRPC, buf)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create backup HTTP request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err = bl.ch.Do(httpReq)
		if err != nil {
			return nil, 0, fmt.Errorf("both primary and backup RPC failed: %w", err)
		}
	} else if err != nil {
		return nil, 0, fmt.Errorf("RPC call failed: %w", err)
	}
	defer resp.Body.Close()

	var bufRes bytes.Buffer
	_, err = io.Copy(&bufRes, resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %w", err)
	}

	return bufRes.Bytes(), resp.StatusCode, nil
}

// callEVMChainWithRetry calls EVM chain with retry logic and graceful error handling
func (bl *FxBlockchain) callEVMChainWithRetry(ctx context.Context, chainName string, method string, params []interface{}, maxRetries int) ([]byte, int, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		response, statusCode, err := bl.callEVMChain(ctx, chainName, method, params)
		if err == nil {
			return response, statusCode, nil
		}

		lastErr = err
		log.Debugw("EVM chain call failed, retrying", "chain", chainName, "attempt", attempt, "maxRetries", maxRetries, "error", err)

		if attempt < maxRetries {
			// Exponential backoff with jitter
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	return nil, 0, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func (bl *FxBlockchain) PlugSeedIfNeeded(ctx context.Context, action string, req interface{}) interface{} {
	switch action {
	case actionSeeded, actionAccountExists, actionAccountFund, actionPoolCreate, actionPoolCancelJoin, actionPoolVote, actionManifestUpload, actionManifestStore, actionManifestRemove, actionManifestRemoveStorer, actionManifestRemoveStored, actionManifestBatchUpload, actionManifestBatchStore, actionTransferToMumbai, actionTransferToGoerli:
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

func convertMobileRequestToFullRequest(mobileReq *ManifestBatchUploadMobileRequest) *ManifestBatchUploadRequest {
	manifestMetadata := make([]ManifestMetadata, len(mobileReq.Cid))
	for i, cid := range mobileReq.Cid {
		manifestMetadata[i] = ManifestMetadata{
			Job: ManifestJob{
				Work:   "Storage",
				Engine: "IPFS",
				Uri:    cid,
			},
		}
	}

	replicationFactor := make([]int, len(mobileReq.Cid))
	for i := range replicationFactor {
		replicationFactor[i] = mobileReq.ReplicationFactor
	}

	return &ManifestBatchUploadRequest{
		Cid:               mobileReq.Cid,
		PoolID:            mobileReq.PoolID,
		ReplicationFactor: replicationFactor,
		ManifestMetadata:  manifestMetadata,
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
	} else {
		log.Debugw("action: ", action, "was permitted from: ", from)
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
			bl.HandlePoolJoin(http.MethodPost, actionPoolJoin, from, w, r)
		},
		actionPoolCancelJoin: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.HandlePoolCancelJoin(http.MethodPost, actionPoolCancelJoin, from, w, r)
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
			bl.HandlePoolLeave(http.MethodPost, actionPoolLeave, from, w, r)
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
		actionManifestBatchStore: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAction(http.MethodPost, actionManifestBatchStore, from, w, r)
		},
		actionManifestBatchUpload: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			// Decode the original mobile request
			var mobileReq ManifestBatchUploadMobileRequest
			if err := json.NewDecoder(r.Body).Decode(&mobileReq); err != nil {
				log.Debug("cannot parse request body: %v", err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			// Convert to the full request format
			fullReq := convertMobileRequestToFullRequest(&mobileReq)
			bl.handleActionManifestBatchUpload(http.MethodPost, actionManifestBatchUpload, from, w, r, fullReq)
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
		actionReplicateInPool: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleReplicateInPool(http.MethodPost, actionReplicateInPool, from, w, r)
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
			// To be removed
		},
		actionEraseBlData: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleEraseBlData(r.Context(), from, w, r)
		},
		actionFetchContainerLogs: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleFetchContainerLogs(r.Context(), from, w, r)
		},
		actionChatWithAI: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleChatWithAI(r.Context(), from, w, r)
		},
		actionFindBestAndTargetInLogs: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleFindBestAndTargetInLogs(r.Context(), from, w, r)
		},
		actionGetFolderSize: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleGetFolderSize(r.Context(), from, w, r)
		},
		actionGetDatastoreSize: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleGetDatastoreSize(r.Context(), from, w, r)
		},

		// Plugin actions
		actionListPlugins: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handlePluginAction(r.Context(), from, w, r, actionListPlugins)
		},
		actionListActivePlugins: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handlePluginAction(r.Context(), from, w, r, actionListActivePlugins)
		},
		actionInstallPlugin: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handlePluginAction(r.Context(), from, w, r, actionInstallPlugin)
		},
		actionUninstallPlugin: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handlePluginAction(r.Context(), from, w, r, actionUninstallPlugin)
		},
		actionShowPluginStatus: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handlePluginAction(r.Context(), from, w, r, actionShowPluginStatus)
		},
		actionGetInstallOutput: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handlePluginAction(r.Context(), from, w, r, actionGetInstallOutput)
		},
		actionGetInstallStatus: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handlePluginAction(r.Context(), from, w, r, actionGetInstallStatus)
		},
		actionUpdatePlugin: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handlePluginAction(r.Context(), from, w, r, actionUpdatePlugin)
		},
	}

	// Look up the function in the map and call it
	handleActionFunc, ok := actionMap[action]
	if !ok {
		log.Errorw("action not found", "from", from, "action", action)
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

func (bl *FxBlockchain) handleReplicateInPool(method string, action string, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", action, "from", from)
	log.Debug("Processing replicate request")
	var req ReplicateRequest
	var res ReplicateResponse
	var poolRes []string

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("cannot parse request body: %v", err)
		http.Error(w, "cannot parse request body", http.StatusBadRequest)
		return
	}
	log.Debugw("Decoded replicate request", "req", req)

	//TODO: Ensure it is optimized for long-running calls
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*time.Duration(bl.timeout))
	defer cancel()
	response, statusCode, err := bl.callBlockchain(ctx, method, actionManifestAvailableBatch, req)
	if err != nil {
		log.Errorw("failed to call blockchain", "err", err)
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
		log.Errorw("failed to call blockchain", "statusCode", statusCode)
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

	if jsonErr := json.Unmarshal(response, &res); jsonErr != nil {
		log.Errorw("failed to call blockchain", "jsonErr", jsonErr)
		// If the response isn't JSON or can't be parsed, use a generic message
		w.WriteHeader(http.StatusFailedDependency)
		errMsg := map[string]interface{}{
			"message":     "An error occurred",
			"description": jsonErr.Error(),
		}
		json.NewEncoder(w).Encode(errMsg)
		return
	}

	log.Debugw("Received replicate response from chain", "res", res)
	if len(res.Manifests) == 0 {
		log.Errorw("no uploadable manifests could be found", "res.Manifests", res.Manifests)
		w.WriteHeader(http.StatusNoContent)
		errMsg := map[string]interface{}{
			"message":     "An error occurred",
			"description": "no uploadable manifests could be found",
		}
		json.NewEncoder(w).Encode(errMsg)
		return
	}

	poolInt, err := strconv.Atoi(bl.topicName)
	if err != nil {
		log.Errorw("failed to call blockchain poolInt", "err", err)
		w.WriteHeader(http.StatusFailedDependency)
		errMsg := map[string]interface{}{
			"message":     "An error occurred",
			"description": "endpoint is not a member of valid pool",
		}
		json.NewEncoder(w).Encode(errMsg)
		return
	}
	log.Debugw("Pool in replicate is", "poolInt", poolInt)

	if req.PoolID != poolInt {
		log.Errorw("Endpoint is not a member of requested replication pool", "req.PoolID", req.PoolID, "poolInt", poolInt)
		w.WriteHeader(http.StatusFailedDependency)
		errMsg := map[string]interface{}{
			"message":     "An error occurred",
			"description": "Endpoint is not a member of requested replication pool",
		}
		json.NewEncoder(w).Encode(errMsg)
		return
	}

	if bl.ipfsClusterApi == nil {
		log.Errorw("ipfs cluster API is nil", "bl.ipfsClusterApi", bl.ipfsClusterApi)
		w.WriteHeader(http.StatusFailedDependency)
		errMsg := map[string]interface{}{
			"message":     "An error occurred",
			"description": "ipfs cluster API is nil",
		}
		json.NewEncoder(w).Encode(errMsg)
		return
	}

	pCtx, pCancel := context.WithTimeout(r.Context(), time.Second*time.Duration(bl.timeout))
	defer pCancel()
	pinOptions := ipfsClusterClientApi.PinOptions{
		Mode: 0,
	}
	for i := 0; i < len(res.Manifests); i++ {
		c, err := cid.Decode(res.Manifests[i].Cid)
		if err != nil {
			log.Errorw("Error decoding CID:", "err", err)
			continue // Or handle the error appropriately
		}
		replicationRes, err := bl.ipfsClusterApi.Pin(pCtx, ipfsClusterClientApi.NewCid(c), pinOptions)
		if err != nil {
			log.Errorw("Error pinning CID:", "err", err)
			continue
		}
		poolRes = append(poolRes, replicationRes.Cid.Cid.String())
	}

	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(poolRes); err != nil {
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
	log.Debugw("Checking authorization", "action", action, "pid", pid, "bl.authorizer", bl.authorizer, "h.ID", bl.h.ID())
	switch action {
	case actionReplicateInPool:
		return (bl.authorizer == bl.h.ID() || bl.authorizer == "")
	case actionBloxFreeSpace, actionAccountFund, actionManifestBatchUpload, actionAssetsBalance, actionGetDatastoreSize, actionGetFolderSize, actionFindBestAndTargetInLogs, actionFetchContainerLogs, actionChatWithAI, actionEraseBlData, actionWifiRemoveall, actionReboot, actionPartition, actionDeleteWifi, actionDisconnectWifi, actionDeleteFulaConfig, actionGetAccount, actionSeeded, actionAccountExists, actionPoolCreate, actionPoolJoin, actionPoolCancelJoin, actionPoolRequests, actionPoolList, actionPoolVote, actionPoolLeave, actionManifestUpload, actionManifestStore, actionManifestAvailable, actionManifestRemove, actionManifestRemoveStorer, actionManifestRemoveStored, actionTransferToMumbai, actionListPlugins, actionListActivePlugins, actionInstallPlugin, actionUninstallPlugin, actionGetInstallStatus, actionGetInstallOutput, actionUpdatePlugin:
		bl.authorizedPeersLock.RLock()
		_, ok := bl.authorizedPeers[pid]
		bl.authorizedPeersLock.RUnlock()
		return ok
	case actionAuth:
		return pid == bl.authorizer && bl.authorizer != ""
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

func (bl *FxBlockchain) checkIfUserHasOpenPoolRequests(ctx context.Context, topicString string) (bool, error) {
	topic, err := strconv.Atoi(topicString)
	if err != nil {
		// Handle the error if the conversion fails
		return false, fmt.Errorf("invalid topic, not an integer: %s", err)
	}
	if topic <= 0 {
		log.Info("poolId should be positive")
		return false, fmt.Errorf("invalid topic, not an integer: %s", err)
	}
	localPeerID := bl.h.ID()
	localPeerIDStr := localPeerID.String()

	req := PoolUserListRequest{
		PoolID:        topic,
		RequestPoolID: topic,
	}

	// Call the existing function to make the request
	responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", actionPoolUserList, req)
	if err != nil {
		// Handle the error from callBlockchain, including the status code for more context
		return false, fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
	}

	// Check if the status code is OK; if not, handle it as an error
	if statusCode != http.StatusOK {
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(responseBody, &errMsg); jsonErr == nil {
			// If the responseBody is JSON, use it in the error message
			return false, fmt.Errorf("unexpected response status: %d, message: %s, description: %s",
				statusCode, errMsg["message"], errMsg["description"])
		} else {
			// If the responseBody is not JSON, return it as a plain text error message
			return false, fmt.Errorf("unexpected response status: %d, body: %s", statusCode, string(responseBody))
		}
	}
	var response PoolUserListResponse

	// Unmarshal the response body into the response struct
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return false, err
	}

	// Now iterate through the users and populate the member map
	log.Debugw("Now iterate through the users and find our status", "peer", localPeerID, "response", response.Users)

	for _, user := range response.Users {
		//Check if self status is in pool request, start ping server and announce join request
		if user.PeerID == localPeerIDStr {
			log.Debugw("Found self peerID", user.PeerID)
			if user.RequestPoolID != nil && strconv.Itoa(*user.RequestPoolID) == topicString {
				log.Debugw("Found self peerID in pool", "peer", user.PeerID, "pool", topicString)
				return true, nil
			} else if user.PoolID != nil {
				log.Debugw("PeerID is already a member of the pool", user.PeerID, "pool", topicString)
				return false, nil
			} else {
				log.Debugw("PeerID is not a member or joining member of the requested pool", user.PeerID, "pool", topicString)
				return false, nil
			}
		}
	}
	return false, nil
}

func (bl *FxBlockchain) clearPoolPeersFromPeerAddr(ctx context.Context, topic int) error {
	bl.membersLock.Lock()
	for i := range bl.members {
		delete(bl.members, i)
	}
	bl.membersLock.Unlock()

	localPeerID := bl.h.ID()

	if topic <= 0 {
		log.Info("Not a member of any pool at the moment")
		return nil
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
		bl.h.Peerstore().ClearAddrs(pid)
	}
	return nil
}

func multiaddrsToStrings(addrs []multiaddr.Multiaddr) []string {
	var addrsStr []string
	for _, addr := range addrs {
		addrsStr = append(addrsStr, addr.String())
	}
	return addrsStr
}

// FetchUsersAndPopulateSets fetches pool members and updates the local members map
// This method is optimized for production use with EVM chains and no voting logic
func (bl *FxBlockchain) FetchUsersAndPopulateSets(ctx context.Context, topicString string, initiate bool, timeout time.Duration) error {
	// Acquire lock to check if another fetch is in progress
	bl.fetchMutex.Lock()
	if bl.isFetching {
		bl.fetchMutex.Unlock()
		return fmt.Errorf("FetchUsersAndPopulateSets is already being executed")
	}
	bl.isFetching = true
	defer func() {
		bl.fetchMutex.Lock()
		bl.isFetching = false
		bl.fetchMutex.Unlock()
	}()
	bl.fetchMutex.Unlock()

	ctx, cancelCtx := context.WithTimeout(ctx, timeout)
	defer cancelCtx()

	log.Debugw("FetchUsersAndPopulateSets executed", "topicString", topicString, "initiate", initiate)

	// Initialize the members map if needed
	if bl.members == nil {
		bl.membersLock.Lock()
		bl.members = make(map[peer.ID]common.MemberStatus)
		bl.membersLock.Unlock()
	}

	// Convert topic from string to int
	topic, err := strconv.Atoi(topicString)
	if err != nil {
		return fmt.Errorf("invalid topic, not an integer: %w", err)
	}
	if topic <= 0 {
		log.Debugw("Not a member of any pool, skipping fetch", "topic", topicString)
		return nil
	}

	// Check if we need to update pool/chain configuration
	if initiate {
		currentChain := bl.getChainName()
		if currentChain == "" || topicString == "0" {
			log.Debugw("Pool or chain not configured, attempting discovery", "topic", topicString, "chain", currentChain)
			if err := bl.discoverPoolAndChain(ctx, uint32(topic)); err != nil {
				log.Errorw("Failed to discover pool and chain", "topic", topic, "error", err)
				// Continue with existing configuration if discovery fails
			}
		}
	}

	bl.lastFetchTime = time.Now()
	return nil
}

// discoverPoolAndChain attempts to discover the correct pool and chain configuration
func (bl *FxBlockchain) discoverPoolAndChain(ctx context.Context, poolID uint32) error {
	chainList := []string{"skale", "base"}

	for _, chainName := range chainList {
		log.Debugw("Checking pool membership on chain", "poolID", poolID, "chain", chainName)

		// Check if our peer ID is a member of this pool on this chain
		membershipReq := IsMemberOfPoolRequest{
			PeerID:    bl.h.ID().String(),
			PoolID:    poolID,
			ChainName: chainName,
		}

		membershipResp, err := bl.HandleIsMemberOfPool(ctx, membershipReq)
		if err != nil {
			log.Debugw("Failed to check membership", "poolID", poolID, "chain", chainName, "error", err)
			continue
		}

		if membershipResp.IsMember {
			log.Infow("Found pool membership", "poolID", poolID, "chain", chainName, "memberAddress", membershipResp.MemberAddress)

			// Update pool name
			if bl.updatePoolName != nil {
				if err := bl.updatePoolName(strconv.Itoa(int(poolID))); err != nil {
					log.Errorw("Failed to update pool name", "poolID", poolID, "error", err)
				}
			}

			// Update chain name
			if bl.updateChainName != nil {
				if err := bl.updateChainName(chainName); err != nil {
					log.Errorw("Failed to update chain name", "chainName", chainName, "error", err)
				}
			}

			return nil
		}
	}

	return fmt.Errorf("not a member of pool %d on any supported chain", poolID)
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

const creatorPeerIDFilePath = "/internal/.tmp/pool_%d_creator.tmp"

func (bl *FxBlockchain) getClusterEndpoint(ctx context.Context, poolID int) (string, error) {
	// 1. Check for existing creator peer ID
	if creatorPeerID, err := loadCreatorPeerID(poolID); err == nil && creatorPeerID != "" {
		log.Debugw("Using cached endpoint for pool", "ID", poolID, "Creator", creatorPeerID)
		return strconv.Itoa(poolID) + ".pools.functionyard.fula.network", nil
	}

	// 2. Get the current chain name from configuration
	chainName := bl.getChainName()
	if chainName == "" {
		// Default to skale chain if no chain is configured
		chainName = "skale"
		log.Debugw("No chain configured, defaulting to skale", "poolID", poolID)
	}

	// 3. Get pool creator peer ID from blockchain
	peerID, err := bl.GetPoolCreatorPeerID(ctx, uint32(poolID), chainName)
	if err != nil {
		log.Errorw("Failed to get pool creator peer ID from blockchain", "poolID", poolID, "chain", chainName, "error", err)

		// Try the other chain as fallback
		fallbackChain := "base"
		if chainName == "base" {
			fallbackChain = "skale"
		}

		log.Debugw("Trying fallback chain", "poolID", poolID, "fallbackChain", fallbackChain)
		peerID, err = bl.GetPoolCreatorPeerID(ctx, uint32(poolID), fallbackChain)
		if err != nil {
			log.Errorw("Failed to get pool creator peer ID from fallback chain", "poolID", poolID, "fallbackChain", fallbackChain, "error", err)
			return "", fmt.Errorf("failed to get pool creator peer ID from both chains: %w", err)
		}

		// Update chain name if fallback succeeded
		chainName = fallbackChain
		if bl.updateChainName != nil {
			if updateErr := bl.updateChainName(chainName); updateErr != nil {
				log.Errorw("Failed to update chain name after fallback", "chainName", chainName, "error", updateErr)
			}
		}
	}

	// 4. Save creator peer ID to cache
	if saveErr := saveCreatorPeerID(poolID, peerID); saveErr != nil {
		log.Debugw("Error saving creator peer ID to cache", "poolID", poolID, "error", saveErr)
	}

	log.Infow("Successfully retrieved cluster endpoint", "poolID", poolID, "chain", chainName, "creatorPeerID", peerID)
	return strconv.Itoa(poolID) + ".pools.functionyard.fula.network", nil
}
func loadCreatorPeerID(poolID int) (string, error) {
	filename := fmt.Sprintf(creatorPeerIDFilePath, poolID)
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // File doesn't exist, not an error
		}
		return "", fmt.Errorf("error reading creator peer ID file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func saveCreatorPeerID(poolID int, peerID string) error {
	filename := fmt.Sprintf(creatorPeerIDFilePath, poolID)
	return os.WriteFile(filename, []byte(peerID), 0644) // Adjust permissions if needed
}

func fetchPoolDetails(ctx context.Context, bl *FxBlockchain, poolID int) (*Pool, error) {
	req := PoolListRequestWithPoolId{PoolID: poolID}
	action := "actionPoolList"

	responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", action, req)
	if err != nil {
		return nil, fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
	}

	if statusCode != http.StatusOK {
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(responseBody, &errMsg); jsonErr == nil {
			return nil, fmt.Errorf("unexpected response status: %d, message: %s, description: %s",
				statusCode, errMsg["message"], errMsg["description"])
		} else {
			return nil, fmt.Errorf("unexpected response status: %d, body: %s", statusCode, string(responseBody))
		}
	}

	var response PoolListResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}

	for _, pool := range response.Pools {
		if pool.PoolID == poolID {
			return &pool, nil
		}
	}

	return nil, fmt.Errorf("pool with ID %d not found", poolID)
}

func (bl *FxBlockchain) fetchUserDetails(ctx context.Context, poolID int) (*PoolUserListResponse, error) {
	req := PoolUserListRequest{
		PoolID: poolID,
	}
	action := "actionPoolUserList"

	responseBody, statusCode, err := bl.callBlockchain(ctx, "POST", action, req)
	if err != nil {
		return nil, fmt.Errorf("blockchain call error: %w, status code: %d", err, statusCode)
	}

	if statusCode != http.StatusOK {
		var errMsg map[string]interface{}
		if jsonErr := json.Unmarshal(responseBody, &errMsg); jsonErr == nil {
			return nil, fmt.Errorf("unexpected response status: %d, message: %s, description: %s",
				statusCode, errMsg["message"], errMsg["description"])
		} else {
			return nil, fmt.Errorf("unexpected response status: %d, body: %s", statusCode, string(responseBody))
		}
	}

	var response PoolUserListResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (bl *FxBlockchain) findPeerID(creatorClusterPeerID string, userDetails *PoolUserListResponse) string {
	for _, user := range userDetails.Users {
		if user.Account == creatorClusterPeerID {
			return user.PeerID
		}
	}
	return ""
}

func (bl *FxBlockchain) handleActionManifestBatchUpload(method string, action string, from peer.ID, w http.ResponseWriter, r *http.Request, req *ManifestBatchUploadRequest) {
	log := log.With("action", action, "from", from)
	res := reflect.New(responseTypes[action]).Interface()
	defer r.Body.Close()
	//TODO: Ensure it is optimized for long-running calls
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*time.Duration(bl.timeout))
	defer cancel()
	if req.PoolID != 0 {
		// Call ipfs-cluster of the pool with replication request
		clusterEndPoint, err := bl.getClusterEndpoint(ctx, req.PoolID)
		if err != nil {
			w.WriteHeader(http.StatusFailedDependency)
			errMsg := map[string]interface{}{
				"message":     "Error",
				"description": string(err.Error()),
			}
			json.NewEncoder(w).Encode(errMsg)
			return
		}
		if clusterEndPoint != "" {
			// Construct the request for the cluster endpoint
			replicationRequest := struct {
				PoolID int      `json:"pool_id"`
				Cids   []string `json:"cids"`
			}{
				PoolID: req.PoolID,
				Cids:   req.Cid,
			}

			reqBody, err := json.Marshal(replicationRequest)
			if err != nil {
				w.WriteHeader(http.StatusFailedDependency)
				errMsg := map[string]interface{}{
					"message":     "Error",
					"description": string(err.Error()),
				}
				json.NewEncoder(w).Encode(errMsg)
				return
			}

			// Make the HTTP request to the cluster endpoint
			clusterURL := fmt.Sprintf("%s/pins", clusterEndPoint)
			log.Debugw("Pinning on ipfs-cluster", "url", clusterURL)
			resp, err := http.Post(clusterURL, "application/json", bytes.NewBuffer(reqBody))
			if err != nil {
				w.WriteHeader(http.StatusFailedDependency)
				errMsg := map[string]interface{}{
					"message":     "Error",
					"description": string(err.Error()),
				}
				json.NewEncoder(w).Encode(errMsg)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusAccepted {
				// Replication request was not accepted
				w.WriteHeader(resp.StatusCode) // Set the appropriate status code
				responseBody, _ := io.ReadAll(resp.Body)
				errMsg := map[string]interface{}{
					"message":     "Replication Error",
					"description": string(responseBody),
				}
				json.NewEncoder(w).Encode(errMsg)
				return
			}

			// Replication request was accepted - continue with existing response handling
		} else {
			w.WriteHeader(http.StatusFailedDependency)
			errMsg := map[string]interface{}{
				"message":     "Error",
				"description": "Wrong cluster endpoint",
			}
			json.NewEncoder(w).Encode(errMsg)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
	err1 := json.Unmarshal([]byte(`{}`), &res)
	if err1 != nil {
		log.Error("failed to format response: %v", err1)
	}

	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Error("failed to write response: %v", err)
	}
}
