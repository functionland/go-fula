package pool

import (
	"context"

	"github.com/ipfs/go-graphsync"
	gs "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipld/go-ipld-prime"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (p *Pool) startGraphExchange(ctx context.Context) {
	gsn := gsnet.NewFromLibp2pHost(p.h)
	p.gx = gs.New(ctx, gsn, p.ls)
	p.gx.RegisterIncomingRequestHook(
		func(p peer.ID, r graphsync.RequestData, ha graphsync.IncomingRequestHookActions) {
			// TODO check the peer is allowed and request is valid
			ha.ValidateRequest()
		})
}

func (p *Pool) Fetch(ctx context.Context, from peer.ID, l ipld.Link) error {
	if exists, err := p.Has(ctx, l); err != nil {
		return err
	} else if exists {
		return nil
	}
	resps, errs := p.gx.Request(ctx, from, l, selectorparse.CommonSelector_ExploreAllRecursively)
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
