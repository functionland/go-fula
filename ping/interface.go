package ping

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Ping interface {
	Start(context.Context) error
	Ping(context.Context, peer.ID) (int, int, error)
	StopServer(context.Context) error
	StopClient(context.Context)
	Shutdown(context.Context) error
}

type PingRequest struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type PingResponse struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}
