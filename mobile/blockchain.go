package fulamobile

import (
	"context"

	"github.com/functionland/go-fula/blockchain"
	wifi "github.com/functionland/go-fula/wap/pkg/wifi"
)

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

// AccountBalance requests blox at Config.BloxAddr to get the balance of the account.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) AccountBalance(account string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.AccountBalance(ctx, c.bloxPid, blockchain.AccountBalanceRequest{Account: account})
}

// AssetsBalance requests blox at Config.BloxAddr to get the balance of the account.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) AssetsBalance(assetId string, classId string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.AssetsBalance(ctx, c.bloxPid, blockchain.AssetsBalanceRequest{Account: "", AssetId: assetId, ClassId: classId})
}

// PoolJoin requests blox at Config.BloxAddr to join a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
func (c *Client) PoolJoin(poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolJoin(ctx, c.bloxPid, blockchain.PoolJoinRequest{PoolID: poolID, PeerID: c.bloxPid.String()})
}

// PoolJoin requests blox at Config.BloxAddr to cancel a join request for a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolCancelJoin(poolID int) ([]byte, error) {
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

// PoolLeave requests blox at Config.BloxAddr to leave a pool with the id.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
// TODO: This still needs rethink as someone should not be able to put another person PeerID in request
func (c *Client) PoolLeave(poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolLeave(ctx, c.bloxPid, blockchain.PoolLeaveRequest{PoolID: poolID})
}

// ManifestAvailable requests blox at Config.BloxAddr to list manifests
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) ManifestAvailable(poolID int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ManifestAvailable(ctx, c.bloxPid, blockchain.ManifestAvailableRequest{PoolID: poolID})
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

// Reboot requests the blox to reboot
func (c *Client) Reboot() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.Reboot(ctx, c.bloxPid)
}

// Reboot requests the blox to reboot
func (c *Client) DeleteWifi(name string) ([]byte, error) {
	ctx := context.TODO()
	// Create the DeleteWifiRequest
	req := wifi.DeleteWifiRequest{
		ConnectionName: name,
	}
	return c.bl.DeleteWifi(ctx, c.bloxPid, req)
}

// Reboot requests the blox to reboot
func (c *Client) DisconnectWifi(name string) ([]byte, error) {
	ctx := context.TODO()
	// Create the DeleteWifiRequest
	req := wifi.DeleteWifiRequest{
		ConnectionName: name,
	}
	return c.bl.DisconnectWifi(ctx, c.bloxPid, req)
}

// Partition requests the blox to partition ssd and nvme
func (c *Client) Partition() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.Partition(ctx, c.bloxPid)
}

// DeleteFulaConfig deletes config.yaml file
func (c *Client) DeleteFulaConfig() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.DeleteFulaConfig(ctx, c.bloxPid)
}
