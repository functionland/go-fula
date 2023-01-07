package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strings"
	"sync"

	logging "github.com/ipfs/go-log/v2"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	FxBlockchainProtocolID = "/fx.land/blockchain/0.0.1"
	FxBlockchainEndPoint   = "127.0.0.1:4000"

	actionSeeded = "seeded"
	actionAuth   = "auth"
)

var (
	_ Blockchain = (*FxBlockchain)(nil)

	log            = logging.Logger("fula/blockchain")
	errInvalidType = errors.New("invalid type for to")
)

type (
	FxBlockchain struct {
		*options
		h host.Host
		s *http.Server
		c *http.Client

		authorizedPeers     map[peer.ID]struct{}
		authorizedPeersLock sync.RWMutex
	}
	seededRequest struct {
		Seed string `json:"seed"`
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
		authorizedPeers: make(map[peer.ID]struct{}),
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

func (bl *FxBlockchain) Seeded(ctx context.Context, to interface{}, r seededRequest) error {
	var method string
	var addr string

	switch to.(type) {
	case string:
		method = http.MethodPost
		addr = "http://" + to.(string) + "account/" + actionSeeded
	case peer.ID:
		if bl.allowTransientConnection {
			ctx = network.WithUseTransient(ctx, "fx.blockchain")
		}
		method = http.MethodGet
		addr = "http://" + to.(peer.ID).String() + ".invalid/" + actionSeeded
	default:
		return errInvalidType
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, addr, &buf)
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
	case resp.StatusCode != http.StatusAccepted:
		return fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return nil
	}
}

func (bl *FxBlockchain) serve(w http.ResponseWriter, r *http.Request) {
	from, err := peer.Decode(r.RemoteAddr)
	if err != nil {
		log.Debugw("cannot parse remote addr as peer ID", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	action := path.Base(r.URL.Path)
	if !bl.authorized(from, action) {
		log.Debugw("rejected unauthorized request", "from", from, "action", action)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	switch action {
	case actionSeeded:
		bl.handleSeeded(from, w, r)
	case actionAuth:
		bl.handleAuthorization(from, w, r)
	default:
		http.Error(w, "", http.StatusNotFound)
	}
}

func (bl *FxBlockchain) handleSeeded(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionSeeded, "from", from)
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorw("failed to read request body", "err", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var p seededRequest
	if err := json.Unmarshal(b, &p); err != nil {
		log.Debugw("cannot parse request body", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	go func() {
		ctx := context.TODO()
		if bl.allowTransientConnection {
			ctx = network.WithUseTransient(ctx, "fx.blockchain")
		}
		bl.Seeded(ctx, FxBlockchainEndPoint, p)
	}()
	w.WriteHeader(http.StatusAccepted)
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
