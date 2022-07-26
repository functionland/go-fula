package mobile

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"sync"

	filePL "github.com/functionland/go-fula/protocols/file"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	manet "github.com/multiformats/go-multiaddr/net"
)

var log = logging.Logger("fula:mobile")

const (
	STORE_PATH = "storePath"
	//I will cheet until i can add threadsafe datastore
	MY_BOX = "/mybox/cheat"
)

type IFula interface {
	AddBox()
	Send()
	ReceiveFileInfo()
	ReceiveFile()
	EncryptSend()
	ReceiveDecryptFile()
}

type Fula struct {
	node  rhost.RoutedHost
	ctx   context.Context
	mybox []peer.AddrInfo
}

func NewFula(repoPath string) (*Fula, error) {
	err := logging.SetLogLevelRegex("fula.*", "DEBUG")
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
	err = f.node.Connect(f.ctx, *peerAddr)
	if err != nil {
		log.Error("peer router failed ", err)
		return err
	}
	f.mybox = append(f.mybox, *peerAddr)
	return nil
}

func (f *Fula) NewStream() (network.Stream, error) {

	peer, err := f.getBox()
	if err != nil {
		return nil, err
	}
	log.Debug("box found", peer)
	ctx := context.Background()
	ctx = network.WithForceDirectDial(ctx, "transiant wont do it")
	s, err := f.node.NewStream(ctx, peer, filePL.PROTOCOL)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (f *Fula) getBox() (peer.ID, error) {
	if len(f.mybox) > 0 {
		return f.mybox[0].ID, nil
	}
	return "", errors.New("no box available")
}

func (f *Fula) Send(filePath string) (string, error) {
	stream, err := f.NewStream()
	if err != nil {
		return "", err
	}
	meta, err := filePL.FromFile(filePath)
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
	stream, err := f.NewStream()
	if err != nil {
		return nil, err
	}
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
	stream, err := f.NewStream()
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
