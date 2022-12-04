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
	Shutdown(context.Context) error
}
