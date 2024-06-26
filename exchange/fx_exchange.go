package exchange

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	bsfetcher "github.com/ipfs/boxo/fetcher/impl/blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	_ "github.com/ipld/go-ipld-prime/codec/raw"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	ipldmc "github.com/ipld/go-ipld-prime/multicodec"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	gostream "github.com/libp2p/go-libp2p-gostream"
	p2phttp "github.com/libp2p/go-libp2p-http"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/sony/gobreaker"
	"golang.org/x/sync/errgroup"
)

const (
	FxExchangeProtocolID = "/fx.land/exchange/0.0.3"

	actionPull = "pull"
	actionPush = "push"
	actionAuth = "auth"
)

var (
	_ Exchange = (*FxExchange)(nil)

	log                           = logging.Logger("fula/exchange")
	errUnauthorized               = errors.New("not authorized")
	exploreAllRecursivelySelector selector.Selector
	breakTime                     = 10 * time.Second
)

type tempAuthEntry struct {
	peerID    peer.ID
	timestamp time.Time
}

var cbSettings = gobreaker.Settings{
	Name:        "Push Circuit Breaker",
	MaxRequests: 3,         // Open circuit after 3 consecutive failures
	Interval:    breakTime, // Remain open for 10 seconds
	OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
		log.Errorf("Circuit breaker '%s' changed state from '%s' to '%s'", name, from, to)
	},
}
var pushCircuitBreaker = gobreaker.NewCircuitBreaker(cbSettings)

func init() {
	var err error
	ssb := builder.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	ss := ssb.ExploreRecursive(selector.RecursionLimitNone(), ssb.ExploreUnion(
		ssb.Matcher(),
		ssb.ExploreAll(ssb.ExploreRecursiveEdge()),
	))
	exploreAllRecursivelySelector, err = ss.Selector()
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

		tempAuths     map[string]tempAuthEntry // A map of CID to peer ID for temporary push authorization
		tempAuthsLock sync.RWMutex             // A lock to ensure concurrent access to tempAuths is safe
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
	tr := &http.Transport{
		DisableKeepAlives: false, // Ensure connections are reused
		MaxIdleConns:      0,
		MaxConnsPerHost:   0,
		IdleConnTimeout:   60 * time.Second,
		// Include any other necessary transport configuration here
	}
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
	e.tempAuths = make(map[string]tempAuthEntry)
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // Ensures we cancel the context when someFunction exits
	if err := e.pub.Start(ctx); err != nil {
		return err
	}
	listen, err := gostream.Listen(e.h, FxExchangeProtocolID)
	if err != nil {
		return err
	}
	e.s.Handler = http.HandlerFunc(e.serve)
	if e.wg != nil {
		log.Debug("Called wg.Add after HandlerFunc")
		e.wg.Add(1)
	}
	go func() {
		if e.wg != nil {
			log.Debug("Called wg.Done after HandlerFunc")
			defer e.wg.Done() // Decrement the counter when the goroutine completes
		}
		defer log.Debug("After HandlerFunc go routine is ending")
		if err := e.s.Serve(listen); !errors.Is(err, http.ErrServerClosed) {
			log.Errorw("HTTP server stopped with error", "err", err)
		}
	}()

	if e.wg != nil {
		log.Debug("Called wg.Add after HandlerFunc2")
		e.wg.Add(1)
	}
	go func() {
		if e.wg != nil {
			log.Debug("Called wg.Done after HandlerFunc2")
			defer e.wg.Done() // Decrement the counter when the goroutine completes
		}
		defer log.Debug("After HandlerFunc2 go routine is ending")
		e.startTempAuthCleanup(ctx)
	}()
	return nil
}

