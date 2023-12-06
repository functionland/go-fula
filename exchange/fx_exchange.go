package exchange

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

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	gs "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	_ "github.com/ipld/go-ipld-prime/codec/raw"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	FxExchangeProtocolID = "/fx.land/exchange/0.0.1"

	actionPull = "pull"
	actionPush = "push"
	actionAuth = "auth"
)

var (
	_ Exchange = (*FxExchange)(nil)

	log             = logging.Logger("fula/exchange")
	errUnauthorized = errors.New("not authorized")
)

type (
	FxExchange struct {
		*options
		h  host.Host
		gx graphsync.GraphExchange
		ls ipld.LinkSystem
		s  *http.Server
		c  *http.Client

		authorizedPeers     map[peer.ID]struct{}
		authorizedPeersLock sync.RWMutex
		pub                 *ipniPublisher
		dht                 *fulaDht
	}
	pushRequest struct {
		Link cid.Cid `json:"link"`
	}
	authorizationRequest struct {
		Subject peer.ID `json:"id"`
		Allow   bool    `json:"allow"`
	}
)

func NewFxExchange(h host.Host, ls ipld.LinkSystem, o ...Option) (*FxExchange, error) {
	opts, err := newOptions(o...)
	if err != nil {
		return nil, err
	}
	e := &FxExchange{
		options: opts,
		h:       h,
		ls:      ls,
		s:       &http.Server{},
		c: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					pid, err := peer.Decode(strings.TrimSuffix(addr, ".invalid:80"))
					if err != nil {
						return nil, err
					}
					return gostream.Dial(ctx, h, pid, FxExchangeProtocolID)
				},
			},
		},
		authorizedPeers: make(map[peer.ID]struct{}),
	}
	if e.authorizer != "" {
		if err := e.SetAuth(context.Background(), h.ID(), e.authorizer, true); err != nil {
			return nil, err
		}
	}
	//if !e.ipniPublishDisabled {
	e.pub, err = newIpniPublisher(h, opts)
	if err != nil {
		return nil, err
	}
	//}
	e.dht, err = newDhtProvider(h, opts)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (e *FxExchange) UpdateDhtPeers(peers []peer.ID) error {
	for _, ma := range peers {
		e.dht.AddPeer(ma)
	}
	log.Infow("peers are set", "peers", peers)
	return nil
}

func (e *FxExchange) GetAuth(ctx context.Context) (peer.ID, error) {
	return e.authorizer, nil
}

func (e *FxExchange) ProvideDht(l ipld.Link) error {
	return e.dht.Provide(l)
}

func (e *FxExchange) PingDht(p peer.ID) error {
	return e.dht.PingDht(p)
}

func (e *FxExchange) FindProvidersDht(l ipld.Link) ([]peer.AddrInfo, error) {
	return e.dht.FindProviders(l)
}

func (e *FxExchange) PutValueDht(ctx context.Context, key string, val string) error {
	return e.dht.dh.PutValue(ctx, key, []byte(val))
}

func (e *FxExchange) SearchValueDht(ctx context.Context, key string) (string, error) {
	valueStream, err := e.dht.dh.SearchValue(ctx, key)
	if err != nil {
		return "", fmt.Errorf("error searching value in DHT: %w", err)
	}

	var mostAccurateValue []byte
	for val := range valueStream {
		mostAccurateValue = val // The last value received before the channel closes is the most accurate
	}

	if mostAccurateValue == nil {
		return "", errors.New("no value found for the key")
	}

	return string(mostAccurateValue), nil
}

func (e *FxExchange) GetAuthorizedPeers(ctx context.Context) ([]peer.ID, error) {
	var peerList []peer.ID
	for peerId := range e.authorizedPeers {
		peerList = append(peerList, peerId)
	}
	if e.options == nil {
		return nil, fmt.Errorf("options is nil")
	}
	e.options.authorizedPeers = peerList
	return peerList, nil
}

func (e *FxExchange) IpniNotifyLink(link ipld.Link) {
	log.Debugw("Notifying link to IPNI publisher...", "link", link)
	e.pub.notifyReceivedLink(link)
	log.Debugw("Successfully notified link to IPNI publisher", "link", link)
}

func (e *FxExchange) Start(ctx context.Context) error {
	gsn := gsnet.NewFromLibp2pHost(e.h)
	e.gx = gs.New(ctx, gsn, e.ls)

	if err := e.pub.Start(ctx); err != nil {
		return err
	}
	if !e.ipniPublishDisabled {
		e.gx.RegisterIncomingBlockHook(func(p peer.ID, responseData graphsync.ResponseData, blockData graphsync.BlockData, hookActions graphsync.IncomingBlockHookActions) {
			go func(link ipld.Link) {
				log.Debugw("Notifying link to IPNI publisher...", "link", link)
				e.pub.notifyReceivedLink(link)
				log.Debugw("Successfully notified link to IPNI publisher", "link", link)
			}(blockData.Link())
		})
	}

	e.gx.RegisterIncomingRequestHook(
		func(p peer.ID, r graphsync.RequestData, ha graphsync.IncomingRequestHookActions) {
			if e.authorized(p, actionPull) {
				ha.ValidateRequest()
			} else {
				ha.TerminateWithError(errUnauthorized)
			}
		})
	listen, err := gostream.Listen(e.h, FxExchangeProtocolID)
	if err != nil {
		return err
	}
	e.s.Handler = http.HandlerFunc(e.serve)
	go func() { e.s.Serve(listen) }()
	return nil
}

