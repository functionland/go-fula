package file

import (
	"io"
	"io/ioutil"

	proto "github.com/golang/protobuf/proto"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
)

const Protocol = "fx/file/1"

var log = logging.Logger("fula:filePL")

func ReceiveFile(stream network.Stream, cid string) (io.Reader, error) {
	reqMsg := &Request{Type: &Request_Receive{Receive: &Chunk{Id: cid}}}
	header, err := proto.Marshal(reqMsg)
	if err != nil {
		log.Error("Can not create Request message")
		return nil, err
	}
	log.Debug("Request Message Created")
	_, err = stream.Write(header)
	if err != nil {
		log.Debug("Sending Request Message Failed")
		return nil, err
	}
	log.Debug("Request Message Sent")
	stream.CloseWrite()
	return stream, nil
}

func ReceiveMeta(stream network.Stream, cid string) ([]byte, error) {
	reqMsg := &Request{
		Type: &Request_Meta{Meta: cid}}

	header, err := proto.Marshal(reqMsg)
	if err != nil {
		return nil, err
	}
	_, err = stream.Write(header)
	if err != nil {
		return nil, err
	}

	stream.CloseWrite()

	buf, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return buf, nil
}

func SendFile(fileCh <-chan []byte, filemeta Meta, stream network.Stream) (*string, error) {

	reqMsg := &Request{Type: &Request_Send{Send: &filemeta}}

	header, err := proto.Marshal(reqMsg)
	if err != nil {
		return nil, err
	}
	_, err = stream.Write(header)
	if err != nil {
		return nil, err
	}
	for res := range fileCh {
		stream.Write(res)
	}
	stream.CloseWrite()
	buf2, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	id := string(buf2)
	return &id, nil
}
