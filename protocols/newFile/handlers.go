package newFile

import (
	"context"
	"fmt"
	"io"

	"github.com/functionland/go-fula/drive"
	fxiface "github.com/functionland/go-fula/fxfs/core/iface"

	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
)

func Handle(ctx context.Context, api fxiface.CoreAPI, ds drive.DriveStore, s network.Stream) error {
	fmt.Println("handling")

	req := &FSRequest{}
	buf, err := io.ReadAll(s)
	if err != nil {
		return err
	}
	proto.Unmarshal(buf, req)

	switch req.Action {
	case ActionType_READ:
		return HandleRead(ctx, api, ds, req.Path, req.DID, s)
	default:
		return fmt.Errorf("Action Type not supported: %s", req.Action)
	}

	// s.Reset()
	// return nil
}

// Read a file at a given path from the user's drive
func HandleRead(ctx context.Context, api fxiface.CoreAPI, ds drive.DriveStore, path string, userDID string, s network.Stream) error {

	ud, err := ds.ResolveCreate(userDID)
	if err != nil {
		log.Error(err)
		return err
	}
	ud.Publish(ctx, api)

	ps, err := ud.PublicSpace(ctx, api)
	if err != nil {
		log.Error(err)
		return err
	}

	file, err := ps.ReadFile(path, drive.ReadFileOpts{})
	if err != nil {
		log.Error(err)
		return err
	}

	go func() {
		defer s.CloseWrite()

		n, err := io.Copy(s, file)
		if err != nil {
			log.Error("Error in writing file on stream", err)
		}

		log.Infof("wrote %d bytes to the stream", n)
	}()

	return nil
}
