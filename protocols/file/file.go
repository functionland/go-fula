package file

import (
	"fmt"
	"io"
	"io/ioutil"

	proto "github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
)

const Protocol = "fx/file/1"

func RegisterFileProtocol(node host.Host) {
	node.SetStreamHandler(Protocol, protocolHandler)
}

func protocolHandler(s network.Stream) {
	fmt.Println("we are at the protocol")
}

func ReceiveFile(stream network.Stream, cid string) (io.Reader, error) {
	reqMsg := &Request{Type: &Request_Receive{Receive: &Chunk{Id: cid}}}
	header, err := proto.Marshal(reqMsg)
	if err != nil {
		return nil, err
	}
	_, err = stream.Write(header)
	if err != nil {
		return nil, err
	}
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
	for res:= range fileCh {
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
