package fulaMobile

import (
	"context"
	"errors"

	"io"

	"os"

	fileP "github.com/functionland/go-fula/protocols/file"
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
	AddBox(string) error
	Read(string, string, string) error
	Write(string, string, string) error
	MkDir(string, string) error
	Ls(string, string) ([]DirEntry, error)
	Delete(string, string) error
}

type EntryType int

const (
	File      EntryType = 1
	Directory EntryType = 2
)

type DirEntry struct {
	Name string
	Type EntryType
	Size int
	Cid  string
}

type Fula struct {
	Node  rhost.RoutedHost
	ctx   context.Context
	mybox []peer.AddrInfo
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
	f := &Fula{Node: *node, ctx: ctx}
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
	err = f.Node.Connect(f.ctx, *peerAddr)
	if err != nil {
		log.Error("peer router failed ", err)
		return err
	}
	f.mybox = append(f.mybox, *peerAddr)
	return nil
}

func (f *Fula) NewStream() (network.Stream, error) {

	peer, err := f.GetBox()
	if err != nil {
		return nil, err
	}
	log.Debug("box found", peer)
	ctx := context.Background()
	ctx = network.WithForceDirectDial(ctx, "transiant wont do it")
	s, err := f.Node.NewStream(ctx, peer, fileP.ProtocolId)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (f *Fula) GetBox() (peer.ID, error) {
	if len(f.mybox) > 0 {
		return f.mybox[0].ID, nil
	}
	return "", errors.New("no box available")
}

// Reads a file from a user's drive
// Calls a ReadRequest on the file protocol
// It writes the recieved file into a specified path on local storage
func (f *Fula) Read(userDID string, filePath string, destPath string) error {
	s, err := f.NewStream()
	if err != nil {
		log.Error("Could not create a file protocol stream", err)
		return err
	}

	ctx := context.Background()
	freader, err := fileP.RequestRead(ctx, s, filePath, userDID)
	if err != nil {
		log.Error("RequestRead failed on the stream", err)
		return err
	}

	if _, err := os.Stat(destPath); err == nil {
		log.Error("Destination file already exists")
		return os.ErrExist
	}

	fw, err := os.Create(destPath)
	if err != nil {
		log.Error("Could not create the destination file")
		return err
	}
	defer fw.Close()

	_, err = io.Copy(fw, freader)
	if err != nil {
		log.Error("Could not write on the destination file")
		return err
	}

	return nil
}

// Writes a file on a user's drive
// Calls a WriteRequest on the file protocol
// It writes an existing file on the local storage in a user's drive in a given location
func (f *Fula) Write(userDID string, srcPath string, destPath string) error {
	s, err := f.NewStream()
	if err != nil {
		log.Error("Could not create a file protocol stream", err)
		return err
	}

	ctx := context.Background()
	sf, err := os.Open(srcPath)
	if err != nil {
		log.Error("Could not open source file")
		return err
	}

	cid, err := fileP.RequestWrite(ctx, s, destPath, userDID, sf)
	if err != nil {
		log.Error("RequestWrite failed on the stream", err)
		return err
	}
	log.Info("Wrote file, CID: ", cid)

	return nil
}

// Makes a new directory on user's drive
// Calls a RequsetMkDir on the file protocol
func (f *Fula) MkDir(userDID string, path string) error {
	s, err := f.NewStream()
	if err != nil {
		log.Error("Could not create a file protocol stream", err)
		return err
	}

	ctx := context.Background()
	_, err = fileP.RequestMkDir(ctx, s, path, userDID)
	if err != nil {
		log.Error("RequestMkDir failed on the stream")
		return err
	}

	return nil
}

// Lists entries in a path on a user's drive
// Calls a RequestLs on the file protocol
func (f *Fula) Ls(userDID string, path string) ([]DirEntry, error) {
	s, err := f.NewStream()
	if err != nil {
		log.Error("Could not create a file protocol stream", err)
		return nil, err
	}

	ctx := context.Background()
	ds, err := fileP.RequestLs(ctx, s, path, userDID)
	if err != nil {
		log.Error("RequestLs failed on the stream", err)
		return nil, err
	}

	dentires := make([]DirEntry, 0)
	for _, de := range ds.Items {
		dentires = append(dentires, DirEntry{Name: de.Name, Size: int(de.Size), Type: EntryType(de.Type), Cid: de.Cid})
	}

	return dentires, nil
}

// Deletes an entry in a path on a user's drive
// Calls a RequestDelete on the file protocol
func (f *Fula) Delete(userDID string, path string) error {
	s, err := f.NewStream()
	if err != nil {
		log.Error("Could not create a file protocol stream", err)
		return err
	}

	ctx := context.Background()
	err = fileP.RequestDelete(ctx, s, path, userDID)
	if err != nil {
		log.Error("RequestDelete failed on the stream", err)
		return err
	}

	return nil
}
