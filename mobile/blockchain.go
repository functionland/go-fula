package fulamobile

import (
	"context"

	"github.com/functionland/go-fula/blockchain"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	_ "github.com/ipld/go-ipld-prime/codec/raw"
)

// Seeded requests blox at Config.BloxAddr to create a seeded account.
// The seed must start with "/", and the addr must be a valid multiaddr that includes peer ID.
func (c *Client) Seeded(seed string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.Seeded(ctx, c.bloxPid, blockchain.SeededRequest{Seed: seed})
}
