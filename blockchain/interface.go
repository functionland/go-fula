package blockchain

import (
	"context"
	"reflect"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	actionSeeded        = "account-seeded"
	actionAccountExists = "account-exists"
	actionPoolCreate    = "pool-create"
	actionPoolJoin      = "pool-join"
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
	PeerID   string `json:"PeerIdTest"`
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

type Blockchain interface {
	Seeded(context.Context, peer.ID, SeededRequest) ([]byte, error)
	AccountExists(context.Context, peer.ID, AccountExistsRequest) ([]byte, error)
	PoolCreate(context.Context, peer.ID, PoolCreateRequest) ([]byte, error)
	PoolJoin(context.Context, peer.ID, PoolJoinRequest) ([]byte, error)
	SetAuth(context.Context, peer.ID, peer.ID, bool) error
}

var requestTypes = map[string]reflect.Type{
	actionSeeded:        reflect.TypeOf(SeededRequest{}),
	actionAccountExists: reflect.TypeOf(AccountExistsRequest{}),
	actionPoolCreate:    reflect.TypeOf(PoolCreateRequest{}),
	actionPoolJoin:      reflect.TypeOf(PoolJoinRequest{}),
}

var responseTypes = map[string]reflect.Type{
	actionSeeded:        reflect.TypeOf(SeededResponse{}),
	actionAccountExists: reflect.TypeOf(AccountExistsResponse{}),
	actionPoolCreate:    reflect.TypeOf(PoolCreateResponse{}),
	actionPoolJoin:      reflect.TypeOf(PoolJoinResponse{}),
}
