package protocol

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	pb "github.com/farhoud/go-fula/fula/protocols/pb/file"

	"github.com/gabriel-vasile/mimetype"
	proto "github.com/gogo/protobuf/proto"
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

func ReceiveFile(stream network.Stream, cid string) []byte {
	reqMsg := &pb.Request{Type: &pb.Request_Receive{Receive: &pb.Chunk{Id: cid}}}
	header, err := proto.Marshal(reqMsg)
	if err != nil {
		panic(err)
	}
	_, err = stream.Write(header)
	if err != nil {
		panic(err)
	}
	stream.CloseWrite()
	buf, err := ioutil.ReadAll(stream)
	if err != nil {
		panic(err)
	}
	return buf
}

func ReceiveMeta(stream network.Stream, cid string) *pb.Meta {
	reqMsg := &pb.Request{
		Type: &pb.Request_Meta{Meta: cid}}

	header, err := proto.Marshal(reqMsg)
	if err != nil {
		panic(err)
	}
	_, err = stream.Write(header)
	if err != nil {
		panic(err)
	}

	stream.CloseWrite()

	buf, err := ioutil.ReadAll(stream)
	if err != nil {
		panic(err)
	}

	meta := &pb.Meta{}
	err = proto.Unmarshal(buf, meta)
	if err != nil {
		panic(err)
	}

	fmt.Println(meta)
	return meta
}

func SendFile(file *os.File, stream network.Stream) string {
	fileInfo, _ := os.Lstat(file.Name())
	fmt.Println(fileInfo)
	mtype, err := mimetype.DetectFile(file.Name())
	if err != nil {
		panic(err)
	}

	reqMsg := &pb.Request{
		Type: &pb.Request_Send{
			Send: &pb.Meta{
				Name:         fileInfo.Name(),
				Size_:        uint64(fileInfo.Size()),
				LastModified: fileInfo.ModTime().Unix(),
				Type:         mtype.String()}}}

	header, err := proto.Marshal(reqMsg)
	if err != nil {
		panic(err)
	}
	fmt.Println("header unmarshald")
	_, err = stream.Write(header)
	if err != nil {
		panic(err)
	}
	fmt.Println("header sent")
	fmt.Println(fileInfo.Size())
	buffer := make([]byte, 1024*10)
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			fmt.Println(n)
			fmt.Println(buffer)
			fmt.Println("end of file")
			break
		}
		if err != nil {
			fmt.Println(err)
			continue
		}
		if n > 0 {
			fmt.Println("sending chunk")
			stream.Write(buffer[:n])
			fmt.Println("chunk sent")
		}

	}
	fmt.Println("file sended")
	stream.CloseWrite()
	buf2, err := ioutil.ReadAll(stream)
	fmt.Println("read happend sended")
	id := string(buf2)
	fmt.Println(id)
	return id
}
