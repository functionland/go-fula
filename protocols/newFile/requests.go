package newFile

import (
	"context"
	"encoding/binary"
	"io"

	"github.com/golang/protobuf/proto"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
)

var log = logging.Logger("newFilePL")

func sendRequest(req *FSRequest, s network.Stream) error {
	reqMsg, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	sbuf := make([]byte, 2)
	binary.LittleEndian.PutUint16(sbuf, uint16(len(reqMsg)))
	n, err := s.Write(sbuf)
	if err != nil {
		return err
	}
	log.Debugf("req size sent on the stream, wrote %d bytes on stream", n)

	n, err = s.Write(reqMsg)
	if err != nil {
		return err
	}
	log.Debugf("req sent, wrote %d bytes on stream", n)

	return nil
}

// Send an FSRequest for Read action, return a io.Reader to read file from the stream
func RequestRead(ctx context.Context, s network.Stream, path string, userDID string) (io.Reader, error) {

	fsReq := &FSRequest{DID: userDID, Path: path, Action: ActionType_READ}

	err := sendRequest(fsReq, s)
	if err != nil {
		return nil, err
	}

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

	err := sendRequest(fsReq, s)
	if err != nil {
		return "", err
	}

	bcid, err := io.ReadAll(s)
	if err != nil {
		log.Error("coudn't receive the cid ", err)
		s.Reset()
		return "", err
	}

	return string(bcid), nil
}

// Send an FSRequest for Write action, wait until receiving a cid for the file
func RequestWrite(ctx context.Context, s network.Stream, path string, userDID string, f io.Reader) (string, error) {
	fsReq := &FSRequest{DID: userDID, Path: path, Action: ActionType_WRITE}

	err := sendRequest(fsReq, s)
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(s, f); err != nil {
		log.Error("error writing the file on the stream: ", err)
		return "", err
	}
	log.Debug("wrote file on the stream successfuly")
	s.CloseWrite()

	bcid, err := io.ReadAll(s)
	if err != nil {
		log.Error("coudn't receive the cid ", err)
		s.Reset()
		return "", err
	}
	s.CloseRead()

	return string(bcid), nil
}

// Send and FSRequest for Delete action, wait until receiving an ack (a one byte)
func RequestDelete(ctx context.Context, s network.Stream, path string, userDID string) error {
	fsReq := &FSRequest{DID: userDID, Path: path, Action: ActionType_DELETE}

	err := sendRequest(fsReq, s)
	if err != nil {
		return err
	}

	_, err = io.ReadAll(s)
	if err != nil {
		log.Error("couldn't receive the cid ", err)
		s.Reset()
		return err
	}

	return nil
}

// Send and FSRequest for Ls action, wait until receiving a list of DirEntries
func RequestLs(ctx context.Context, s network.Stream, path string, userDID string) (*DirEntries, error) {
	fsReq := &FSRequest{DID: userDID, Path: path, Action: ActionType_LS}

	err := sendRequest(fsReq, s)
	if err != nil {
		return nil, err
	}

	dbuf, err := io.ReadAll(s)
	if err != nil {
		log.Error("couldn't receive the cid ", err)
		s.Reset()
		return nil, err
	}

	dentries := &DirEntries{}
	proto.Unmarshal(dbuf, dentries)

	return dentries, nil
}
