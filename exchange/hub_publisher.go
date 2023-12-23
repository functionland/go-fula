package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	p2phttp "github.com/libp2p/go-libp2p-http"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multihash"
)

type hubPublisher struct {
	*options
	h      host.Host
	buffer chan cid.Cid
	ctx    context.Context
	cancel context.CancelFunc
	client *http.Client
}

type PutContentRequest struct {
	Mutlihashes []multihash.Multihash
}

func newHubPublisher(h host.Host, opts *options) (*hubPublisher, error) {
	p := &hubPublisher{
		h:       h,
		options: opts,
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.buffer = make(chan cid.Cid, p.ipniPublishChanBuffer)

	tr := &http.Transport{}
	tr.RegisterProtocol("libp2p", p2phttp.NewTransport(h, p2phttp.ProtocolOption("/fx.land/hub/0.0.1")))
	p.client = &http.Client{Transport: tr}
	if err := p.assureConnectionToHub(p.ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *hubPublisher) assureConnectionToHub(ctx context.Context) error {
	a, err := peer.AddrInfoFromString("/dns/hub.dev.fx.land/tcp/40004/p2p/12D3KooWFmfEsXjWotvqJ6B3ASXx1w3p6udj8R9f344a9JTu2k4R")
	if err != nil {
		return err
	}
	p.h.Peerstore().AddAddrs(a.ID, a.Addrs, peerstore.PermanentAddrTTL)
	if err := p.h.Connect(ctx, *a); err != nil {
		log.Errorw("Failed to connect to hub", "err", err)
		return err
	}
	return nil
}
func (p *hubPublisher) Start(_ context.Context) error {
	go func() {
		unpublished := make(map[cid.Cid]struct{})
		var publishing atomic.Bool
		maybePublish := func() {
			remaining := len(unpublished)
			if remaining == 0 {
				log.Debug("No remaining entries to publish")
				return
			}
			if publishing.Load() {
				log.Debugw("IPNI publishing in progress", "remaining", remaining)
				return
			}
			log.Debugw("Attempting to publish links to IPNI", "count", remaining)
			mhs := make([]multihash.Multihash, 0, remaining)
			for c := range unpublished {
				mhs = append(mhs, c.Hash())
				delete(unpublished, c)
			}
			publishing.Store(true)
			go func(entries []multihash.Multihash) {
				log.Debug("IPNI publish attempt in progress...")
				defer func() {
					publishing.Store(false)
					log.Debug("Finished attempt to publish to IPNI.")
				}()
				if err := p.publish(entries); err != nil {
					log.Errorw("Failed to publish to IPNI", "entriesCount", len(mhs), "err", err)
				}
			}(mhs)
		}
		for {
			select {
			case <-p.ctx.Done():
				log.Infow("IPNI publisher stopped", "remainingLinks", len(unpublished))
				return
			case <-p.ipniPublishTicker.C:
				maybePublish()
			case c := <-p.buffer:
				unpublished[c] = struct{}{}
				maybePublish()
			}
		}
	}()
	return nil
}

func (p *hubPublisher) notifyReceivedLink(l ipld.Link) {
	if l == nil {
		return
	}
	link, ok := l.(cidlink.Link)
	if ok &&
		!cid.Undef.Equals(link.Cid) &&
		link.Cid.Prefix().MhType != multihash.IDENTITY {
		p.buffer <- link.Cid
	}
}

func (p *hubPublisher) publish(mhs []multihash.Multihash) error {
	if err := p.assureConnectionToHub(p.ctx); err != nil {
		return err
	}
	r := &PutContentRequest{
		Mutlihashes: mhs,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&r); err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPut, "libp2p://12D3KooWFmfEsXjWotvqJ6B3ASXx1w3p6udj8R9f344a9JTu2k4R/content", &buf)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(request)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("receievd non OK response from hub: %d", resp.StatusCode)
	}
	return nil
}

func (p *hubPublisher) shutdown() error {
	p.cancel()
	p.ipniPublishTicker.Stop()
	return nil
}
