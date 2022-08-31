package newFile

import (
	"context"
	"io"

	"github.com/golang/protobuf/proto"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
)

var log = logging.Logger("newFilePL")

// Send an FSRequest for Read action, return a io.Reader to read file from the stream
func RequestRead(ctx context.Context, s network.Stream, path string, userDID string) (io.Reader, error) {

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
		defer s.Close()

		if _, err := io.Copy(pw, s); err != nil {
			log.Error("error in reading file from stream", err)
		}
	}()

	return pr, nil
}

// Send an FSRequest for MkDir action, wait until receiving a cid for the directory
func RequestMkDir(ctx context.Context, s network.Stream, path string, userDID string) (string, error) {
	fsReq := &FSRequest{DID: userDID, Path: path, Action: ActionType_MKDIR}

	reqMsg, err := proto.Marshal(fsReq)
	if err != nil {
		return "", err
	}

	n, err := s.Write(reqMsg)
	if err != nil {
		return "", err
	}
	log.Debugf("readFile req sent, wrote %d bytes on stream", n)

	err = s.CloseWrite()
	if err != nil {
		log.Error("can't close write stream. ", err)
		s.Reset()
		return "", err
	}
	log.Debug("stream write closed successfuly")

	bcid, err := io.ReadAll(s)
	if err != nil {
		log.Error("coudn't receive the cid ", err)
		s.Reset()
		return "", err
	}

	return string(bcid), nil
}
