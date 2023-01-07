package blockchain

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Blockchain interface {
	Start(context.Context) error
	Seeded(context.Context, peer.ID, seededRequest) error
	SetAuth(context.Context, peer.ID, peer.ID, bool) error
	Shutdown(context.Context) error
}
