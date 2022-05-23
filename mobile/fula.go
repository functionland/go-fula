package mobile

import (
	"context"
	"encoding/json"
	"log"
	"os"

	filePL "github.com/functionland/go-fula/protocols/file"
	graphPL "github.com/functionland/go-fula/protocols/graph"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	mplex "github.com/libp2p/go-libp2p-mplex"
	noise "github.com/libp2p/go-libp2p-noise"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

const STORE_PATH = "storePath"

type IFula interface {
	AddBox()
	Send()
	ReceiveFileInfo()
	ReceiveFile()
}

type Fula struct {
	node  host.Host
	peers []peer.ID
	ctx context.Context
}

func NewFula() (*Fula, error) {
	node, err := create()
	ctx := context.Background()
	if err != nil {
		return nil, err
	}
	f := &Fula{node: node, ctx: ctx}
	return f, nil
}

func (f *Fula) AddBox(boxAddr string)  error {
	peerAddr, err := peer.AddrInfoFromString(boxAddr)
	node := f.node
	if err != nil {
		return err
	}
	f.peers = append(f.peers, peerAddr.ID)
	node.Peerstore().AddAddrs(peerAddr.ID, peerAddr.Addrs, peerstore.PermanentAddrTTL)
	return nil
}

func (f *Fula) Send(filePath string) (string,error){
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	stream, err := f.node.NewStream(f.ctx, f.peers[0], filePL.Protocol)
	defer stream.Close()
	if err != nil {
		return "", err
	}

	res,err := filePL.SendFile(file, stream)

	return *res, nil
}

func (f *Fula) ReceiveFileInfo(fileId string) ([]byte, error) {
	stream, err := f.node.NewStream(f.ctx, f.peers[0], filePL.Protocol)
	defer stream.Close()
	if err != nil {
		return nil, err
	}
	meta,err := filePL.ReceiveMeta(stream, fileId)
	
	return meta, nil
}

func (f *Fula) ReceiveFile(fileId string, filePath string) (error){
	stream, err := f.node.NewStream(f.ctx, f.peers[0], filePL.Protocol)
	if err != nil {
		return err
	}
	defer stream.Close()
	fBytes,err := filePL.ReceiveFile(stream, fileId)
	if err != nil {
		return err
	}
	err = os.WriteFile(filePath, *fBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (f *Fula) GraphQL(query string, values string) ([]byte, error){
	stream, err := f.node.NewStream(f.ctx, f.peers[0], graphPL.Protocol)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	val, err := structpb.NewValue(map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(values), &val)
	res, err := graphPL.GraphQL(query, val, stream)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func create() (host.Host, error) {
	// Now, normally you do not just want a simple host, you want
	// that is fully configured to best support your p2p application.
	// Let's create a second host setting some more options.

	// Set your own keypair
	priv, _, err := crypto.GenerateKeyPair(
		crypto.Ed25519, // Select your key type. Ed25519 are nice short
		-1,             // Select key length when possible (i.e. RSA).
	)
	if err != nil {
		panic(err)
	}
	con, err := connmgr.NewConnManager(1, 1)
	if err != nil {
		panic(err)
	}
	node, err := libp2p.New(
		// Use the keypair we generated
		libp2p.Identity(priv),
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/9000",      // regular tcp connections
			"/ip4/0.0.0.0/udp/9000/quic", // a UDP endpoint for the QUIC transport
		),
		// support noise connections
		libp2p.Security(noise.ID, noise.New),
		// support any other default transports (TCP)
		libp2p.DefaultTransports,
		// Let's prevent our peer from having too many
		// connections by attaching a connection manager.
		libp2p.ConnectionManager(con),
		libp2p.Muxer("/mplex/6.7.0", mplex.DefaultTransport),
		// Let this host use relays and advertise itself on relays if
		// it finds it is behind NAT. Use libp2p.Relay(options...) to
		// enable active relays and more.
		// libp2p.EnableAutoRelay(),
		libp2p.DisableRelay(),
		// If you want to help other peers to figure out if they are behind
		// NATs, you can launch the server-side of AutoNAT too (AutoRelay
		// already runs the client)
		//
		// This service is highly rate-limited and should not cause any
		// performance issues.
		libp2p.EnableNATService(),
	)

	log.Printf("Hello World, my second hosts ID is %s\n", node.ID())
	filePL.RegisterFileProtocol(node)
	return node, err
}