func (e *FxExchange) Pull(ctx context.Context, from peer.ID, l ipld.Link) error {
	if e.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.exchange")
	}

	cid := l.(cidlink.Link).Cid

	e.tempAuthsLock.Lock()
	e.tempAuths[cid.String()] = tempAuthEntry{peerID: from, timestamp: time.Now()}
	e.tempAuthsLock.Unlock()
	log.Debugw("Setting a temporary auth for Push in Pull", "from", from, "cid", cid.String(), "e.tempAuths", e.tempAuths)

	r := pullRequest{Link: cid}
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
		log.Debug("Successfully sent push request")
		return nil
	}
}
func (e *FxExchange) PullBlock(ctx context.Context, from peer.ID, l ipld.Link) error {
	if e.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.exchange")
	}

	cl := l.(cidlink.Link).Cid

	e.tempAuthsLock.Lock()
	e.tempAuths[cl.String()] = tempAuthEntry{peerID: from, timestamp: time.Now()}
	e.tempAuthsLock.Unlock()

	log.Debugw("Setting a temporary auth for Push in Pull", "from", from, "cid", cl.String(), "e.tempAuths", e.tempAuths)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "libp2p://"+from.String()+"/"+actionPull+"/"+cl.String(), nil)
	if err != nil {
		log.Errorw("Failed to instantiate pull request", "err", err)
		return err
	}
	switch resp, err := e.c.Do(req); {
	case err != nil:
		log.Errorw("Failed to read pull response", "err", err)
		return err
	case resp.StatusCode != http.StatusOK:
		defer resp.Body.Close()
		log.Errorw("Expected status accepted on pull response", "got", resp.StatusCode)
		if b, err := io.ReadAll(resp.Body); err != nil {
			log.Errorw("Failed to read response in non-OK status", "got", resp.StatusCode, "err", err)
			return fmt.Errorf("unexpected response: %d", resp.StatusCode)
		} else {
			return fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
	default:
		if err := e.decodeAndStoreBlock(ctx, resp.Body, l); err != nil {
			log.Errorw("Failed to pull block", "err", err)
			return err
		}
		log.Debug("Successfully sent push request")
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
			LinkTargetNodePrototypeChooser: bsfetcher.DefaultPrototypeChooser,
			LinkVisitOnlyOnce:              true,
		},
	}
	var eg errgroup.Group
	// Recursively traverse the node and push all its leaves.
	err = progress.WalkMatching(node, exploreAllRecursivelySelector, func(progress traversal.Progress, node datamodel.Node) error {
		eg.Go(func() error {
			// Create a new context for this specific push operation
			e.pushRateLimiter.Take()
			link, err := e.ls.ComputeLink(l.Prototype(), node)
			if err != nil {
				return err
			}
			return e.pushOneNode(ctx, node, to, link)
		})
		return nil
	})
	if err != nil {
		log.Errorw("Failed to traverse link during push", "err", err)
		return err
	}
	return eg.Wait()
}

