package exchange

import (
	"context"
	"sync/atomic"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipni/go-libipni/ingest/schema"
	"github.com/ipni/index-provider/engine"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/multiformats/go-multihash"
	"github.com/multiformats/go-varint"
)

var (
	ipniContextID = []byte("fx.land")
	ipniMetadata  = varint.ToUvarint(0xfe001)
)

type ipniPublisher struct {
	*options
	h      host.Host
	buffer chan cid.Cid
	e      *engine.Engine
	ctx    context.Context
	cancel context.CancelFunc
}

func newIpniPublisher(h host.Host, opts *options) (*ipniPublisher, error) {
	p := &ipniPublisher{
		h:       h,
		options: opts,
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.buffer = make(chan cid.Cid, p.ipniPublishChanBuffer)
	var err error
	p.e, err = engine.New(opts.ipniProviderEngineOpts...)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *ipniPublisher) Start(ctx context.Context) error {
	if err := p.e.Start(ctx); err != nil {
		return err
	}
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

func (p *ipniPublisher) notifyReceivedLink(l ipld.Link) {
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

func (p *ipniPublisher) publish(mhs []multihash.Multihash) error {
	chunk, err := schema.EntryChunk{
		Entries: mhs,
	}.ToNode()
	if err != nil {
		log.Errorw("Failed to instantiate entries chunk node", "err", err)
		return err
	}
	chunkLink, err := p.e.LinkSystem().Store(ipld.LinkContext{Ctx: p.ctx}, schema.Linkproto, chunk)
	if err != nil {
		log.Errorw("Failed to store chunk in IPNI provider engine", "err", err)
		return err
	}
	addrs := p.h.Addrs()
	saddrs := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		saddrs = append(saddrs, addr.String())
	}

	ad := schema.Advertisement{
		PreviousID: nil,
		Provider:   p.h.ID().String(),
		Addresses:  saddrs,
		Entries:    chunkLink,
		ContextID:  ipniContextID,
		Metadata:   ipniMetadata,
	}
	if err := ad.Sign(p.h.Peerstore().PrivKey(p.h.ID())); err != nil {
		log.Errorw("Failed to sign IPNI advertisement", "err", err)
		return err
	}
	adLink, err := p.e.Publish(p.ctx, ad)
	if err != nil {
		log.Errorw("Failed to publish IPNI advertisement", "err", err)
		return err
	}
	log.Infow("Successfully published ad to IPNI", "ad", adLink.String(), "entriesCount", len(mhs))
	return nil
}

func (p *ipniPublisher) shutdown() error {
	p.cancel()
	p.ipniPublishTicker.Stop()
	return p.e.Shutdown()
}
