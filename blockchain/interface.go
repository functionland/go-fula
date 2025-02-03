package blockchain

import (
	"context"
	"reflect"

	wifi "github.com/functionland/go-fula/wap/pkg/wifi"
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	actionSeeded                            = "account-seeded"
	actionAccountExists                     = "account-exists"
	actionAccountCreate                     = "account-create"
	actionAccountFund                       = "account-fund"
	actionAccountBalance                    = "account-balance"
	actionAssetsBalance                     = "asset-balance"
	actionTransferToMumbai                  = "fula-mumbai-convert_tokens"
	actionTransferToGoerli                  = "fula-goerli-convert_tokens"
	actionPoolCreate                        = "fula-pool-create"
	actionPoolJoin                          = "fula-pool-join"
	actionPoolCancelJoin                    = "fula-pool-cancel_join"
	actionPoolRequests                      = "fula-pool-poolrequests"
	actionPoolList                          = "fula-pool"
	actionPoolUserList                      = "fula-pool-users"
	actionPoolVote                          = "fula-pool-vote"
	actionPoolLeave                         = "fula-pool-leave"
	actionManifestUpload                    = "fula-manifest-upload"
	actionManifestStore                     = "fula-manifest-storage"
	actionManifestBatchStore                = "fula-manifest-batch_storage"
	actionManifestBatchUpload               = "fula-manifest-batch_upload"
	actionManifestAvailable                 = "fula-manifest-available"
	actionManifestRemove                    = "fula-manifest-remove"
	actionManifestRemoveStorer              = "fula-manifest-remove_storer"
	actionManifestRemoveStored              = "fula-manifest-remove_storing_manifest"
	actionManifestAvailableBatch            = "fula-manifest-available_batch"
	actionManifestAvailableAllaccountsBatch = "fula-manifest-available_allaccounts_batch"

	//Hardware
	actionBloxFreeSpace           = "blox-free-space"
	actionEraseBlData             = "erase-blockchain-data"
	actionWifiRemoveall           = "wifi-removeall"
	actionReboot                  = "reboot"
	actionPartition               = "partition"
	actionDeleteFulaConfig        = "delete-fula-config"
	actionDeleteWifi              = "delete-wifi"
	actionDisconnectWifi          = "disconnect-wifi"
	actionGetAccount              = "get-account"
	actionFetchContainerLogs      = "fetch-container-logs"
	actionFindBestAndTargetInLogs = "find-bestandtarget-inlogs"
	actionGetFolderSize           = "get-folder-size"
	actionGetDatastoreSize        = "get-datastore-size"

	// Cluster
	actionReplicateInPool = "replicate"

	// Plugins
	actionListPlugins       = "list-plugins"
	actionInstallPlugin     = "install-plugin"
	actionUninstallPlugin   = "uninstall-plugin"
	actionShowPluginStatus  = "show-plugin-status"
	actionListActivePlugins = "list-active-plugins"
	actionGetInstallOutput  = "get-install-output"
	actionGetInstallStatus  = "get-install-status"
	actionUpdatePlugin      = "update-plugin"

	// AI
	actionChatWithAI = "chat-ai"
)

type ReplicateRequest struct {
	Cids    []string `json:"cids"`
	Account string   `json:"uploader"`
	PoolID  int      `json:"pool_id"`
}

type AvailableAllaccountsBatchRequest struct {
	Cids   []string `json:"cids"`
	PoolID int      `json:"pool_id"`
}

type ReplicateResponse struct {
	Manifests []BatchManifest `json:"manifests"`
}

type BatchManifest struct {
	Cid                  string `json:"cid"`
	ReplicationAvailable int    `json:"replication_available"`
}

type BatchManifestAllaccountsResponse struct {
	Manifests []BatchManifestAllaccounts `json:"manifests"`
}

type BatchManifestAllaccounts struct {
	Cid string `json:"cid"`
}

