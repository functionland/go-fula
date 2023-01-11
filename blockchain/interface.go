package blockchain

import (
	"context"
	"reflect"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	actionSeeded               = "account-seeded"
	actionAccountExists        = "account-exists"
	actionPoolCreate           = "fula-pool-create"
	actionPoolJoin             = "fula-pool-join"
	actionPoolCancelJoin       = "fula-pool-cancel_join"
	actionPoolRequests         = "fula-pool-requests"
	actionPoolList             = "fula-pool-all"
	actionPoolVote             = "fula-pool-vote"
	actionPoolLeave            = "fula-pool-leave"
	actionManifestUpload       = "manifest-upload"
	actionManifestStore        = "manifest-storage"
	actionManifestAvailable    = "manifest-available"
	actionManifestRemove       = "manifest-remove"
	actionManifestRemoveStorer = "manifest-remove_storer"
	actionManifestRemoveStored = "manifest-remove_storing_manifest"
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

type PoolListResponse struct {
	PoolID       int      `json:"pool_id"`
	Owner        string   `json:"owner"`
	PoolName     string   `json:"pool_name"`
	Parent       string   `json:"parent"`
	Participants []string `json:"participants"`
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

type ManifestAvailableResponse struct {
	ReplicationAvailable int              `json:"replication_available"`
	ManifestMetadata     ManifestMetadata `json:"manifest_metadata"`
	PoolID               int              `json:"pool_id"`
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
	PoolCreate(context.Context, peer.ID, PoolCreateRequest) ([]byte, error)
	PoolJoin(context.Context, peer.ID, PoolJoinRequest) ([]byte, error)
	PoolCancelJoin(context.Context, peer.ID, PoolCancelJoinRequest) ([]byte, error)
	PoolListRequests(context.Context, peer.ID, PoolRequestsRequest) ([]byte, error)
	PoolList(context.Context, peer.ID, PoolListRequest) ([]byte, error)
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
	actionPoolCreate:           reflect.TypeOf(PoolCreateRequest{}),
	actionPoolJoin:             reflect.TypeOf(PoolJoinRequest{}),
	actionPoolCancelJoin:       reflect.TypeOf(PoolCancelJoinRequest{}),
	actionPoolList:             reflect.TypeOf(PoolListRequest{}),
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
	actionPoolCreate:           reflect.TypeOf(PoolCreateResponse{}),
	actionPoolJoin:             reflect.TypeOf(PoolJoinResponse{}),
	actionPoolCancelJoin:       reflect.TypeOf(PoolCancelJoinResponse{}),
	actionPoolList:             reflect.TypeOf(PoolListResponse{}),
	actionPoolVote:             reflect.TypeOf(PoolVoteResponse{}),
	actionPoolLeave:            reflect.TypeOf(PoolLeaveResponse{}),
	actionManifestUpload:       reflect.TypeOf(ManifestUploadResponse{}),
	actionManifestStore:        reflect.TypeOf(ManifestStoreResponse{}),
	actionManifestAvailable:    reflect.TypeOf(ManifestAvailableResponse{}),
	actionManifestRemove:       reflect.TypeOf(ManifestRemoveResponse{}),
	actionManifestRemoveStorer: reflect.TypeOf(ManifestRemoveStorerResponse{}),
	actionManifestRemoveStored: reflect.TypeOf(ManifestRemoveStoredResponse{}),
}
