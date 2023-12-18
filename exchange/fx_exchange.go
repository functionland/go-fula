package exchange

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"sync"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	_ "github.com/ipld/go-ipld-prime/codec/raw"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	gostream "github.com/libp2p/go-libp2p-gostream"
	p2phttp "github.com/libp2p/go-libp2p-http"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multicodec"
)

const (
	FxExchangeProtocolID = "/fx.land/exchange/0.0.2"

	actionPull = "pull"
	actionPush = "push"
	actionAuth = "auth"
)

var (
	_ Exchange = (*FxExchange)(nil)

	log                           = logging.Logger("fula/exchange")
	errUnauthorized               = errors.New("not authorized")
	exploreAllRecursivelySelector selector.Selector
	pl                            = cidlink.LinkPrototype{
		Prefix: cid.Prefix{
			Version:  1,
			Codec:    uint64(multicodec.DagCbor),
			MhType:   uint64(multicodec.Sha2_256),
			MhLength: -1,
		},
	}
)

func init() {
	var err error
	exploreAllRecursivelySelector, err = selector.ParseSelector(selectorparse.CommonSelector_ExploreAllRecursively)
	if err != nil {
		panic("failed to parse IPLD built-in selector selectorparse.CommonSelector_ExploreAllRecursively: " + err.Error())
	}
}

