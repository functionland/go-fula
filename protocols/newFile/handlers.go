package newFile

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/functionland/go-fula/drive"
	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	files "github.com/ipfs/go-ipfs-files"

	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/network"
)

// Root handler for newFile protocol
// It reads 2 bytes from the stream which determines the size of the request messeage
// It then reads the request message and passes the stream to the appropriate handler
func Handle(ctx context.Context, api fxiface.CoreAPI, ds drive.DriveStore, s network.Stream) error {
	fmt.Println("handling")

	sizebuf := make([]byte, 2)
	_, err := io.ReadFull(s, sizebuf)
	if err != nil {
		return err
	}

	reqbuf := make([]byte, binary.LittleEndian.Uint16(sizebuf))
	_, err = io.ReadAtLeast(s, reqbuf, int(binary.LittleEndian.Uint16(sizebuf)))
	if err != nil {
		return err
	}

	req := &FSRequest{}
	proto.Unmarshal(reqbuf, req)

	switch req.Action {
	case ActionType_READ:
		return HandleRead(ctx, api, ds, req.Path, req.DID, s)
	case ActionType_MKDIR:
		return HandleMkDir(ctx, api, ds, req.Path, req.DID, s)
	case ActionType_WRITE:
		return HandleWrite(ctx, api, ds, req.Path, req.DID, s)
	case ActionType_DELETE:
		return HandleDelete(ctx, api, ds, req.Path, req.DID, s)
	case ActionType_LS:
		return HandleLs(ctx, api, ds, req.Path, req.DID, s)
	default:
		s.Reset()
		return fmt.Errorf("Action Type not supported: %s", req.Action)
	}
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

// Create a new directory at a given location in the user's drive
func HandleMkDir(ctx context.Context, api fxiface.CoreAPI, ds drive.DriveStore, path string, userDID string, s network.Stream) error {
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

	dcid, err := ps.MkDir(path, drive.MkDirOpts{})
	if err != nil {
		log.Error(err)
		return err
	}

	go func() {
		defer s.CloseWrite()

		if _, err := s.Write([]byte(dcid)); err != nil {
			log.Error("error in writing directory cid on the stream", err)
		}
	}()

	return nil
}

// Handle FSRequest with Delete action, delete a Node at a given location in the user's drive
func HandleDelete(ctx context.Context, api fxiface.CoreAPI, ds drive.DriveStore, path string, userDID string, s network.Stream) error {
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

	_, err = ps.DeleteFile(path, drive.DeleteFileOpts{})
	if err != nil {
		log.Error(err)
		return err
	}

	go func() {
		defer s.CloseWrite()

		if _, err := s.Write([]byte{1}); err != nil {
			log.Error("error in writing delete ack on the stream", err)
		}
	}()

	return nil
}

// Handle FSRequest with Ls action, list entries existing in a given path in a user's drive
func HandleLs(ctx context.Context, api fxiface.CoreAPI, ds drive.DriveStore, path string, userDID string, s network.Stream) error {
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

	de, err := ps.ListEntries(path, drive.ListEntriesOpts{})
	if err != nil {
		log.Error(err)
		return err
	}

	items := make([]*DirEntry, 0)
	for d := range de {
		items = append(items, &DirEntry{Name: d.Name, Type: EntryType(d.Type), Cid: d.Cid.String(), Size: int32(d.Size)})
	}
	dents := &DirEntries{Items: items}

	go func() {
		defer s.CloseWrite()

		dbuf, err := proto.Marshal(dents)
		if err != nil {
			log.Error("error in encoding the DirEntries")
		}
		if _, err := s.Write(dbuf); err != nil {
			log.Error("error in writing directory entries on the stream", err)
		}
	}()

	return nil
}

// Handle FSRequest with Write action, write a file at a given location in the user's drive
func HandleWrite(ctx context.Context, api fxiface.CoreAPI, ds drive.DriveStore, path string, userDID string, s network.Stream) error {
	ud, err := ds.ResolveCreate(userDID)
	if err != nil {
		log.Error(err)
		return err
	}
	err = ud.Publish(context.Background(), api)
	if err != nil {
		log.Error(err)
		return err
	}

	ps, err := ud.PublicSpace(ctx, api)
	if err != nil {
		log.Error(err)
		return err
	}

	pr, pw := io.Pipe()

	f := files.NewReaderFile(pr)

	go func() {
		defer s.CloseRead()
		defer pw.Close()

		io.Copy(pw, s)
	}()

	fcid, err := ps.WriteFile(path, f, drive.WriteFileOpts{})
	if err != nil {
		log.Error("error in receiving file", err)
	}

	go func() {
		defer s.CloseWrite()

		if _, err := s.Write([]byte(fcid)); err != nil {
			log.Error("error in writing the file cid on the stream", err)
		}
	}()

	return nil
}