type LinkWithLimit struct {
	Link  ipld.Link
	Limit int
}

type SeededRequest struct {
	//Seed string `json:"seed"`
}

type SeededResponse struct {
	Seed    string `json:"seed"`
	Account string `json:"account"`
}

type GetAccountResponse struct {
	Account string `json:"account"`
}
type GetAccountRequest struct {
}

type AccountExistsRequest struct {
	Account string `json:"account"`
}

type AccountExistsResponse struct {
	Account string `json:"account"`
	Exists  bool   `json:"exists"`
}
type AccountCreateRequest struct {
	Account string `json:"account"`
}

type AccountCreateResponse struct {
	Account string `json:"account"`
}

type AccountFundRequest struct {
	Amount BigInt `json:"amount"`
	To     string `json:"to"`
}
type AccountFundResponse struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Amount string `json:"amount"`
}

type AccountBalanceRequest struct {
	Account string `json:"account"`
}
type AccountBalanceResponse struct {
	Amount string `json:"amount"`
}

type AssetsBalanceRequest struct {
	Account string `json:"account"`
	ClassId uint64 `json:"class_id"`
	AssetId uint64 `json:"asset_id"`
}
type AssetsBalanceResponse struct {
	Amount uint64 `json:"amount"`
}
type MobileAssetsBalanceResponse struct {
	Amount string `json:"amount"`
}

type TransferToFulaRequest struct {
	Wallet string `json:"wallet_account"`
	Amount uint64 `json:"amount"`
	Chain  string `json:"chain"`
}
type TransferToFulaResponse struct {
	Msg         string `json:"msg"`
	Description string `json:"description"`
}

type PoolCreateRequest struct {
	PoolName string `json:"pool_name"`
	PeerID   string `json:"peer_id"`
}

type PoolCreateResponse struct {
	Owner  string `json:"owner"`
	PoolID int    `json:"pool_id"`
}

type PoolJoinRequest struct {
	PoolID int    `json:"pool_id"`
	PeerID string `json:"peer_id"`
}

type PoolJoinResponse struct {
	Account string `json:"account"`
	PoolID  int    `json:"pool_id"`
}

type PoolCancelJoinRequest struct {
	PoolID int `json:"pool_id"`
}

type PoolCancelJoinResponse struct {
	Account string `json:"account"`
	PoolID  int    `json:"pool_id"`
}

type PoolRequestsRequest struct {
	PoolID int `json:"pool_id"`
}

type PoolRequest struct {
	PoolID        int      `json:"pool_id"`
	Account       string   `json:"account"`
	Voted         []string `json:"voted"`
	PositiveVotes int      `json:"positive_votes"`
	PeerID        string   `json:"peer_id"`
}

type PoolRequestsResponse struct {
	PoolRequests []PoolRequest `json:"poolrequests"`
}

type PoolListRequest struct {
}

type PoolListRequestWithPoolId struct {
	PoolID int `json:"pool_id"`
}

type PoolUserListRequest struct {
	PoolID        int `json:"pool_id"`
	RequestPoolID int `json:"request_pool_id"`
}

type User struct {
	PoolID        *int   `json:"pool_id"`
	RequestPoolID *int   `json:"request_pool_id"`
	Account       string `json:"account"`
	PeerID        string `json:"peer_id"`
}

type Pool struct {
	PoolID       int      `json:"pool_id"`
	Creator      string   `json:"creator"`
	PoolName     string   `json:"pool_name"`
	Parent       string   `json:"parent"`
	Participants []string `json:"participants"`
	Region       string   `json:"region"`
}

type PoolListResponse struct {
	Pools []Pool `json:"pools"`
}

type PoolUserListResponse struct {
	Users []User `json:"users"`
}

type PoolVoteRequest struct {
	PoolID    int    `json:"pool_id"`
	Account   string `json:"account"`
	VoteValue bool   `json:"vote_value"`
	PeerID    string `json:"peer_id"`
}