type (
	FxExchange struct {
		*options
		h  host.Host
		ls ipld.LinkSystem
		s  *http.Server
		c  *http.Client

		authorizedPeers     map[peer.ID]struct{}
		authorizedPeersLock sync.RWMutex
		pub                 *hubPublisher
		dht                 *fulaDht
	}
	pullRequest struct {
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
	tr := &http.Transport{}
	tr.RegisterProtocol("libp2p", p2phttp.NewTransport(h, p2phttp.ProtocolOption(FxExchangeProtocolID)))
	client := &http.Client{Transport: tr}

	e := &FxExchange{
		options:         opts,
		h:               h,
		ls:              ls,
		s:               &http.Server{},
		c:               client,
		authorizedPeers: make(map[peer.ID]struct{}),
	}
	if e.authorizer != "" {
		if err := e.SetAuth(context.Background(), h.ID(), e.authorizer, true); err != nil {
			return nil, err
		}
	}
	//if !e.ipniPublishDisabled {
	e.pub, err = newHubPublisher(h, opts)
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
func (e *FxExchange) FindProvidersIpni(l ipld.Link, relays []string) ([]peer.AddrInfo, error) {
	cidStr := l.String() // Convert IPLD Link to string
	url := fmt.Sprintf("%s%s", e.ipniGetEndpoint, cidStr)

	// Make HTTP GET request
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var result struct {
		MultihashResults []MultihashResult `json:"MultihashResults"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var addrInfos []peer.AddrInfo
	for _, mhResult := range result.MultihashResults {
		for _, provider := range mhResult.ProviderResults {
			// Decode ContextID from Base64
			decodedBytes, err := base64.StdEncoding.DecodeString(provider.ContextID)
			if err != nil {
				log.Errorf("Base64 Decode Error: %v", err)
				continue
			}
			decodedContextID := string(decodedBytes)

			// Convert decoded bytes to peer.ID
			pid, err := peer.Decode(decodedContextID)
			if err != nil {
				log.Errorf("Error converting to peer.ID: %v", err)
				continue
			}

			// Convert relay addresses to multiaddr.Multiaddr and add to AddrInfo
			var addrs []multiaddr.Multiaddr
			for _, relay := range relays {
				maddr, err := multiaddr.NewMultiaddr(relay)
				if err != nil {
					log.Errorf("Error parsing multiaddr: %v", err)
					continue
				}
				addrs = append(addrs, maddr)
			}

			addrInfo := peer.AddrInfo{
				ID:    pid,
				Addrs: addrs,
			}
			addrInfos = append(addrInfos, addrInfo)
		}
	}

	return addrInfos, nil
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

	if err := e.pub.Start(ctx); err != nil {
		return err
	}
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
	r := pullRequest{Link: l.(cidlink.Link).Cid}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		log.Errorw("Failed to encode pull request", "err", err)
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "libp2p://"+from.String()+"/"+actionPull, &buf)
	if err != nil {
		log.Errorw("Failed to instantiate pull request", "err", err)
		return err
	}
	resp, err := e.c.Do(req)
	if err != nil {
		log.Errorw("Failed to send pull request", "err", err)
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		log.Errorw("Failed to read pull response", "err", err)
		return err
	case resp.StatusCode != http.StatusAccepted:
		log.Errorw("Expected status accepted on pull response", "got", resp.StatusCode)
		return fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		log.Infow("Successfully sent push request")
		return nil
	}
}

func (e *FxExchange) Push(ctx context.Context, to peer.ID, l ipld.Link) error {
	if e.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.exchange")
	}

	log := log.With("cid", l.(cidlink.Link).Cid)
	node, err := e.ls.Load(ipld.LinkContext{Ctx: ctx}, l, basicnode.Prototype.Any)
	if err != nil {
		log.Errorw("Failed to load link prior to push ", "err", err)
		return err
	}

	progress := traversal.Progress{
		Cfg: &traversal.Config{
			Ctx:                            ctx,
			LinkSystem:                     e.ls,
			LinkTargetNodePrototypeChooser: basicnode.Chooser,
		},
	}

	// Recursively traverse the node and push all its leaves.
	err = progress.WalkAdv(node, exploreAllRecursivelySelector, func(progress traversal.Progress, node datamodel.Node, _ traversal.VisitReason) error {
		return e.pushOneNode(ctx, node, to)
	})
	if err != nil {
		log.Errorw("Failed to traverse link during push", "err", err)
		return err
	}
	return nil
}

func (e *FxExchange) pushOneNode(ctx context.Context, node ipld.Node, to peer.ID) error {
	var buf bytes.Buffer
	err := dagcbor.Encode(node, &buf)
	if err != nil {
		log.Errorw("Failed encode node to be pushed", "err", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "libp2p://"+to.String()+"/"+actionPush, &buf)
	if err != nil {
		log.Errorw("Failed to instantiate push request", "err", err)
		return err
	}
	resp, err := e.c.Do(req)
	if err != nil {
		log.Errorw("Failed to send push request", "err", err)
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		log.Errorw("Failed to read the response from push", "err", err)
		return err
	case resp.StatusCode != http.StatusOK:
		log.Errorw("Received non-OK response from push", "err", err)
		return fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		log.Infow("Successfully pushed traversed node")
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
		http.Error(w, errUnauthorized.Error(), http.StatusUnauthorized)
		return
	}
	switch action {
	case actionPull:
		e.handlePull(from, w, r)
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
	if r.Method != http.MethodPost {
		log.Errorw("Only POST is allowed on push", "got", r.Method)
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	na := basicnode.Prototype.Any.NewBuilder()
	err := dagcbor.Decode(na, r.Body)
	if err != nil {
		log.Errorw("Failed to decode pushed node as dagcbor", "err", err)
		http.Error(w, "failed to decode node", http.StatusBadRequest)
		return
	}
	l, err := e.ls.Store(ipld.LinkContext{Ctx: r.Context()}, pl, na.Build())
	if err != nil {
		log.Errorw("Failed to store pushed node", "err", err)
		http.Error(w, "failed to store node", http.StatusInternalServerError)
		return
	}
	log.Infow("Successfully stored pushed node", "cid", l.(cidlink.Link).Cid)
	log.Debugw("Notifying stored pushed link to IPNI publisher", "link", l)
	e.pub.notifyReceivedLink(l)
	log.Debugw("Successfully notified stored pushed link to IPNI publisher", "link", l)
}

func (e *FxExchange) handlePull(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionPull, "from", from)
	if r.Method != http.MethodPost {
		log.Errorw("Only POST is allowed on pull", "got", r.Method)
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorw("failed to read pull request body", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	var p pullRequest
	if err := json.Unmarshal(b, &p); err != nil {
		log.Errorw("Cannot parse pull request body", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	log = log.With("link", p.Link)
	w.WriteHeader(http.StatusAccepted)
	log.Info("Accepted pull request")
	log.Info("Instantiating background push in response to pull request")
	ctx := context.TODO()
	if e.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.exchange")
	}
	if err := e.Push(ctx, from, cidlink.Link{Cid: p.Link}); err != nil {
		log.Errorw("Failed to fetch in response to push", "err", err)
	} else {
		log.Infow("Successfully finished background push in response to pull request")
	}
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "libp2p://"+on.String()+"/"+actionAuth, &buf)
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
	case actionPush:
		e.authorizedPeersLock.RLock()
		_, ok := e.authorizedPeers[pid]
		e.authorizedPeersLock.RUnlock()
		return ok
	case actionAuth:
		return pid == e.authorizer
	case actionPull:
		//TODO: Check if requestor is part of the same pool or the owner of the cid
		return true
	default:
		return false
	}
}

func (e *FxExchange) Shutdown(ctx context.Context) error {
	//if !e.ipniPublishDisabled {
	if err := e.pub.shutdown(); err != nil {
		log.Warnw("Failed to shutdown IPNI publisher gracefully", "err", err)
	}
	e.dht.dh.Close()
	e.dht.Shutdown()
	//}
	e.c.CloseIdleConnections()
	return e.s.Shutdown(ctx)
}
