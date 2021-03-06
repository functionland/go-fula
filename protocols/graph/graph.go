package graph

import (
	"fmt"
	"io/ioutil"
	proto "github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

const Protocol = "fx/graph/1"

func RegisterGraphProtocol(node host.Host) {
	node.SetStreamHandler(Protocol, protocolHandler)
}

func protocolHandler(s network.Stream) {
	fmt.Println("Empty handler for Graph, not Implemented")
}

func GraphQL(query string, values *structpb.Value, stream network.Stream) ([]byte, error) {

	reqMsg := &Request{Query: query, Subscribe: false, OperationName: "", VariableValues: values}
	
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
	
	return buf, nil
}