type PoolVoteResponse struct {
	Account string `json:"account"`
	PoolID  int    `json:"pool_id"`
	Result  string `json:"result"`
}

type PoolLeaveRequest struct {
	PoolID int `json:"pool_id"`
}

type PoolLeaveResponse struct {
	Account string `json:"account"`
	PoolID  int    `json:"pool_id"`
}

type ManifestJob struct {
	Work   string `json:"work"`
	Engine string `json:"engine"`
	Uri    string `json:"uri"`
}

type ManifestMetadata struct {
	Job ManifestJob `json:"job"`
}

type ManifestUploadRequest struct {
	PoolID            int              `json:"pool_id"`
	ReplicationFactor int              `json:"replication_factor"`
	ManifestMetadata  ManifestMetadata `json:"manifest_metadata"`
	Cid               string           `json:"cid"`
}

type ManifestUploadResponse struct {
	Uploader         string           `json:"uploader"`
	Storage          []string         `json:"storage"`
	ManifestMetadata ManifestMetadata `json:"manifest_metadata"`
	PoolID           int              `json:"pool_id"`
}

type ManifestStoreRequest struct {
	Uploader string `json:"uploader"`
	Cid      string `json:"cid"`
	PoolID   int    `json:"pool_id"`
}

type ManifestStoreResponse struct {
	PoolID   int    `json:"pool_id"`
	Storage  string `json:"storage"`
	Uploader string `json:"uploader"`
	Cid      string `json:"cid"`
}

type ManifestBatchStoreRequest struct {
	Cid    []string `json:"cid"`
	PoolID int      `json:"pool_id"`
}

type ManifestBatchStoreResponse struct {
	PoolID int      `json:"pool_id"`
	Storer string   `json:"storer"`
	Cid    []string `json:"cid"`
}

type ManifestBatchUploadRequest struct {
	Cid               []string           `json:"cid"`
	PoolID            int                `json:"pool_id"`
	ReplicationFactor []int              `json:"replication_factor"`
	ManifestMetadata  []ManifestMetadata `json:"manifest_metadata"`
}

type ManifestBatchUploadMobileRequest struct {
	Cid               []string `json:"cid"`
	PoolID            int      `json:"pool_id"`
	ReplicationFactor int      `json:"replication_factor"`
}

type ManifestBatchUploadResponse struct {
	PoolID int      `json:"pool_id"`
	Storer string   `json:"storer"`
	Cid    []string `json:"cid"`
}

type ManifestAvailableRequest struct {
	PoolID int `json:"pool_id"`
}

type ManifestData struct {
	Uploader         string           `json:"uploader"`
	ManifestMetadata ManifestMetadata `json:"manifest_metadata"`
}

type Manifest struct {
	PoolID               int              `json:"pool_id"`
	ReplicationAvailable int              `json:"replication_available"`
	ManifestMetadata     ManifestMetadata `json:"manifest_metadata"`
}

type ManifestAvailableResponse struct {
	Manifests []Manifest `json:"manifests"`
}

type ManifestRemoveRequest struct {
	Cid    string `json:"cid"`
	PoolID int    `json:"pool_id"`
}

type ManifestRemoveResponse struct {
	Uploader string `json:"uploader"`
	Cid      string `json:"cid"`
	PoolID   int    `json:"pool_id"`
}

type ManifestRemoveStorerRequest struct {
	Storage string `json:"storage"`
	Cid     string `json:"cid"`
	PoolID  int    `json:"pool_id"`
}

type ManifestRemoveStorerResponse struct {
	Uploader string `json:"uploader"`
	Storage  string `json:"storage"`
	Cid      string `json:"cid"`
	PoolID   int    `json:"pool_id"`
}

type ManifestRemoveStoredRequest struct {
	Uploader string `json:"uploader"`
	Cid      string `json:"cid"`
	PoolID   int    `json:"pool_id"`
}

