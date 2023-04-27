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
	"strings"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/sys/unix"
)

const (
	FxBlockchainProtocolID = "/fx.land/blockchain/0.0.1"
	actionAuth             = "auth"
)

var (
	_ Blockchain = (*FxBlockchain)(nil)

	log = logging.Logger("fula/blockchain")
)

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
	}
	authorizationRequest struct {
		Subject peer.ID `json:"id"`
		Allow   bool    `json:"allow"`
	}
)

func NewFxBlockchain(h host.Host, keyStorer KeyStorer, o ...Option) (*FxBlockchain, error) {
	opts, err := newOptions(o...)
	if err != nil {
		return nil, err
	}
	bl := &FxBlockchain{
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
		keyStorer: keyStorer,
	}
	if bl.authorizer != "" {
		if err := bl.SetAuth(context.Background(), h.ID(), bl.authorizer, true); err != nil {
			return nil, err
		}
	}
	return bl, nil
}

func (bl *FxBlockchain) Start(ctx context.Context) error {
	listen, err := gostream.Listen(bl.h, FxBlockchainProtocolID)
	if err != nil {
		return err
	}
	bl.s.Handler = http.HandlerFunc(bl.serve)
	go func() { bl.s.Serve(listen) }()
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

type ReqInterface interface{}

func (bl *FxBlockchain) PlugSeedIfNeeded(ctx context.Context, action string, req interface{}) interface{} {
	switch action {
	case actionSeeded, actionAccountExists, actionPoolCreate, actionPoolJoin, actionPoolCancelJoin, actionPoolRequests, actionPoolList, actionPoolVote, actionPoolLeave, actionManifestUpload, actionManifestStore, actionManifestAvailable, actionManifestRemove, actionManifestRemoveStorer, actionManifestRemoveStored:
		seed, err := bl.keyStorer.LoadKey(ctx)
		if err != nil {
			log.Errorw("seed is empty", "err", err)
			seed = []byte{}
		}
		return struct {
			ReqInterface
			Seed string
		}{
			ReqInterface: req,
			Seed:         string(seed),
		}
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
			//TODO: We should check if from owns the blox
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
		w.WriteHeader(http.StatusBadRequest)
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
	stat := unix.Statfs_t{}

	err := unix.Statfs(os.Getenv("FULA_BLOX_STORE_DIR"), &stat)
	if err != nil {
		log.Errorw("calling unix.Statfs", "storeDir", os.Getenv("FULA_BLOX_STORE_DIR"))
		http.Error(w, "calling unix.Statfs", http.StatusInternalServerError)
		return
	}
	var Size float32 = float32(stat.Blocks * uint64(stat.Bsize))
	var Avail float32 = float32(stat.Bfree * uint64(stat.Bsize))
	var Used float32 = float32(Size - Avail)
	var UsedPercentage float32 = 0.0
	if Size > 0.0 {
		UsedPercentage = Used / Size * 100.0
	}
	out := BloxFreeSpaceResponse{
		Size:           Size / float32(GB),
		Avail:          Avail / float32(GB),
		Used:           Used / float32(GB),
		UsedPercentage: UsedPercentage,
	}
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
	case actionBloxFreeSpace, actionSeeded, actionAccountExists, actionPoolCreate, actionPoolJoin, actionPoolCancelJoin, actionPoolRequests, actionPoolList, actionPoolVote, actionPoolLeave, actionManifestUpload, actionManifestStore, actionManifestAvailable, actionManifestRemove, actionManifestRemoveStorer, actionManifestRemoveStored:
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
	return bl.s.Shutdown(ctx)
}
