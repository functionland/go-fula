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
	"github.com/google/uuid"
)

type PluginParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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

// PoolJoinWithChain requests blox at Config.BloxAddr to join a pool with the id on a specific chain.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
func (c *Client) PoolJoinWithChain(poolID int, chainName string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolJoin(ctx, c.bloxPid, blockchain.PoolJoinRequest{
		PoolID:    poolID,
		PeerID:    c.bloxPid.String(),
		ChainName: chainName,
	})
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

// PoolLeaveWithChain requests blox at Config.BloxAddr to leave a pool with the id on a specific chain.
// the addr must be a valid multiaddr that includes peer ID.
// Note that this call is only allowed on a user's own blox
func (c *Client) PoolLeaveWithChain(poolID int, chainName string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.PoolLeave(ctx, c.bloxPid, blockchain.PoolLeaveRequest{
		PoolID:    poolID,
		ChainName: chainName,
	})
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

func (c *Client) ChatWithAI(aiModel string, userMessage string) ([]byte, error) {
	ctx := context.TODO()

	buffer, err := c.bl.ChatWithAI(ctx, c.bloxPid, wifi.ChatWithAIRequest{
		AIModel:     aiModel,
		UserMessage: userMessage,
	})
	if err != nil {
		return nil, fmt.Errorf("error starting ChatWithAI: %v", err)
	}

	streamID := uuid.New().String() // Generate a unique stream ID
	c.mu.Lock()
	c.streams[streamID] = buffer // Store the StreamBuffer in the map
	c.mu.Unlock()

	return []byte(streamID), nil // Return the stream ID as a byte slice
}

func (c *Client) GetChatChunk(streamID string) (string, error) {
	c.mu.Lock()
	buffer, ok := c.streams[streamID]
	c.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("invalid stream ID")
	}

	for {
		chunk, err := buffer.GetChunk()
		if err != nil { // Stream closed or errored out
			c.mu.Lock()
			delete(c.streams, streamID) // Remove the stream from the map
			c.mu.Unlock()
			return "", err
		}

		// Skip empty chunks and wait for a valid chunk
		if chunk != "" {
			return chunk, nil
		}
	}
}

func (c *Client) GetStreamIterator(streamID string) (*StreamIterator, error) {
	c.mu.Lock()
	buffer, ok := c.streams[streamID]
	c.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("invalid stream ID")
	}

	return &StreamIterator{buffer: buffer}, nil
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

// ListPlugins requests the blox to list all available plugins
func (c *Client) ListPlugins() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ListPlugins(ctx, c.bloxPid)
}

func (c *Client) ListActivePlugins() ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ListActivePlugins(ctx, c.bloxPid)
}

// InstallPlugin requests the blox to install a specific plugin
func (c *Client) InstallPlugin(pluginName string, params string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.InstallPlugin(ctx, c.bloxPid, pluginName, params)
}

// UninstallPlugin requests the blox to uninstall a specific plugin
func (c *Client) UninstallPlugin(pluginName string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.UninstallPlugin(ctx, c.bloxPid, pluginName)
}

// ShowPluginStatus requests the status of a specific plugin
func (c *Client) ShowPluginStatus(pluginName string, lines int) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.ShowPluginStatus(ctx, pluginName, lines)
}

// InstallPlugin requests the blox to install a specific plugin
func (c *Client) GetInstallOutput(pluginName string, params string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.GetInstallOutput(ctx, c.bloxPid, pluginName, params)
}

// InstallPlugin requests the blox to install a specific plugin
func (c *Client) GetInstallStatus(pluginName string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.GetInstallStatus(ctx, c.bloxPid, pluginName)
}

func (c *Client) UpdatePlugin(pluginName string) ([]byte, error) {
	ctx := context.TODO()
	return c.bl.UpdatePlugin(ctx, c.bloxPid, pluginName)
}