type ManifestRemoveStoredResponse struct {
	Uploader string `json:"uploader"`
	Storage  string `json:"storage"`
	Cid      string `json:"cid"`
	PoolID   int    `json:"pool_id"`
}

// Plugins
// Plugin structures

// ListPlugins
type ListPluginsRequest struct{}

type ListPluginsResponse struct {
	Plugins []PluginInfo `json:"plugins"`
}

// InstallPlugin
type InstallPluginRequest struct {
	PluginName string `json:"plugin_name"`
	Params     string `json:"params"`
}

type InstallPluginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// UninstallPlugin
type UninstallPluginRequest struct {
	PluginName string `json:"plugin_name"`
}

type UninstallPluginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ShowPluginStatus
type ShowPluginStatusRequest struct {
	PluginName string `json:"plugin_name"`
	Lines      int    `json:"lines"`
	Follow     bool   `json:"follow"`
}

type ShowPluginStatusResponse struct {
	Status []string `json:"status"`
}

type ListActivePluginsRequest struct{}
type ListActivePluginsResponse struct {
	ActivePlugins []string `json:"active_plugins"`
}

// GetInstallOutput
type GetInstallOutputRequest struct {
	PluginName string `json:"plugin_name"`
	Params     string `json:"params"`
}

type GetInstallOutputResponse struct {
	Outputs map[string]string `json:"outputs"`
}

// GetInstallStatus
type GetInstallStatusRequest struct {
	PluginName string `json:"plugin_name"`
}

type GetInstallStatusResponse struct {
	Status string `json:"status"`
}

type UpdatePluginRequest struct {
	PluginName string `json:"plugin_name"`
}
type UpdatePluginResponse struct {
	Msg    string `json:"msg"`
	Status bool   `json:"status"`
}

