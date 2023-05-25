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

// AccountCreate requests blox at Config.BloxAddr to create a account.
func (c *Client) AccountCreate() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.AccountCreate(ctx, c.bloxPid)
}

// AccountFund requests blox at Config.BloxAddr to fund the account.
// the addr must be a valid multiaddr that includes peer ID.
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) AccountFund(seed string, amount blockchain.BigInt, to string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.AccountFund(ctx, c.bloxPid, blockchain.AccountFundRequest{Amount: amount, To: to})
}

// AccountBalance requests blox at Config.BloxAddr to get the balance of the account.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) AccountBalance(account string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.AccountBalance(ctx, c.bloxPid, blockchain.AccountBalanceRequest{Account: account})
}

// PoolCreate requests blox at Config.BloxAddr to creates a pool with the name.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolCreate(seed string, poolName string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolCreate(ctx, c.bloxPid, blockchain.PoolCreateRequest{PoolName: poolName, PeerID: c.bloxPid.String()})
}

// PoolJoin requests blox at Config.BloxAddr to join a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolJoin(seed string, poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolJoin(ctx, c.bloxPid, blockchain.PoolJoinRequest{PoolID: poolID, PeerID: c.bloxPid.String()})
}

// PoolJoin requests blox at Config.BloxAddr to cancel a join request for a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolCancelJoin(seed string, poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolCancelJoin(ctx, c.bloxPid, blockchain.PoolCancelJoinRequest{PoolID: poolID})
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

// PoolUserList requests blox at Config.BloxAddr to list the input pool users.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) PoolUserList(poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolUserList(ctx, c.bloxPid, blockchain.PoolUserListRequest{PoolID: poolID})
}

// PoolVote requests blox at Config.BloxAddr to vote for a join request.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolVote(seed string, poolID int, account string, voteValue bool) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolVote(ctx, c.bloxPid, blockchain.PoolVoteRequest{PoolID: poolID, Account: account, VoteValue: voteValue})
}

// PoolLeave requests blox at Config.BloxAddr to leave a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolLeave(seed string, poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolLeave(ctx, c.bloxPid, blockchain.PoolLeaveRequest{PoolID: poolID})
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
	return c.bl.ManifestUpload(ctx, c.bloxPid, blockchain.ManifestUploadRequest{PoolID: poolID, ReplicationFactor: ReplicationFactor, ManifestMetadata: manifestMetadata})
}

// ManifestStore requests blox at Config.BloxAddr to store a manifest(store request)
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestStore(seed string, poolID int, uploader string, cid string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ManifestStore(ctx, c.bloxPid, blockchain.ManifestStoreRequest{PoolID: poolID, Uploader: uploader, Cid: cid})
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
	return c.bl.ManifestRemove(ctx, c.bloxPid, blockchain.ManifestRemoveRequest{Cid: cid, PoolID: poolID})
}

// The uploader or admin can remove an account that is storing a given manifest.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestRemoveStorer(seed string, storage string, poolID int, cid string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ManifestRemoveStorer(ctx, c.bloxPid, blockchain.ManifestRemoveStorerRequest{Storage: storage, Cid: cid, PoolID: poolID})
}

// The storer can stop storing a given manifest
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestRemoveStored(seed string, uploader string, poolID int, cid string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ManifestRemoveStored(ctx, c.bloxPid, blockchain.ManifestRemoveStoredRequest{Uploader: uploader, Cid: cid, PoolID: poolID})
}

//////////////////////////////////////////////////
/////////////////////HARDWARE/////////////////////
//////////////////////////////////////////////////

// BloxFreeSpace requests the blox avail/used free space information.
func (c *Client) BloxFreeSpace() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.BloxFreeSpace(ctx, c.bloxPid)
}

// WifiRemoveall requests the blox to remove all saved wifis
func (c *Client) WifiRemoveall() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.WifiRemoveall(ctx, c.bloxPid)
}
