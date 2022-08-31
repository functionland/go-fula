package newFile

import (
	"context"
	"io"

	"github.com/golang/protobuf/proto"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
)

var log = logging.Logger("newFilePL")

func ReadFile(ctx context.Context, s network.Stream, path string, userDID string) (io.Reader, error) {

	fsReq := &FSRequest{DID: userDID, Path: path, Action: ActionType_READ}

	reqMsg, err := proto.Marshal(fsReq)
	if err != nil {
		return nil, err
	}

	n, err := s.Write(reqMsg)
	if err != nil {
		return nil, err
	}
	log.Debugf("readFile req sent, wrote %d bytes on stream", n)

	err = s.CloseWrite()
	if err != nil {
		log.Error("can't close write stream. ", err)
		s.Reset()
		return nil, err
	}
	log.Debug("stream write closed successfuly")

	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		if _, err := io.Copy(pw, s); err != nil {
			log.Error("error in reading file from stream", err)
		}
	}()

	return pr, nil
	//Read file from the stream
	// buf, err := io.ReadAll(s)
	// if err != nil {
	// 	log.Error("can't read result from stream")
	// 	s.Reset()
	// 	return nil, err
	// }
	// id := string(buf)
	// log.Debugf("received cid: %s", id)
	// err = s.CloseRead()
	// if err != nil {
	// 	log.Error("failed to close read stream. ", err)
	// 	s.Reset()
	// 	return nil, err
	// }
	// s.Close()
	// return id, nil
}