type Blockchain interface {
	Seeded(context.Context, peer.ID, SeededRequest) ([]byte, error)
	AccountExists(context.Context, peer.ID, AccountExistsRequest) ([]byte, error)
	AccountCreate(context.Context, peer.ID) ([]byte, error)
	AccountFund(context.Context, peer.ID, AccountFundRequest) ([]byte, error)
	AccountBalance(context.Context, peer.ID, AccountBalanceRequest) ([]byte, error)
	AssetsBalance(context.Context, peer.ID, AssetsBalanceRequest) ([]byte, error)
	TransferToFula(context.Context, peer.ID, TransferToFulaRequest) ([]byte, error)
	PoolCreate(context.Context, peer.ID, PoolCreateRequest) ([]byte, error)
	PoolJoin(context.Context, peer.ID, PoolJoinRequest) ([]byte, error)
	PoolCancelJoin(context.Context, peer.ID, PoolCancelJoinRequest) ([]byte, error)
	PoolRequests(context.Context, peer.ID, PoolRequestsRequest) ([]byte, error)
	PoolList(context.Context, peer.ID, PoolListRequest) ([]byte, error)
	PoolUserList(context.Context, peer.ID, PoolUserListRequest) ([]byte, error)
	PoolVote(context.Context, peer.ID, PoolVoteRequest) ([]byte, error)
	PoolLeave(context.Context, peer.ID, PoolLeaveRequest) ([]byte, error)
	ManifestUpload(context.Context, peer.ID, ManifestUploadRequest) ([]byte, error)
	ManifestBatchStore(context.Context, peer.ID, ManifestBatchStoreRequest) ([]byte, error)
	ManifestBatchUpload(context.Context, peer.ID, ManifestBatchUploadMobileRequest) ([]byte, error)
	ReplicateInPool(context.Context, peer.ID, ReplicateRequest) ([]byte, error)
	ManifestStore(context.Context, peer.ID, ManifestStoreRequest) ([]byte, error)
	ManifestAvailable(context.Context, peer.ID, ManifestAvailableRequest) ([]byte, error)
	ManifestRemove(context.Context, peer.ID, ManifestRemoveRequest) ([]byte, error)
	ManifestRemoveStorer(context.Context, peer.ID, ManifestRemoveStorerRequest) ([]byte, error)
	ManifestRemoveStored(context.Context, peer.ID, ManifestRemoveStoredRequest) ([]byte, error)
	SetAuth(context.Context, peer.ID, peer.ID, bool) error

	//Hardware
	BloxFreeSpace(context.Context, peer.ID) ([]byte, error)
	EraseBlData(context.Context, peer.ID) ([]byte, error)
	WifiRemoveall(context.Context, peer.ID) ([]byte, error)
	Reboot(context.Context, peer.ID) ([]byte, error)
	DeleteWifi(context.Context, peer.ID, wifi.DeleteWifiRequest) ([]byte, error)
	DisconnectWifi(context.Context, peer.ID, wifi.DeleteWifiRequest) ([]byte, error)
	Partition(context.Context, peer.ID) ([]byte, error)
	DeleteFulaConfig(context.Context, peer.ID) ([]byte, error)
	GetAccount(context.Context, peer.ID) ([]byte, error)
	FetchContainerLogs(context.Context, peer.ID, wifi.FetchContainerLogsRequest) ([]byte, error)
	FindBestAndTargetInLogs(context.Context, peer.ID, wifi.FindBestAndTargetInLogsRequest) ([]byte, error)
	GetFolderSize(context.Context, peer.ID, wifi.GetFolderSizeRequest) ([]byte, error)
	GetDatastoreSize(context.Context, peer.ID, wifi.GetDatastoreSizeRequest) ([]byte, error)

	//Plugins
	ListPlugins(context.Context, peer.ID) ([]byte, error)
	ListActivePlugins(context.Context, peer.ID) ([]byte, error)
	InstallPlugin(context.Context, peer.ID, string, string) ([]byte, error)
	UninstallPlugin(context.Context, peer.ID, string) ([]byte, error)
	ShowPluginStatus(context.Context, string, int) ([]byte, error)
	GetInstallOutput(context.Context, peer.ID, string, string) ([]byte, error)
	GetInstallStatus(context.Context, peer.ID, string) ([]byte, error)
	UpdatePlugin(context.Context, peer.ID, string) ([]byte, error)

	// AI
	ChatWithAI(context.Context, peer.ID, wifi.ChatWithAIRequest) (*StreamBuffer, error)
}

