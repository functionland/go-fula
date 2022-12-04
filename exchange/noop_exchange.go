package exchange

import (
	"context"

	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core/peer"
)

var _ Exchange = (*NoopExchange)(nil)

type NoopExchange struct{}

func (n NoopExchange) Start(context.Context) error {
	log.Debug("Started noop exchange.")
	return nil
}

func (n NoopExchange) Push(ctx context.Context, to peer.ID, l ipld.Link) error {
	log.Debugw("Pushed noop exchange.", "to", to, "link", l)
	return nil
}

func (n NoopExchange) Pull(ctx context.Context, from peer.ID, l ipld.Link) error {
	log.Debugw("Pulled noop exchange.", "from", from, "link", l)
	return nil
}

func (n NoopExchange) Shutdown(ctx context.Context) error {
	log.Debug("Shut down noop exchange.")
	return nil
}
