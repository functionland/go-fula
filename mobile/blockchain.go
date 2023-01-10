package fulamobile

import (
	"context"

	"github.com/functionland/go-fula/blockchain"
)

// Seeded requests blox at Config.BloxAddr to create a seeded account.
// The seed must start with "/", and the addr must be a valid multiaddr that includes peer ID.
func (c *Client) Seeded(seed string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.Seeded(ctx, c.bloxPid, blockchain.SeededRequest{Seed: seed})
}

// AccountExists requests blox at Config.BloxAddr to check if the account exists or not.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) AccountExists(account string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.AccountExists(ctx, c.bloxPid, blockchain.AccountExistsRequest{Account: account})
}

// PoolCreate requests blox at Config.BloxAddr to creates a pool with the name.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
func (c *Client) PoolCreate(seed string, poolName string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolCreate(ctx, c.bloxPid, blockchain.PoolCreateRequest{Seed: seed, PoolName: poolName, PeerID: c.bloxPid.String()})
}
