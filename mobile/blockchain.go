package fulamobile

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

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

type AssetsBalanceResponse struct {
	Amount uint64 `json:"amount"`
}

// AssetsBalance requests blox at Config.BloxAddr to get the balance of the account.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) AssetsBalance(account string, assetId int, classId int) ([]byte, error) {
	ctx := context.TODO()
	responseBytes, err := c.bl.AssetsBalance(ctx, c.bloxPid, blockchain.AssetsBalanceRequest{Account: account, AssetId: uint64(assetId), ClassId: uint64(classId)})
	if err != nil {
		return nil, err
	}

	// Decode the response into the temporary struct
	var tempResponse AssetsBalanceResponse
	err = json.Unmarshal(responseBytes, &tempResponse)
	if err != nil {
		return nil, err
	}

	// Construct a new response with Amount as a string
	modifiedResponse := map[string]string{
		"amount": strconv.FormatUint(tempResponse.Amount, 10),
	}

	// Re-encode the modified response to JSON
	modifiedResponseBytes, err := json.Marshal(modifiedResponse)
	if err != nil {
		return nil, err
	}

	return modifiedResponseBytes, nil
}

// AccountFund requests blox at Config.BloxAddr to fund the account.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) AccountFund(account string) ([]byte, error) {
	ctx := context.TODO()
	amountString := "1000000000000000000"

	// Create a new big.Int
	bigAmount := new(big.Int)
	_, ok := bigAmount.SetString(amountString, 10)
	if !ok {
		err := fmt.Errorf("error: the number %s is not valid", amountString)
		return nil, err
	}

	// Convert big.Int to blockchain.BigInt
	amount := blockchain.BigInt{Int: *bigAmount}
	return c.bl.AccountFund(ctx, c.bloxPid, blockchain.AccountFundRequest{Amount: amount, To: account})
}

func (c *Client) TransferToFula(amountStr string, walletAccount string, chain string) ([]byte, error) {
	ctx := context.TODO()
	// Convert amount from string to uint64
	amount, err := strconv.ParseUint(amountStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %v", err)
	}
	convertInput := blockchain.TransferToFulaRequest{
		Wallet: walletAccount,
		Amount: amount, // Use the converted amount
		Chain:  chain,
	}
	return c.bl.TransferToFula(ctx, c.bloxPid, convertInput)
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

func (c *Client) BatchUploadManifest(cidsBytes []byte, poolID int, replicationFactor int) ([]byte, error) {
	ctx := context.TODO()
	cidArray := strings.Split(string(cidsBytes), "|")
	return c.bl.ManifestBatchUpload(ctx, c.bloxPid, blockchain.ManifestBatchUploadMobileRequest{Cid: cidArray, PoolID: poolID, ReplicationFactor: replicationFactor})
}

func (c *Client) ReplicateInPool(cidsBytes []byte, account string, poolID int) []byte {
	ctx := context.TODO()
	cidArray := strings.Split(string(cidsBytes), "|")

	responseBytes, err := c.bl.ReplicateInPool(ctx, c.bloxPid, blockchain.ReplicateRequest{Cids: cidArray, Account: account, PoolID: poolID})
	if err != nil {
		// Convert the error to a JSON object and then to []byte
		errJSON := map[string]string{"error": err.Error()}
		errBytes, jsonErr := json.Marshal(errJSON)
		if jsonErr != nil {
			// In case JSON marshaling fails, fallback to a simple byte conversion of the error string
			return []byte("Error marshaling error to JSON: " + jsonErr.Error())
		}
		return errBytes
	}

	return responseBytes
}

//////////////////////////////////////////////////
/////////////////////HARDWARE/////////////////////
//////////////////////////////////////////////////

// BloxFreeSpace requests the blox avail/used free space information.
func (c *Client) BloxFreeSpace() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.BloxFreeSpace(ctx, c.bloxPid)
}

// EraseBlData requests the blox to erase the data related to blockchain
func (c *Client) EraseBlData() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.EraseBlData(ctx, c.bloxPid)
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

// GetAccount requests blox at Config.BloxAddr to get the balance of the account.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) GetAccount() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.GetAccount(ctx, c.bloxPid)
}

// GetAccount requests blox at Config.BloxAddr to get the balance of the account.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) FetchContainerLogs(ContainerName string, TailCount string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.FetchContainerLogs(ctx, c.bloxPid, wifi.FetchContainerLogsRequest{ContainerName: ContainerName, TailCount: TailCount})
}

// GetAccount requests blox at Config.BloxAddr to get the balance of the account.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) FindBestAndTargetInLogs(NodeContainerName string, TailCount string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.FindBestAndTargetInLogs(ctx, c.bloxPid, wifi.FindBestAndTargetInLogsRequest{NodeContainerName: NodeContainerName, TailCount: TailCount})
}

// GetAccount requests blox at Config.BloxAddr to get the balance of the account.
// the addr must be a valid multiaddr that includes peer ID.
func (c *Client) GetFolderSize(folderPath string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.GetFolderSize(ctx, c.bloxPid, wifi.GetFolderSizeRequest{FolderPath: folderPath})
}

func (c *Client) GetDatastoreSize() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.GetDatastoreSize(ctx, c.bloxPid, wifi.GetDatastoreSizeRequest{})
}
