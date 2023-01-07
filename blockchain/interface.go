package blockchain

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
)

type SeededRequest struct {
	Seed string `json:"seed"`
}

type Blockchain interface {
	Start(context.Context) error
	Seeded(context.Context, peer.ID, SeededRequest) ([]byte, error)
	SetAuth(context.Context, peer.ID, peer.ID, bool) error
	Shutdown(context.Context) error
}
