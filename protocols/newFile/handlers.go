package newFile

import (
	"context"
	"fmt"
	"io"

	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
)

func Handle(ctx context.Context, api fxiface.CoreAPI, s network.Stream) error {
	fmt.Println("handling")

	req := &FSRequest{}
	buf, err := io.ReadAll(s)
	if err != nil {
		return err
	}
	proto.Unmarshal(buf, req)

	switch req.Action {
	case ActionType_READ:
		return HandleRead(ctx, api, req.Path, req.DID, s)
	default:
		return fmt.Errorf("Action Type not supported: %s", req.Action)
	}

	// s.Reset()
	// return nil
}

func HandleRead(ctx context.Context, api fxiface.CoreAPI, path string, userDID string, s network.Stream) error {
	return nil
}
