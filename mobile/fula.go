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
	"github.com/libp2p/go-libp2p-core/peer"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
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
	node rhost.RoutedHost
	ctx  context.Context
}

func NewFula(repoPath string) (*Fula, error) {
	err := logging.SetLogLevelRegex(".*", "DEBUG")
	if err != nil {
		panic("logger failed")
	}
	ctx := context.Background()
	node, err := create(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	f := &Fula{node: *node, ctx: ctx}
	return f, nil
}

func (f *Fula) AddBox(boxAddr string) error {
	peerAddr, err := peer.AddrInfoFromString(boxAddr)
	if err != nil {
		return err
	}

	hasAdder := len(peerAddr.Addrs) != 0
	if hasAdder {
		var check bool
		check = manet.IsIPLoopback(peerAddr.Addrs[0])
		if check {
			return errors.New("cant Use loop back")
		}
		check = manet.IsIPUnspecified(peerAddr.Addrs[0])
		if check {
			return errors.New("not Specified")
		}
	}

	ps := f.node.Peerstore()
	err = f.node.Connect(f.ctx, *peerAddr)
	if err != nil {
		log.Error("peer router failed ", err)
		return err
	}
	// ps.AddAddrs(peerAddr.ID, peerAddr.Addrs, peerstore.PermanentAddrTTL)
	ps.AddProtocols(peerAddr.ID, filePL.PROTOCOL, graphPL.Protocol)
	return nil
}

func (f *Fula) getBox(protocol string) (peer.ID, error) {
	ps := f.node.Peerstore()
	allPeers := ps.PeersWithAddrs()
	boxPeers := peer.IDSlice{}
	for _, id := range allPeers {
		supported, err := ps.SupportsProtocols(id, protocol)
		if err == nil && len(supported) > 0 {
			boxPeers = append(boxPeers, id)
		}

	}
	log.Debug("box peers", boxPeers.String())
	if len(boxPeers) > 0 {
		return boxPeers[0], nil
	}
	return "", errors.New("no box available")
}

func (f *Fula) Send(filePath string) (string, error) {
	peer, err := f.getBox(filePL.PROTOCOL)
	log.Debug("box found", peer)
	if err != nil {
		return "", err
	}

	meta, err := filePL.FromFile(filePath)
	if err != nil {
		return "", err
	}

	stream, err := f.node.NewStream(f.ctx, peer, filePL.PROTOCOL)
	if err != nil {
		return "", err
	}
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
	peer, err := f.getBox(filePL.PROTOCOL)
	if err != nil {
		log.Error("cant get box", err)
		return nil, err
	}
	stream, err := f.node.NewStream(f.ctx, peer, filePL.PROTOCOL)
	if err != nil {
		log.Error("failed to open new sream", err)
		return nil, err
	}
	meta, err := filePL.ReceiveMeta(stream, fileId)
	if err != nil {
		log.Error("protocol failed to receive meta", err)
		return nil, err
	}
	return meta, nil
}

func (f *Fula) ReceiveFile(fileId string, filePath string) error {
	peer, err := f.getBox(filePL.PROTOCOL)
	if err != nil {
		return err
	}
	stream, err := f.node.NewStream(f.ctx, peer, filePL.PROTOCOL)
	if err != nil {
		return err
	}
	fReader, err := filePL.ReceiveFile(stream, fileId)
	if err != nil {
		return err
	}
	fBytes, err := ioutil.ReadAll(fReader)
	if err != nil {
		return err
	}
	stream.Close()
	err = os.WriteFile(filePath, fBytes, 0755)
	if err != nil {
		return err
	}
	return nil
}

func (f *Fula) GraphQL(query string, values string) ([]byte, error) {
	peer, err := f.getBox(filePL.PROTOCOL)
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
