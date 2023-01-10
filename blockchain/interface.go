package blockchain

import (
	"context"
	"reflect"

	"github.com/libp2p/go-libp2p/core/peer"
)

type SeededRequest struct {
	Seed string `json:"seed"`
}

type SeededResponse struct {
	Seed    string `json:"seed"`
	Account string `json:"account"`
}

type Blockchain interface {
	Start(context.Context) error
	Seeded(context.Context, peer.ID, SeededRequest) ([]byte, error)
	SetAuth(context.Context, peer.ID, peer.ID, bool) error
	Shutdown(context.Context) error
}

const actionSeeded = "account-seeded"

var requestTypes = map[string]reflect.Type{
	actionSeeded: reflect.TypeOf(SeededRequest{}),
}

var responseTypes = map[string]reflect.Type{
	actionSeeded: reflect.TypeOf(SeededResponse{}),
}
