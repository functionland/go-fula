package exchange

import (
	"context"

	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Exchange interface {
	Start(context.Context) error
	Push(context.Context, peer.ID, ipld.Link) error
	// Pull recursively traverses the given link and syncs all its associated blocks and itself from the given peer ID.
	Pull(context.Context, peer.ID, ipld.Link) error
	// PullBlock Pulls a single block associated to the given link from the given peer ID.
	PullBlock(context.Context, peer.ID, ipld.Link) error
	SetAuth(context.Context, peer.ID, peer.ID, bool) error
	Shutdown(context.Context) error
	ShutdownIpfs(context.Context) error
	IpniNotifyLink(link ipld.Link)
	FindProvidersIpni(l ipld.Link, relays []string) ([]peer.AddrInfo, error)
}
