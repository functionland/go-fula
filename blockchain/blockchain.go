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
		c  *http.Client
		ch *http.Client

		authorizedPeers     map[peer.ID]struct{}
		authorizedPeersLock sync.RWMutex

		bufPool *sync.Pool
		reqPool *sync.Pool
	}
	authorizationRequest struct {
		Subject peer.ID `json:"id"`
		Allow   bool    `json:"allow"`
	}
)

func NewFxBlockchain(h host.Host, o ...Option) (*FxBlockchain, error) {
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

func (bl *FxBlockchain) callBlockchain(ctx context.Context, action string, p interface{}) ([]byte, error) {
	method := http.MethodPost
	addr := "http://" + bl.blockchainEndPoint + "/" + strings.Replace(action, "-", "/", -1)

	// Use the bufPool and reqPool to reuse bytes.Buffer and http.Request objects
	buf := bl.bufPool.Get().(*bytes.Buffer)
	req := bl.reqPool.Get().(*http.Request)
	defer func() {
		bl.putBuf(buf)
		bl.putReq(req)
	}()

	if err := json.NewEncoder(buf).Encode(p); err != nil {
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
			bl.handleAction(actionSeeded, from, w, r)
		},
		actionAuth: func(from peer.ID, w http.ResponseWriter, r *http.Request) {
			bl.handleAuthorization(from, w, r)
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

func (bl *FxBlockchain) handleAction(action string, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", action, "from", from)
	req := reflect.New(requestTypes[action]).Interface()
	res := reflect.New(responseTypes[action]).Interface()

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug("cannot parse request body: %v", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	//go func() {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*30)
	defer cancel()
	response, err := bl.callBlockchain(ctx, action, req)
	if err != nil {
		log.Error("failed to process seeded request: %v", err)
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
	//}()
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
	case actionSeeded:
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
	return bl.s.Shutdown(ctx)
}