func (e *FxExchange) pushOneNode(ctx context.Context, node ipld.Node, to peer.ID, link datamodel.Link) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var buf bytes.Buffer

	c := link.(cidlink.Link).Cid
	encoder, err := ipldmc.LookupEncoder(c.Prefix().Codec)
	if err != nil {
		log.Errorw("No encoder found to encode pushing node", "err", err)
		return err
	}

	if err := encoder(node, &buf); err != nil {
		log.Errorw("Failed encode node to be pushed", "err", err)
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "libp2p://"+to.String()+"/"+actionPush+"/"+c.String(), &buf)
	if err != nil {
		log.Errorw("Failed to instantiate push request", "err", err)
		return err
	}
	_, err = pushCircuitBreaker.Execute(func() (interface{}, error) {
		resp, err := e.c.Do(req)
		if err != nil {
			log.Errorw("Failed to do push request1", "err", err)
			return nil, err
		}
		defer resp.Body.Close()

		// Handle successful response with your existing logic
		b, err := io.ReadAll(resp.Body)
		switch {
		case err != nil:
			log.Errorw("Failed to read the response from push", "err", err)
			return nil, err // Signal an error to the circuit breaker
		case resp.StatusCode != http.StatusOK:
			log.Errorw("Received non-OK response from push", "err", err)
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		default:
			log.Debug("Successfully pushed traversed node")
			return nil, nil // Signal success to the circuit breaker
		}
	})
	// Handle potential errors and circuit breaker state
	if err != nil {
		switch {
		case errors.Is(err, gobreaker.ErrOpenState):
			// Circuit breaker is open (likely due to previous errors)
			return fmt.Errorf("circuit breaker open: %w", err)
		default:
			// Potentially retry or return the error based on its nature
			return retryWithBackoff(ctx, func() error {
				// Attempt to execute the request within the circuit breaker again
				_, err := pushCircuitBreaker.Execute(func() (interface{}, error) {
					resp, err := e.c.Do(req)
					if err != nil {
						log.Errorw("Failed to do push request on retry", "err", err)
						return nil, err // Signal failure to the circuit breaker
					}
					defer resp.Body.Close()

					// Handle successful response with your existing logic
					b, err := io.ReadAll(resp.Body)
					switch {
					case err != nil:
						log.Errorw("Failed to read the response from push", "err", err)
						return nil, err // Signal an error to the circuit breaker
					case resp.StatusCode != http.StatusOK:
						log.Errorw("Received non-OK response from push", "err", err)
						return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
					default:
						log.Debug("Successfully pushed traversed node")
						return nil, nil // Signal success to the circuit breaker
					}
				})
				return err // Return potential errors after circuit breaker execution attempt
			})
		}
	}
	return nil
}
func retryWithBackoff(ctx context.Context, fn func() error) error {
	maxRetries := 5
	baseDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		delay := baseDelay * time.Duration(math.Pow(2, float64(attempt)))
		jitter := time.Duration(rand.Float64() * float64(delay))
		select {
		case <-time.After(delay + jitter):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("failed after %d retries", maxRetries)
}

func (e *FxExchange) serve(w http.ResponseWriter, r *http.Request) {
	from, err := peer.Decode(r.RemoteAddr)
	if err != nil {
		log.Debugw("cannot parse remote addr as peer ID", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	var action, c string
	if len(segments) > 0 {
		action = segments[0]
	}
	if len(segments) > 1 {
		c = segments[1]
	}
	log.Debugw("serve", "action", action, "from", from, "c", c)
	if !e.authorized(from, action, c) {
		log.Debugw("rejected unauthorized request", "from", from, "action", action, "c", c)
		http.Error(w, errUnauthorized.Error(), http.StatusUnauthorized)
		return
	}
	switch action {
	case actionPull:
		e.handlePull(from, w, r, c)
	case actionPush:
		e.handlePush(from, w, r, c)
	case actionAuth:
		e.handleAuthorization(from, w, r)
	default:
		http.Error(w, "", http.StatusNotFound)
	}
}

func (e *FxExchange) handlePush(from peer.ID, w http.ResponseWriter, r *http.Request, c string) {
	log := log.With("action", actionPush, "from", from, "cid", c)
	if r.Method != http.MethodPost {
		log.Errorw("Only POST is allowed on push", "got", r.Method)
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}
	cidInPath, err := cid.Decode(c)
	if err != nil {
		log.Error("Invalid CID in path")
		http.Error(w, "invalid cid", http.StatusBadRequest)
		return
	}
	if err := e.decodeAndStoreBlock(r.Context(), r.Body, cidlink.Link{Cid: cidInPath}); err != nil {
		http.Error(w, "invalid cid", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (e *FxExchange) decodeAndStoreBlock(ctx context.Context, r io.ReadCloser, link ipld.Link) error {
	defer r.Close()
	codec := link.(cidlink.Link).Cid.Prefix().Codec
	decoder, err := ipldmc.LookupDecoder(codec)
	if err != nil {
		log.Errorw("No decoder found", "codec", codec)
		return err
	}
	nb := basicnode.Prototype.Any.NewBuilder()
	if err = decoder(nb, r); err != nil {
		log.Errorw("Failed to decode pushed node as dagcbor", "err", err)
		return err
	}
	storedLink, err := e.ls.Store(
		ipld.LinkContext{Ctx: ctx}, link.Prototype(), nb.Build())
	if err != nil {
		log.Errorw("Failed to store pushed node", "err", err)
		return err
	}
	log.Debugw("Successfully stored pushed node", "cid", storedLink.(cidlink.Link).Cid)
	log.Debugw("Notifying stored pushed link to IPNI publisher", "link", storedLink)
	e.pub.notifyReceivedLink(storedLink)
	log.Debugw("Successfully notified stored pushed link to IPNI publisher", "link", storedLink)
	return nil
}

func (e *FxExchange) handlePull(from peer.ID, w http.ResponseWriter, r *http.Request, c string) {
	log := log.With("action", actionPull, "from", from)
	switch r.Method {
	case http.MethodGet:
		cc, err := cid.Decode(c)
		if err != nil {
			log.Errorw("Cannot decode CID while handking GET /pull", "cid", c, "err", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		raw, err := e.ls.LoadRaw(ipld.LinkContext{Ctx: r.Context()}, cidlink.Link{Cid: cc})
		if err != nil {
			if errors.Is(err, datastore.ErrNotFound) {
				log.Errorw("Cannot find link locally while handing GET /pull", "cid", c)
				http.Error(w, "", http.StatusNotFound)
				return
			}
			log.Errorw("Failed to load link while handing GET /pull", "cid", c)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(raw); err != nil {
			log.Errorw("Failed to write response while handing GET /pull", "cid", c)
		}
		return
	case http.MethodPost: // Proceed; POST is allowed.
	default:
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
	log.Debug("Accepted pull request")
	log.Debug("Instantiating background push in response to pull request")
	ctx := context.TODO()
	if e.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.exchange")
	}
	if err := e.Push(ctx, from, cidlink.Link{Cid: p.Link}); err != nil {
		log.Errorw("Failed to fetch in response to push", "err", err)
	} else {
		log.Debug("Successfully finished background push in response to pull request")
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

func (e *FxExchange) authorized(pid peer.ID, action string, cid string) bool {
	log.Debugw("checking authorization", "pid", pid, "action", action, "cid", cid)
	if (e.authorizer == e.h.ID() || e.authorizer == "") && action != actionAuth { //to cover the cases where in poolHost mode
		// If no authorizer is set allow all.
		return true
	} else if e.authorizer == "" && action == actionAuth {
		return false
	}
	switch action {
	case actionPush:
		e.tempAuthsLock.Lock()
		defer e.tempAuthsLock.Unlock()

		log.Debugw("Temporary auth check", "e.tempAuths", e.tempAuths, "pid", pid, "cid", cid)
		// Retrieve the temporary authorization entry for the CID
		entry, ok := e.tempAuths[cid]
		if ok {
			// Check if the peer ID matches and the authorization hasn't expired
			if entry.peerID == pid {
				delete(e.tempAuths, cid) // Remove the temporary authorization after it's used
				log.Debugw("Temporary auth check passed", "pid", pid, "cid", cid)
				return true
			}
		}

		// Check for permanent authorization
		e.authorizedPeersLock.RLock()
		_, ok = e.authorizedPeers[pid]
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

func (e *FxExchange) startTempAuthCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.cleanupTempAuths()
		case <-ctx.Done():
			log.Debug("After HandlerFunc2 go routine (startTempAuthCleanup) should end now")
			return
		}
	}
}

func (e *FxExchange) cleanupTempAuths() {
	e.tempAuthsLock.Lock()
	defer e.tempAuthsLock.Unlock()

	for cid, entry := range e.tempAuths {
		if time.Since(entry.timestamp) >= 5*time.Minute {
			delete(e.tempAuths, cid)
		}
	}
}

func (e *FxExchange) Shutdown(ctx context.Context) error {
	log.Info("Started shutdown IPNI publisher")
	//if !e.ipniPublishDisabled {
	if err := e.pub.shutdown(); err != nil {
		log.Warnw("Failed to shutdown IPNI publisher gracefully", "err", err)
	}
	log.Info("Completed shutdown IPNI publisher")
	e.dht.dh.Close()
	e.dht.Shutdown()
	log.Info("Completed shutdown dht")
	//}
	e.c.CloseIdleConnections()
	log.Info("Completed CloseIdleConnections")
	return e.s.Shutdown(ctx)
}
