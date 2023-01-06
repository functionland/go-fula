package exchange

import (
	"context"

	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Exchange interface {
	Start(context.Context) error
	Push(context.Context, peer.ID, ipld.Link) error
	Cmd(context.Context, peer.ID, string, string) error
	Pull(context.Context, peer.ID, ipld.Link) error
	SetAuth(context.Context, peer.ID, peer.ID, bool) error
	Shutdown(context.Context) error
}