var requestTypes = map[string]reflect.Type{
	actionSeeded:                            reflect.TypeOf(SeededRequest{}),
	actionAccountExists:                     reflect.TypeOf(AccountExistsRequest{}),
	actionAccountCreate:                     reflect.TypeOf(AccountCreateRequest{}),
	actionAccountFund:                       reflect.TypeOf(AccountFundRequest{}),
	actionPoolCreate:                        reflect.TypeOf(PoolCreateRequest{}),
	actionPoolJoin:                          reflect.TypeOf(PoolJoinRequest{}),
	actionPoolRequests:                      reflect.TypeOf(PoolRequestsRequest{}),
	actionPoolCancelJoin:                    reflect.TypeOf(PoolCancelJoinRequest{}),
	actionPoolList:                          reflect.TypeOf(PoolListRequest{}),
	actionPoolUserList:                      reflect.TypeOf(PoolUserListRequest{}),
	actionPoolVote:                          reflect.TypeOf(PoolVoteRequest{}),
	actionPoolLeave:                         reflect.TypeOf(PoolLeaveRequest{}),
	actionManifestUpload:                    reflect.TypeOf(ManifestUploadRequest{}),
	actionManifestStore:                     reflect.TypeOf(ManifestStoreRequest{}),
	actionManifestBatchStore:                reflect.TypeOf(ManifestBatchStoreRequest{}),
	actionManifestBatchUpload:               reflect.TypeOf(ManifestBatchUploadRequest{}),
	actionManifestAvailable:                 reflect.TypeOf(ManifestAvailableRequest{}),
	actionManifestAvailableBatch:            reflect.TypeOf(ReplicateRequest{}),
	actionManifestAvailableAllaccountsBatch: reflect.TypeOf(AvailableAllaccountsBatchRequest{}),
	actionManifestRemove:                    reflect.TypeOf(ManifestRemoveRequest{}),
	actionManifestRemoveStorer:              reflect.TypeOf(ManifestRemoveStorerRequest{}),
	actionManifestRemoveStored:              reflect.TypeOf(ManifestRemoveStoredRequest{}),
	actionAssetsBalance:                     reflect.TypeOf(AssetsBalanceRequest{}),
	actionTransferToGoerli:                  reflect.TypeOf(TransferToFulaRequest{}),
	actionTransferToMumbai:                  reflect.TypeOf(TransferToFulaRequest{}),
	actionReplicateInPool:                   reflect.TypeOf(ReplicateRequest{}),

	//Hardware
	actionBloxFreeSpace:           reflect.TypeOf(wifi.BloxFreeSpaceRequest{}),
	actionEraseBlData:             reflect.TypeOf(wifi.EraseBlDataRequest{}),
	actionWifiRemoveall:           reflect.TypeOf(wifi.WifiRemoveallRequest{}),
	actionReboot:                  reflect.TypeOf(wifi.RebootRequest{}),
	actionPartition:               reflect.TypeOf(wifi.PartitionRequest{}),
	actionDeleteFulaConfig:        reflect.TypeOf(wifi.DeleteFulaConfigRequest{}),
	actionDeleteWifi:              reflect.TypeOf(wifi.DeleteWifiRequest{}),
	actionDisconnectWifi:          reflect.TypeOf(wifi.DeleteWifiRequest{}),
	actionGetAccount:              reflect.TypeOf(GetAccountRequest{}),
	actionFetchContainerLogs:      reflect.TypeOf(wifi.FetchContainerLogsRequest{}),
	actionFindBestAndTargetInLogs: reflect.TypeOf(wifi.FindBestAndTargetInLogsRequest{}),
	actionGetFolderSize:           reflect.TypeOf(wifi.GetFolderSizeRequest{}),
	actionGetDatastoreSize:        reflect.TypeOf(wifi.GetDatastoreSizeRequest{}),

	// Plugins
	actionListPlugins:       reflect.TypeOf(ListPluginsRequest{}),
	actionListActivePlugins: reflect.TypeOf(ListActivePluginsRequest{}),
	actionInstallPlugin:     reflect.TypeOf(InstallPluginRequest{}),
	actionUninstallPlugin:   reflect.TypeOf(UninstallPluginRequest{}),
	actionShowPluginStatus:  reflect.TypeOf(ShowPluginStatusRequest{}),
	actionGetInstallOutput:  reflect.TypeOf(GetInstallOutputRequest{}),
	actionGetInstallStatus:  reflect.TypeOf(GetInstallStatusRequest{}),
	actionUpdatePlugin:      reflect.TypeOf(UpdatePluginRequest{}),

	// AI
	actionChatWithAI: reflect.TypeOf(wifi.ChatWithAIRequest{}),
}

