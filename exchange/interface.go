package exchange

import (
	"context"

	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Exchange interface {
	Start(context.Context) error
	Push(context.Context, peer.ID, ipld.Link) error
	Pull(context.Context, peer.ID, ipld.Link) error
	SetAuth(context.Context, peer.ID, peer.ID, bool) error
	Shutdown(context.Context) error
	IpniNotifyLink(link ipld.Link)
	FindProvidersIpni(l ipld.Link, relays []string) ([]peer.AddrInfo, error)
}
