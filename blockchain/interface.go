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
	PoolID bool   `json:"pool_id"`
}

type Blockchain interface {
	Seeded(context.Context, peer.ID, SeededRequest) ([]byte, error)
	AccountExists(context.Context, peer.ID, AccountExistsRequest) ([]byte, error)
	PoolCreate(context.Context, peer.ID, PoolCreateRequest) ([]byte, error)
	SetAuth(context.Context, peer.ID, peer.ID, bool) error
}

var requestTypes = map[string]reflect.Type{
	actionSeeded:        reflect.TypeOf(SeededRequest{}),
	actionAccountExists: reflect.TypeOf(AccountExistsRequest{}),
	actionPoolCreate:    reflect.TypeOf(PoolCreateRequest{}),
}

var responseTypes = map[string]reflect.Type{
	actionSeeded:        reflect.TypeOf(SeededResponse{}),
	actionAccountExists: reflect.TypeOf(AccountExistsResponse{}),
	actionPoolCreate:    reflect.TypeOf(PoolCreateResponse{}),
}