var responseTypes = map[string]reflect.Type{
	actionSeeded:                            reflect.TypeOf(SeededResponse{}),
	actionAccountExists:                     reflect.TypeOf(AccountExistsResponse{}),
	actionAccountCreate:                     reflect.TypeOf(AccountCreateResponse{}),
	actionAccountFund:                       reflect.TypeOf(AccountFundResponse{}),
	actionPoolCreate:                        reflect.TypeOf(PoolCreateResponse{}),
	actionPoolJoin:                          reflect.TypeOf(PoolJoinResponse{}),
	actionPoolRequests:                      reflect.TypeOf(PoolRequestsResponse{}),
	actionPoolCancelJoin:                    reflect.TypeOf(PoolCancelJoinResponse{}),
	actionPoolList:                          reflect.TypeOf(PoolListResponse{}),
	actionPoolUserList:                      reflect.TypeOf(PoolUserListResponse{}),
	actionPoolVote:                          reflect.TypeOf(PoolVoteResponse{}),
	actionPoolLeave:                         reflect.TypeOf(PoolLeaveResponse{}),
	actionManifestUpload:                    reflect.TypeOf(ManifestUploadResponse{}),
	actionManifestStore:                     reflect.TypeOf(ManifestStoreResponse{}),
	actionManifestBatchStore:                reflect.TypeOf(ManifestBatchStoreResponse{}),
	actionManifestBatchUpload:               reflect.TypeOf(ManifestBatchUploadResponse{}),
	actionManifestAvailable:                 reflect.TypeOf(ManifestAvailableResponse{}),
	actionManifestAvailableBatch:            reflect.TypeOf(ReplicateResponse{}),
	actionManifestAvailableAllaccountsBatch: reflect.TypeOf(BatchManifestAllaccountsResponse{}),
	actionManifestRemove:                    reflect.TypeOf(ManifestRemoveResponse{}),
	actionManifestRemoveStorer:              reflect.TypeOf(ManifestRemoveStorerResponse{}),
	actionManifestRemoveStored:              reflect.TypeOf(ManifestRemoveStoredResponse{}),
	actionAssetsBalance:                     reflect.TypeOf(AssetsBalanceResponse{}),
	actionTransferToGoerli:                  reflect.TypeOf(TransferToFulaResponse{}),
	actionTransferToMumbai:                  reflect.TypeOf(TransferToFulaResponse{}),
	actionReplicateInPool:                   reflect.TypeOf(ReplicateResponse{}),

	//Hardware
	actionBloxFreeSpace:           reflect.TypeOf(wifi.BloxFreeSpaceResponse{}),
	actionEraseBlData:             reflect.TypeOf(wifi.EraseBlDataResponse{}),
	actionWifiRemoveall:           reflect.TypeOf(wifi.WifiRemoveallResponse{}),
	actionReboot:                  reflect.TypeOf(wifi.RebootResponse{}),
	actionPartition:               reflect.TypeOf(wifi.PartitionResponse{}),
	actionDeleteFulaConfig:        reflect.TypeOf(wifi.DeleteFulaConfigResponse{}),
	actionDeleteWifi:              reflect.TypeOf(wifi.DeleteWifiResponse{}),
	actionDisconnectWifi:          reflect.TypeOf(wifi.DeleteWifiResponse{}),
	actionGetAccount:              reflect.TypeOf(GetAccountResponse{}),
	actionFetchContainerLogs:      reflect.TypeOf(wifi.FetchContainerLogsResponse{}),
	actionFindBestAndTargetInLogs: reflect.TypeOf(wifi.FindBestAndTargetInLogsResponse{}),
	actionGetFolderSize:           reflect.TypeOf(wifi.GetFolderSizeResponse{}),
	actionGetDatastoreSize:        reflect.TypeOf(wifi.GetDatastoreSizeResponse{}),

	// Plugins
	actionListPlugins:       reflect.TypeOf(ListPluginsResponse{}),
	actionListActivePlugins: reflect.TypeOf(ListActivePluginsResponse{}),
	actionInstallPlugin:     reflect.TypeOf(InstallPluginResponse{}),
	actionUninstallPlugin:   reflect.TypeOf(UninstallPluginResponse{}),
	actionShowPluginStatus:  reflect.TypeOf(ShowPluginStatusResponse{}),
	actionGetInstallOutput:  reflect.TypeOf(GetInstallOutputResponse{}),
	actionGetInstallStatus:  reflect.TypeOf(GetInstallStatusResponse{}),
	actionUpdatePlugin:      reflect.TypeOf(UpdatePluginResponse{}),

	// AI
	actionChatWithAI: reflect.TypeOf(wifi.ChatWithAIResponse{}),
}
