package blockchain

import (
	"context"
	"reflect"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	actionSeeded               = "account-seeded"
	actionAccountExists        = "account-exists"
	actionAccountCreate        = "account-create"
	actionPoolCreate           = "fula-pool-create"
	actionPoolJoin             = "fula-pool-join"
	actionPoolCancelJoin       = "fula-pool-cancel_join"
	actionPoolRequests         = "fula-pool-poolrequests"
	actionPoolList             = "fula-pool-all"
	actionPoolUserList         = "fula-pool-users"
	actionPoolVote             = "fula-pool-vote"
	actionPoolLeave            = "fula-pool-leave"
	actionManifestUpload       = "fula-manifest-upload"
	actionManifestStore        = "fula-manifest-storage"
	actionManifestAvailable    = "fula-manifest-available"
	actionManifestRemove       = "fula-manifest-remove"
	actionManifestRemoveStorer = "fula-manifest-remove_storer"
	actionManifestRemoveStored = "fula-manifest-remove_storing_manifest"
)

type SeededRequest struct {
	Seed string `json:"seed"`
}

type SeededResponse struct {
	Seed    string `json:"seed"`
	Account string `json:"account"`
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

type PoolCreateRequest struct {
	Seed     string `json:"seed"`
	PoolName string `json:"pool_name"`
	PeerID   string `json:"peer_id"`
}

type PoolCreateResponse struct {
	Owner  string `json:"owner"`
	PoolID int    `json:"pool_id"`
}

type PoolJoinRequest struct {
	Seed   string `json:"seed"`
	PoolID int    `json:"pool_id"`
	PeerID string `json:"peer_id"`
}

type PoolJoinResponse struct {
	Account string `json:"account"`
	PoolID  int    `json:"pool_id"`
}

type PoolCancelJoinRequest struct {
	Seed   string `json:"seed"`
	PoolID int    `json:"pool_id"`
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

type PoolUserListRequest struct {
	PoolID int `json:"pool_id"`
}

type User struct {
	PoolID        int    `json:"pool_id"`
	RequestPoolID int    `json:"request_pool_id"`
	Account       string `json:"account"`
	PeerID        string `json:"peer_id"`
}

type Pool struct {
	PoolID       int      `json:"pool_id"`
	Owner        string   `json:"owner"`
	PoolName     string   `json:"pool_name"`
	Parent       string   `json:"parent"`
	Participants []string `json:"participants"`
}

type PoolListResponse struct {
	Pools []Pool `json:"pools"`
}

type PoolUserListResponse struct {
	Users []User `json:"users"`
}

type PoolVoteRequest struct {
	Seed      string `json:"seed"`
	PoolID    int    `json:"pool_id"`
	Account   string `json:"account"`
	VoteValue bool   `json:"vote_value"`
}

type PoolVoteResponse struct {
	Account string `json:"account"`
	PoolID  int    `json:"pool_id"`
}

type PoolLeaveRequest struct {
	Seed   string `json:"seed"`
	PoolID int    `json:"pool_id"`
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
	Seed              string           `json:"seed"`
	PoolID            int              `json:"pool_id"`
	ReplicationFactor int              `json:"replication_factor"`
	ManifestMetadata  ManifestMetadata `json:"manifest_metadata"`
}

type ManifestUploadResponse struct {
	Uploader         string           `json:"uploader"`
	Storage          []string         `json:"storage"`
	ManifestMetadata ManifestMetadata `json:"manifest_metadata"`
	PoolID           int              `json:"pool_id"`
}

type ManifestStoreRequest struct {
	Uploader string `json:"uploader"`
	Seed     string `json:"seed"`
	Cid      string `json:"cid"`
	PoolID   int    `json:"pool_id"`
}

type ManifestStoreResponse struct {
	PoolID   int    `json:"pool_id"`
	Storage  string `json:"storage"`
	Uploader string `json:"uploader"`
	Cid      string `json:"cid"`
}

type ManifestAvailableRequest struct {
	PoolID int `json:"pool_id"`
}

type ManifestData struct {
	Uploader         string           `json:"uploader"`
	ManifestMetadata ManifestMetadata `json:"manifest_metadata"`
}

type Manifest struct {
	PoolID               int          `json:"pool_id"`
	ReplicationAvailable int          `json:"replication_available"`
	ManifestData         ManifestData `json:"manifest_data"`
}

type ManifestAvailableResponse struct {
	Manifests []Manifest `json:"manifests"`
}

type ManifestRemoveRequest struct {
	Seed   string `json:"seed"`
	Cid    string `json:"cid"`
	PoolID int    `json:"pool_id"`
}

type ManifestRemoveResponse struct {
	Uploader string `json:"uploader"`
	Cid      string `json:"cid"`
	PoolID   int    `json:"pool_id"`
}

type ManifestRemoveStorerRequest struct {
	Seed    string `json:"seed"`
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
	Seed     string `json:"seed"`
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

type Blockchain interface {
	Seeded(context.Context, peer.ID, SeededRequest) ([]byte, error)
	AccountExists(context.Context, peer.ID, AccountExistsRequest) ([]byte, error)
	AccountCreate(context.Context, peer.ID) ([]byte, error)
	PoolCreate(context.Context, peer.ID, PoolCreateRequest) ([]byte, error)
	PoolJoin(context.Context, peer.ID, PoolJoinRequest) ([]byte, error)
	PoolCancelJoin(context.Context, peer.ID, PoolCancelJoinRequest) ([]byte, error)
	PoolRequests(context.Context, peer.ID, PoolRequestsRequest) ([]byte, error)
	PoolList(context.Context, peer.ID, PoolListRequest) ([]byte, error)
	PoolUserList(context.Context, peer.ID, PoolUserListRequest) ([]byte, error)
	PoolVote(context.Context, peer.ID, PoolVoteRequest) ([]byte, error)
	PoolLeave(context.Context, peer.ID, PoolLeaveRequest) ([]byte, error)
	ManifestUpload(context.Context, peer.ID, ManifestUploadRequest) ([]byte, error)
	ManifestStore(context.Context, peer.ID, ManifestStoreRequest) ([]byte, error)
	ManifestAvailable(context.Context, peer.ID, ManifestAvailableRequest) ([]byte, error)
	ManifestRemove(context.Context, peer.ID, ManifestRemoveRequest) ([]byte, error)
	ManifestRemoveStorer(context.Context, peer.ID, ManifestRemoveStorerRequest) ([]byte, error)
	ManifestRemoveStored(context.Context, peer.ID, ManifestRemoveStoredRequest) ([]byte, error)
	SetAuth(context.Context, peer.ID, peer.ID, bool) error
}

var requestTypes = map[string]reflect.Type{
	actionSeeded:               reflect.TypeOf(SeededRequest{}),
	actionAccountExists:        reflect.TypeOf(AccountExistsRequest{}),
	actionAccountCreate:        reflect.TypeOf(AccountCreateRequest{}),
	actionPoolCreate:           reflect.TypeOf(PoolCreateRequest{}),
	actionPoolJoin:             reflect.TypeOf(PoolJoinRequest{}),
	actionPoolRequests:         reflect.TypeOf(PoolRequestsRequest{}),
	actionPoolCancelJoin:       reflect.TypeOf(PoolCancelJoinRequest{}),
	actionPoolList:             reflect.TypeOf(PoolListRequest{}),
	actionPoolUserList:         reflect.TypeOf(PoolUserListRequest{}),
	actionPoolVote:             reflect.TypeOf(PoolVoteRequest{}),
	actionPoolLeave:            reflect.TypeOf(PoolLeaveRequest{}),
	actionManifestUpload:       reflect.TypeOf(ManifestUploadRequest{}),
	actionManifestStore:        reflect.TypeOf(ManifestStoreRequest{}),
	actionManifestAvailable:    reflect.TypeOf(ManifestAvailableRequest{}),
	actionManifestRemove:       reflect.TypeOf(ManifestRemoveRequest{}),
	actionManifestRemoveStorer: reflect.TypeOf(ManifestRemoveStorerRequest{}),
	actionManifestRemoveStored: reflect.TypeOf(ManifestRemoveStoredRequest{}),
}

var responseTypes = map[string]reflect.Type{
	actionSeeded:               reflect.TypeOf(SeededResponse{}),
	actionAccountExists:        reflect.TypeOf(AccountExistsResponse{}),
	actionAccountCreate:        reflect.TypeOf(AccountCreateResponse{}),
	actionPoolCreate:           reflect.TypeOf(PoolCreateResponse{}),
	actionPoolJoin:             reflect.TypeOf(PoolJoinResponse{}),
	actionPoolRequests:         reflect.TypeOf(PoolRequestsResponse{}),
	actionPoolCancelJoin:       reflect.TypeOf(PoolCancelJoinResponse{}),
	actionPoolList:             reflect.TypeOf(PoolListResponse{}),
	actionPoolUserList:         reflect.TypeOf(PoolUserListResponse{}),
	actionPoolVote:             reflect.TypeOf(PoolVoteResponse{}),
	actionPoolLeave:            reflect.TypeOf(PoolLeaveResponse{}),
	actionManifestUpload:       reflect.TypeOf(ManifestUploadResponse{}),
	actionManifestStore:        reflect.TypeOf(ManifestStoreResponse{}),
	actionManifestAvailable:    reflect.TypeOf(ManifestAvailableResponse{}),
	actionManifestRemove:       reflect.TypeOf(ManifestRemoveResponse{}),
	actionManifestRemoveStorer: reflect.TypeOf(ManifestRemoveStorerResponse{}),
	actionManifestRemoveStored: reflect.TypeOf(ManifestRemoveStoredResponse{}),
}
