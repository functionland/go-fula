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
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolCreate(seed string, poolName string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolCreate(ctx, c.bloxPid, blockchain.PoolCreateRequest{Seed: seed, PoolName: poolName, PeerID: c.bloxPid.String()})
}

// PoolJoin requests blox at Config.BloxAddr to join a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolJoin(seed string, poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolJoin(ctx, c.bloxPid, blockchain.PoolJoinRequest{Seed: seed, PoolID: poolID, PeerID: c.bloxPid.String()})
}

// PoolJoin requests blox at Config.BloxAddr to cancel a join request for a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolCancelJoin(seed string, poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolCancelJoin(ctx, c.bloxPid, blockchain.PoolCancelJoinRequest{Seed: seed, PoolID: poolID})
}

// PoolListRequests requests blox at Config.BloxAddr to list the join request for a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) PoolRequests(poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolRequests(ctx, c.bloxPid, blockchain.PoolRequestsRequest{PoolID: poolID})
}

// PoolList requests blox at Config.BloxAddr to list the pools.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) PoolList() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolList(ctx, c.bloxPid, blockchain.PoolListRequest{})
}

// PoolVote requests blox at Config.BloxAddr to vote for a join request.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolVote(seed string, poolID int, account string, voteValue bool) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolVote(ctx, c.bloxPid, blockchain.PoolVoteRequest{Seed: seed, PoolID: poolID, Account: account, VoteValue: voteValue})
}

// PoolLeave requests blox at Config.BloxAddr to leave a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolLeave(seed string, poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolLeave(ctx, c.bloxPid, blockchain.PoolLeaveRequest{Seed: seed, PoolID: poolID})
}

// ManifestUpload requests blox at Config.BloxAddr to add a manifest(upload request)
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestUpload(seed string, poolID int, ReplicationFactor int, uri string) ([]byte, error) {
	ctx := context.TODO()
	manifestJob := blockchain.ManifestJob{
		Work:   "IPFS",
		Engine: "Storage",
		Uri:    uri,
	}
	manifestMetadata := blockchain.ManifestMetadata{
		Job: manifestJob,
	}
	return c.bl.ManifestUpload(ctx, c.bloxPid, blockchain.ManifestUploadRequest{Seed: seed, PoolID: poolID, ReplicationFactor: ReplicationFactor, ManifestMetadata: manifestMetadata})
}

// ManifestStore requests blox at Config.BloxAddr to store a manifest(store request)
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestStore(seed string, poolID int, uploader string, cid string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ManifestStore(ctx, c.bloxPid, blockchain.ManifestStoreRequest{Seed: seed, PoolID: poolID, Uploader: uploader, Cid: cid})
}

// ManifestAvailable requests blox at Config.BloxAddr to list manifests
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestAvailable(poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ManifestAvailable(ctx, c.bloxPid, blockchain.ManifestAvailableRequest{PoolID: poolID})
}

// ManifestRemove requests blox at Config.BloxAddr to remove a manifest
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestRemove(seed string, poolID int, cid string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ManifestRemove(ctx, c.bloxPid, blockchain.ManifestRemoveRequest{Seed: seed, Cid: cid, PoolID: poolID})
}

// The uploader or admin can remove an account that is storing a given manifest.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestRemoveStorer(seed string, storage string, poolID int, cid string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ManifestRemoveStorer(ctx, c.bloxPid, blockchain.ManifestRemoveStorerRequest{Seed: seed, Storage: storage, Cid: cid, PoolID: poolID})
}

// The storer can stop storing a given manifest
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestRemoveStored(seed string, uploader string, poolID int, cid string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ManifestRemoveStored(ctx, c.bloxPid, blockchain.ManifestRemoveStoredRequest{Seed: seed, Uploader: uploader, Cid: cid, PoolID: poolID})
}
