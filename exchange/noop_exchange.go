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

func (n NoopExchange) Push(_ context.Context, to peer.ID, l ipld.Link) error {
	log.Debugw("Pushed noop exchange.", "to", to, "link", l)
	return nil
}

func (n NoopExchange) Pull(_ context.Context, from peer.ID, l ipld.Link) error {
	log.Debugw("Pulled noop exchange.", "from", from, "link", l)
	return nil
}

func (n NoopExchange) PullBlock(_ context.Context, from peer.ID, l ipld.Link) error {
	log.Debugw("Pulled block noop exchange.", "from", from, "link", l)
	return nil
}

func (n NoopExchange) SetAuth(_ context.Context, on peer.ID, subject peer.ID, allow bool) error {
	log.Debugw("Set auth noop exchange.", "on", on, "subject", subject, "allow", allow)
	return nil
}

func (n NoopExchange) Shutdown(context.Context) error {
	log.Debug("Shut down noop exchange.")
	return nil
}

func (n NoopExchange) IpniNotifyLink(l ipld.Link) {
	log.Debugw("IpniNotifyLink noop exchange.", "link", l)
}

func (n NoopExchange) FindProvidersIpni(l ipld.Link, relays []string) ([]peer.AddrInfo, error) {
	log.Debugw("FindProvidersIpni noop exchange.", "link", l)
	return []peer.AddrInfo{}, nil
}
