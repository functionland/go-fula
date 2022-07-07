package mobile

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"sync"

	filePL "github.com/functionland/go-fula/protocols/file"
	graphPL "github.com/functionland/go-fula/protocols/graph"
	logging "github.com/ipfs/go-log"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	mplex "github.com/libp2p/go-libp2p-mplex"
	noise "github.com/libp2p/go-libp2p-noise"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	manet "github.com/multiformats/go-multiaddr/net"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

var log = logging.Logger("fula:mobile")

const STORE_PATH = "storePath"

type IFula interface {
	AddBox()
	Send()
	ReceiveFileInfo()
	ReceiveFile()
	EncryptSend()
	ReceiveDecryptFile()
}

type Fula struct {
	node host.Host
	ctx  context.Context
}

func NewFula() (*Fula, error) {
	err := logging.SetLogLevelRegex("fula:.*", "DEBUG")
	if err != nil {
		panic("logger failed")
	}
	node, err := create()
	ctx := context.Background()
	if err != nil {
		return nil, err
	}
	f := &Fula{node: node, ctx: ctx}
	return f, nil
}

func (f *Fula) AddBox(boxAddr string) error {
	peerAddr, err := peer.AddrInfoFromString(boxAddr)
	if err != nil {
		return err
	}
	var check bool
	check = len(peerAddr.Addrs) == 0
	if check {
		return errors.New("bad Input")
	}
	check = manet.IsIPLoopback(peerAddr.Addrs[0])
	if check {
		return errors.New("cant Use loop back")
	}
	check = manet.IsIPUnspecified(peerAddr.Addrs[0])
	if check {
		return errors.New("not Specified")
	}

	ps := f.node.Peerstore()
	ps.AddAddrs(peerAddr.ID, peerAddr.Addrs, peerstore.PermanentAddrTTL)
	ps.AddProtocols(peerAddr.ID, filePL.Protocol, graphPL.Protocol)
	return nil
}

func (f *Fula) getBox(protocol string) (peer.ID, error) {
	ps := f.node.Peerstore()
	allPeers := ps.PeersWithAddrs()
	log.Debug("peers with address", allPeers.String())
	boxPeers := peer.IDSlice{}
	for _, id := range allPeers {
		supported, err := ps.SupportsProtocols(id, protocol)
		if err == nil && len(supported) > 0 {
			boxPeers = append(boxPeers, id)
		}

	}
	log.Debug("box peers", boxPeers.String())
	for _, id := range boxPeers {
		_, err := f.node.Network().DialPeer(f.ctx, id)
		if err == nil {
			return id, nil
		}

	}
	return "", errors.New("no box available")
}

func (f *Fula) Send(filePath string) (string, error) {
	peer, err := f.getBox(filePL.Protocol)
	log.Debug("box found", peer)
	if err != nil {
		return "", err
	}

	meta, err := filePL.FromFile(filePath)
	if err != nil {
		return "", err
	}

	stream, err := f.node.NewStream(f.ctx, peer, filePL.Protocol)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	wg := sync.WaitGroup{}
	fileCh := make(chan []byte)

	go readFile(filePath, fileCh, &wg)

	res, err := filePL.SendFile(fileCh, meta.ToMetaProto(), stream, &wg)
	if err != nil {
		return "", err
	}
	return *res, nil
}

func readFile(filePath string, fileCh chan []byte, wg *sync.WaitGroup) error {
	defer close(fileCh)
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	// fileInfo, err := os.Lstat(filePath)
	if err != nil {
		return err
	}
	buf := make([]byte, 1024*16)
	buffer := bufio.NewReader(file)
	// fileCh <- buf

	for {

		n, err := buffer.Read(buf)
		if err == io.EOF {
			log.Debug("EOF found")
			break
		}
		if err != nil {
			log.Error("cant read the file:", err)
			return err
		}
		if n > 0 {
			log.Debug("try to write file buffer :", buf[:10])
			wg.Add(1)
			fileCh <- buf[:n]
			wg.Wait()
			log.Debugf("writed file buffer size %d: %v", n, buf[:10])
		}

	}
	return nil
}

func (f *Fula) ReceiveFileInfo(fileId string) ([]byte, error) {
	peer, err := f.getBox(filePL.Protocol)
	if err != nil {
		return nil, err
	}
	stream, err := f.node.NewStream(f.ctx, peer, filePL.Protocol)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	meta, err := filePL.ReceiveMeta(stream, fileId)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

func (f *Fula) ReceiveFile(fileId string, filePath string) error {
	peer, err := f.getBox(filePL.Protocol)
	if err != nil {
		return err
	}
	stream, err := f.node.NewStream(f.ctx, peer, filePL.Protocol)
	if err != nil {
		return err
	}
	defer stream.Close()
	fReader, err := filePL.ReceiveFile(stream, fileId)
	if err != nil {
		return err
	}
	fBytes, err := ioutil.ReadAll(fReader)
	if err != nil {
		return err
	}
	err = os.WriteFile(filePath, fBytes, 0755)
	if err != nil {
		return err
	}
	return nil
}

func (f *Fula) GraphQL(query string, values string) ([]byte, error) {
	peer, err := f.getBox(filePL.Protocol)
	if err != nil {
		return nil, err
	}
	stream, err := f.node.NewStream(f.ctx, peer, graphPL.Protocol)
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

	log.Info("Start With", node.ID())
	return node, err
}