func (e *FxExchange) Pull(ctx context.Context, from peer.ID, l ipld.Link) error {
	if e.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.exchange")
	}

	if e.wg != nil {
		e.wg.Add(1)
	}
	go func() {
		if e.wg != nil {
			defer e.wg.Done()
		}
		// Call Provide for the last block link of each response
		if err := e.dht.Provide(l); err != nil {
			log.Warnw("Failed to provide link via DHT", "link", l, "err", err)
			// Handle the error
		} else {
			log.Debug("Success provide link via DHT")
		}
	}()
	resps, errs := e.gx.Request(ctx, from, l, selectorparse.CommonSelector_ExploreAllRecursively)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case resp, ok := <-resps:
			if !ok {
				return nil
			}
			log.Infow("synced node", "node", resp.Node)

			// Call Provide for the last block link of each response
			if err := e.dht.Provide(resp.LastBlock.Link); err != nil {
				log.Warnw("Failed to provide link via DHT2", "link", resp.LastBlock.Link, "err", err)
				// Decide how to handle the error, e.g., continue, return, etc.
			} else {
				log.Debug("Success provide link via DHT")
			}

		case err, ok := <-errs:
			if !ok {
				return nil
			}
			log.Warnw("sync failed", "err", err)
		}
	}
}

func (e *FxExchange) Push(ctx context.Context, to peer.ID, l ipld.Link) error {
	if e.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.exchange")
	}
	r := pushRequest{Link: l.(cidlink.Link).Cid}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPush, &buf)
	if err != nil {
		return err
	}
	resp, err := e.c.Do(req)
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

func (e *FxExchange) serve(w http.ResponseWriter, r *http.Request) {
	from, err := peer.Decode(r.RemoteAddr)
	if err != nil {
		log.Debugw("cannot parse remote addr as peer ID", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	action := path.Base(r.URL.Path)
	if !e.authorized(from, action) {
		log.Debugw("rejected unauthorized request", "from", from, "action", action)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	switch action {
	case actionPush:
		e.handlePush(from, w, r)
	case actionAuth:
		e.handleAuthorization(from, w, r)
	default:
		http.Error(w, "", http.StatusNotFound)
	}
}

func (e *FxExchange) handlePush(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionPush, "from", from)
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorw("failed to read request body", "err", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	var p pushRequest
	if err := json.Unmarshal(b, &p); err != nil {
		log.Debugw("cannot parse request body", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	go func() {
		ctx := context.TODO()
		if e.allowTransientConnection {
			ctx = network.WithUseTransient(ctx, "fx.exchange")
		}
		if err := e.Pull(ctx, from, cidlink.Link{Cid: p.Link}); err != nil {
			log.Warnw("failed to fetch in response to push", "err", err)
		} else {
			log.Debugw("successfully fetched in response to push", "from", from, "link", p.Link)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}

func (e *FxExchange) handleAuthorization(from peer.ID, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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
	e.authorizedPeersLock.Lock()
	if a.Allow {
		e.authorizedPeers[a.Subject] = struct{}{}
	} else {
		delete(e.authorizedPeers, a.Subject)
	}
	log.Infow("Authorizing peers for ", "a.Subject", a.Subject, "from", from)
	e.authorizedPeersLock.Unlock()
	if err := e.updateAuthorizePeers(ctx); err != nil {
		log.Errorw("failed to update authorized peers", "err", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (e *FxExchange) updateAuthorizePeers(ctx context.Context) error {
	var peerList []peer.ID
	peerList, _ = e.GetAuthorizedPeers(ctx)
	e.options.authorizedPeers = peerList
	if e.options == nil {
		return fmt.Errorf("options is nil")
	}
	log.Infow("update authorized peers", "peers", e.options.authorizedPeers)
	if e.updateConfig == nil {
		return nil
	}
	err := e.updateConfig(e.options.authorizedPeers)
	log.Infow("update authorized peers", "err", err)
	if err != nil {
		return err
	}
	return nil
}

func (e *FxExchange) SetAuth(ctx context.Context, on peer.ID, subject peer.ID, allow bool) error {
	// Check if auth is for local host; if so, handle it locally.
	if on == e.h.ID() {
		e.authorizedPeersLock.Lock()
		if allow {
			e.authorizedPeers[subject] = struct{}{}
		} else {
			delete(e.authorizedPeers, subject)
		}
		e.authorizedPeersLock.Unlock()

		// Save the updated authorized peers to config file
		err := e.updateAuthorizePeers(ctx)
		if err != nil {
			return err
		}
		return nil
	}
	if e.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.exchange")
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
	resp, err := e.c.Do(req)
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

func (e *FxExchange) authorized(pid peer.ID, action string) bool {
	if e.authorizer == "" {
		// If no authorizer is set allow all.
		return true
	}
	switch action {
	case actionPull, actionPush:
		e.authorizedPeersLock.RLock()
		_, ok := e.authorizedPeers[pid]
		e.authorizedPeersLock.RUnlock()
		return ok
	case actionAuth:
		return pid == e.authorizer
	default:
		return false
	}
}

func (e *FxExchange) Shutdown(ctx context.Context) error {
	//if !e.ipniPublishDisabled {
	if err := e.pub.shutdown(); err != nil {
		log.Warnw("Failed to shutdown IPNI publisher gracefully", "err", err)
	}
	//}
	e.c.CloseIdleConnections()
	return e.s.Shutdown(ctx)
}
