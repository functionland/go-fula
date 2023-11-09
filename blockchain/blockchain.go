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

	// Increment the WaitGroup counter before starting the goroutine
	bl.wg.Add(1)

	go func() {
		defer bl.wg.Done() // Decrement the counter when the goroutine completes

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
	bl.wg.Add(1)
	go func() {
		defer bl.wg.Done()
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

func (bl *FxBlockchain) callBlockchain(ctx context.Context, method string, action string, p interface{}) ([]byte, error) {
	addr := "http://" + bl.blockchainEndPoint + "/" + strings.Replace(action, "-", "/", -1)

	// Use the bufPool and reqPool to reuse bytes.Buffer and http.Request objects
	buf := bl.bufPool.Get().(*bytes.Buffer)
	req := bl.reqPool.Get().(*http.Request)
	defer func() {
		bl.putBuf(buf)
		bl.putReq(req)
	}()

	preparedRequest := bl.PlugSeedIfNeeded(ctx, action, p)
	if err := json.NewEncoder(buf).Encode(preparedRequest); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, addr, buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := bl.ch.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var bufRes bytes.Buffer
	_, err = io.Copy(&bufRes, resp.Body)
	if err != nil {
		return nil, err
	}
	b := bufRes.Bytes()
	switch {
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) PlugSeedIfNeeded(ctx context.Context, action string, req interface{}) interface{} {
	switch action {
	case actionSeeded, actionAccountExists, actionPoolCreate, actionPoolJoin, actionPoolCancelJoin, actionPoolRequests, actionPoolList, actionPoolVote, actionPoolLeave, actionManifestUpload, actionManifestStore, actionManifestAvailable, actionManifestRemove, actionManifestRemoveStorer, actionManifestRemoveStored:
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
		log.Debug("cannot parse remote addr as peer ID: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	action := path.Base(r.URL.Path)
	if !bl.authorized(from, action) {
		log.Debug("rejected unauthorized request from %s for action %s", from, action)
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
	response, err := bl.callBlockchain(ctx, method, action, req)
	if err != nil {
		errorMessage := err.Error()
		http.Error(w, errorMessage, http.StatusBadRequest)
		log.Error("failed to process action request: %v", err)
		return
	} else {
		w.WriteHeader(http.StatusAccepted)
	}

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
	case actionBloxFreeSpace, actionWifiRemoveall, actionReboot, actionPartition, actionDeleteFulaConfig, actionSeeded, actionAccountExists, actionPoolCreate, actionPoolJoin, actionPoolCancelJoin, actionPoolRequests, actionPoolList, actionPoolVote, actionPoolLeave, actionManifestUpload, actionManifestStore, actionManifestAvailable, actionManifestRemove, actionManifestRemoveStorer, actionManifestRemoveStored:
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

func (bl *FxBlockchain) FetchUsersAndPopulateSets(ctx context.Context, topicString string, initiate bool) error {
	// Update last fetch time on successful fetch
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

	if initiate {
		//If members list is empty we should check what peerIDs we already voted on and update to avoid re-voting
		isMembersEmpty := bl.IsMembersEmpty()
		if isMembersEmpty {
			// Call the bl.PoolRequests and get the list of requests
			req := PoolRequestsRequest{
				PoolID: topic, // assuming 'topic' is your pool id
			}
			responseBody, err := bl.PoolRequests(ctx, bl.h.ID(), req)
			if err != nil {
				return err
			}
			var poolRequestsResponse PoolRequestsResponse

			// Unmarshal the response body into the poolRequestsResponse struct
			if err := json.Unmarshal(responseBody, &poolRequestsResponse); err != nil {
				return err
			}

			// For each one check if voted field in the response, contains the bl.h.ID().String() and if so it means we already voted
			// Move it to members with status UnKnown.
			for _, request := range poolRequestsResponse.PoolRequests {
				if contains(request.Voted, bl.h.ID().String()) {
					pid, err := peer.Decode(request.PeerID)
					if err != nil {
						return err
					}
					bl.membersLock.Lock()
					bl.members[pid] = common.Unknown
					bl.membersLock.Unlock()

					// Create a slice to hold the multiaddresses for the peer
					var addrs []multiaddr.Multiaddr

					// Loop through the static relays and convert them to multiaddr
					for _, relay := range bl.relays {
						ma, err := multiaddr.NewMultiaddr(relay)
						if err != nil {
							bl.membersLock.Unlock()
							return err
						}
						addrs = append(addrs, ma)
					}

					// Add the relay addresses to the peerstore for the peer ID
					bl.h.Peerstore().AddAddrs(pid, addrs, peerstore.PermanentAddrTTL)
				}
			}
		}
	}

	//Get hte list of both join requests and joined members for the pool
	// Create a struct for the POST payload
	payload := PoolUserListRequest{
		PoolID: topic,
	}

	// Call the existing function to make the request
	responseBody, err := bl.PoolUserList(ctx, bl.h.ID(), payload) // 'to' should be the peer ID you want to send requests to
	if err != nil {
		return err
	}
	var response PoolUserListResponse

	// Unmarshal the response body into the response struct
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return err
	}

	// Now iterate through the users and populate the member map
	bl.membersLock.Lock()
	for _, user := range response.Users {
		pid, err := peer.Decode(user.PeerID)
		if err != nil {
			return err
		}

		if initiate {
			//Check if self status is in pool request, start ping server and announce join request
			if user.PeerID == bl.h.ID().String() {
				log.Debugw("Found self peerID", user.PeerID)
				if user.RequestPoolID != nil {
					if !bl.p.Status() {
						err = bl.p.Start(ctx)
						if err != nil {
							log.Errorw("Error when starting hte Ping Server", "PeerID", user.PeerID, "err", err)
						} else {
							bl.a.AnnounceJoinPoolRequestPeriodically(ctx)
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
		existingStatus, exists := bl.members[pid]

		var status common.MemberStatus
		if user.PoolID != nil && *user.PoolID == topic {
			status = common.Approved
		} else if user.RequestPoolID != nil && *user.RequestPoolID == topic {
			status = common.Pending
		} else {
			// Skip users that do not match the topic criteria
			continue
		}

		if initiate {
			//Vote for any peer that has not votd already
			if exists && existingStatus != common.Unknown {
				err = bl.HandlePoolJoinRequest(ctx, pid, topicString, false)
				if err == nil {
					status = common.Unknown
				}
			}
		}

		if exists {
			if existingStatus != common.Approved && status == common.Approved {
				// If the user is already pending and now approved, update to ApprovedOrPending
				bl.members[pid] = common.Approved
			}
			// If the user status is the same as before, there's no need to update
		} else {
			// If the user does not exist in the map, add them
			bl.members[pid] = status
			// Create a slice to hold the multiaddresses for the peer
			var addrs []multiaddr.Multiaddr

			// Loop through the static relays and convert them to multiaddr
			for _, relay := range bl.relays {
				ma, err := multiaddr.NewMultiaddr(relay)
				if err != nil {
					bl.membersLock.Unlock()
					return err
				}
				addrs = append(addrs, ma)
			}

			// Add the relay addresses to the peerstore for the peer ID
			bl.h.Peerstore().AddAddrs(pid, addrs, peerstore.PermanentAddrTTL)
		}
	}
	bl.membersLock.Unlock()

	return nil
}

func (bl *FxBlockchain) GetMemberStatus(id peer.ID) (common.MemberStatus, bool) {
	bl.membersLock.RLock()
	status, exists := bl.members[id]
	if !exists {
		// If the peer.ID doesn't exist in the members map, we treat it as an error case.
		return common.MemberStatus(0), false
	}
	bl.membersLock.RUnlock()
	return status, true
}
