package file

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/gabriel-vasile/mimetype"
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

func ReceiveFile(stream network.Stream, cid string) (*[]byte, error) {
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
	buf, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	return &buf, nil
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

func SendFile(file *os.File, stream network.Stream) (*string, error) {
	fileInfo, _ := os.Lstat(file.Name())

	mtype, err := mimetype.DetectFile(file.Name())
	if err != nil {
		return nil, err
	}

	reqMsg := &Request{
		Type: &Request_Send{
			Send: &Meta{
				Name:         fileInfo.Name(),
				Size_:        uint64(fileInfo.Size()),
				LastModified: fileInfo.ModTime().Unix(),
				Type:         mtype.String()}}}

	header, err := proto.Marshal(reqMsg)
	if err != nil {
		return nil, err
	}
	_, err = stream.Write(header)
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, 1024*10)
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if n > 0 {
			stream.Write(buffer[:n])
		}

	}
	stream.CloseWrite()
	buf2, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}
	id := string(buf2)
	return &id, nil
}
