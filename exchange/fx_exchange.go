package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-graphsync"
	gs "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

const FxExchangeProtocolID = "/fx.land/exchange/0.0.1"

var (
	log          = logging.Logger("fula/exchange")
	_   Exchange = (*FxExchange)(nil)
)

type (
	FxExchange struct {
		h  host.Host
		gx graphsync.GraphExchange
		ls ipld.LinkSystem
		s  *http.Server
		c  *http.Client
	}
	pushRequest struct {
		Link cid.Cid `json:"link"`
	}
)

func NewFxExchange(h host.Host, ls ipld.LinkSystem) *FxExchange {
	return &FxExchange{
		h:  h,
		ls: ls,
		s:  &http.Server{},
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
	}
}

func (e *FxExchange) Start(ctx context.Context) error {
	gsn := gsnet.NewFromLibp2pHost(e.h)
	e.gx = gs.New(ctx, gsn, e.ls)
	e.gx.RegisterIncomingRequestHook(
		func(p peer.ID, r graphsync.RequestData, ha graphsync.IncomingRequestHookActions) {
			// TODO check the peer is allowed and request is valid
			ha.ValidateRequest()
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
		case err, ok := <-errs:
			if !ok {
				return nil
			}
			log.Warnw("sync failed", "err", err)
		}
	}
}

func (e *FxExchange) Push(ctx context.Context, to peer.ID, l ipld.Link) error {
	r := pushRequest{Link: l.(cidlink.Link).Cid}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/push", &buf)
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
	switch path.Base(r.URL.Path) {
	case "push":
		e.handlePush(w, r)
	default:
		http.Error(w, "", http.StatusNotFound)
	}
}

func (e *FxExchange) handlePush(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorw("failed to read request body", "err", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	// TODO: verify source peer ID is allowed
	pid, err := peer.Decode(r.RemoteAddr)

	var p pushRequest
	if err := json.Unmarshal(b, &p); err != nil {
		log.Debugw("cannot parse request body", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	l := cidlink.Link{Cid: p.Link}
	ctx := context.TODO()
	if err != nil {
		log.Debugw("cannot parse remote addr as peer ID", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	go func() {
		if err := e.Pull(ctx, pid, l); err != nil {
			log.Warnw("failed to fetch in response to push", "err", err)
		} else {
			log.Debugw("successfully fetched in response to push", "from", pid, "link", p.Link)
		}
	}()
	w.WriteHeader(http.StatusAccepted)
}

func (e *FxExchange) Shutdown(ctx context.Context) error {
	e.c.CloseIdleConnections()
	return e.s.Shutdown(ctx)
}
