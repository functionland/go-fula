package fulaMobile

import (
	"bytes"
	"context"
	"errors"

	"io"

	"os"

	fileP "github.com/functionland/go-fula/protocols/file"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-graphsync"
	gs "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/core/host"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multicodec"
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
	h     host.Host
	ctx   context.Context
	mybox []peer.AddrInfo
	ls    ipld.LinkSystem
	gx    graphsync.GraphExchange
	ds    datastore.Batching
}

func NewFula(repoPath string) (*Fula, error) {
	// err := logging.SetLogLevelRegex(".*", "DEBUG")

	// if err != nil {
	// 	// TODO: don't panic!
	// 	panic("logger failed")
	// }
	ctx := context.Background()
	node, host, err := create(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	f := &Fula{Node: *node, h: host, ctx: ctx}

	f.ds = dssync.MutexWrap(datastore.NewMapDatastore())

	f.ls = cidlink.DefaultLinkSystem()
	f.ls.StorageReadOpener = f.blockReadOpener
	f.ls.StorageWriteOpener = f.blockWriteOpener

	f.startGraphExchange(ctx)

	return f, nil
}

func (f *Fula) startGraphExchange(ctx context.Context) {

	gsn := gsnet.NewFromLibp2pHost(f.h)
	f.gx = gs.New(ctx, gsn, f.ls)
	f.gx.RegisterIncomingRequestHook(
		func(p peer.ID, r graphsync.RequestData, ha graphsync.IncomingRequestHookActions) {
			// TODO check the peer is allowed and request is valid
			ha.ValidateRequest()
		})
}

func (f *Fula) blockWriteOpener(ctx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
	buf := bytes.NewBuffer(nil)
	return buf, func(l ipld.Link) error {
		return f.ds.Put(ctx.Ctx, toDatastoreKey(l), buf.Bytes())
	}, nil
}

func (f *Fula) blockReadOpener(ctx ipld.LinkContext, l ipld.Link) (io.Reader, error) {
	val, err := f.ds.Get(ctx.Ctx, toDatastoreKey(l))
	switch err {
	case nil:
		return bytes.NewBuffer(val), nil
	case datastore.ErrNotFound:
		return nil, format.ErrNotFound{Cid: l.(cidlink.Link).Cid}
	default:
		return nil, err
	}
}

var lp = cidlink.LinkPrototype{
	Prefix: cid.Prefix{
		Version:  1,
		Codec:    uint64(multicodec.DagCbor),
		MhType:   uint64(multicodec.Sha2_256),
		MhLength: -1,
	},
}

func (f *Fula) Store(ctx context.Context, n ipld.Node) (ipld.Link, error) {
	return f.ls.Store(ipld.LinkContext{Ctx: ctx}, lp, n)
}

func (f *Fula) Load(ctx context.Context, l ipld.Link, np ipld.NodePrototype) (ipld.Node, error) {
	return f.ls.Load(ipld.LinkContext{Ctx: ctx}, l, np)
}

func (f *Fula) Has(ctx context.Context, l ipld.Link) (bool, error) {
	return f.ds.Has(ctx, toDatastoreKey(l))
}

func toDatastoreKey(l ipld.Link) datastore.Key {
	return datastore.NewKey(l.Binary())
}

func (f *Fula) Fetch(ctx context.Context, from peer.ID, l ipld.Link) error {
	if exists, err := f.Has(ctx, l); err != nil {
		return err
	} else if exists {
		return nil
	}
	resps, errs := f.gx.Request(ctx, from, l, selectorparse.CommonSelector_ExploreAllRecursively)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case resp, ok := <-resps:
			if !ok {
				return nil
			}
			log.Infow("synced node", "node", resp.Node)
		case err, ok := <-errs:
			if !ok {
				return nil
			}
			log.Warnw("sync failed", "err", err)
		}
	}
}

func (f *Fula) AddBox(id peer.ID, addrs []multiaddr.Multiaddr) error {
	// peerAddr, err := peer.AddrInfoFromString(boxAddr)
	// if err != nil {
	// 	return err
	// }

	// hasAdder := len(peerAddr.Addrs) != 0
	// if hasAdder {
	// 	if manet.IsIPLoopback(peerAddr.Addrs[0]) {
	// 		return errors.New("cant Use loop back")
	// 	}
	// 	if manet.IsIPUnspecified(peerAddr.Addrs[0]) {
	// 		return errors.New("not Specified")
	// 	}
	// }

	// if err = f.Node.Connect(f.ctx, *peerAddr); err != nil {
	// 	log.Error("peer router failed ", err.Error())
	// 	return err
	// }

	if err := f.h.Connect(f.ctx, peer.AddrInfo{ID: id, Addrs: addrs}); err != nil {
		log.Error("peer connect failed ", err.Error())
		return err
	}

	f.mybox = append(f.mybox, peer.AddrInfo{ID: id, Addrs: addrs})
	return nil
}

func (f *Fula) NewStream() (network.Stream, error) {

	peerID, err := f.GetBox()
	if err != nil {
		return nil, err
	}
	log.Debug("box found", peerID)

	ctx := network.WithForceDirectDial(context.Background(), "transient wont do it")
	s, err := f.Node.NewStream(ctx, peerID, fileP.ProtocolId)
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
		log.Error("Could not create a file protocol stream", err.Error())
		return err
	}

	freader, err := fileP.RequestRead(context.Background(), s, filePath, userDID)
	if err != nil {
		log.Error("RequestRead failed on the stream", err.Error())
		return err
	}

	if _, err = os.Stat(destPath); err == nil {
		log.Error("Destination file already exists")
		return os.ErrExist
	}

	fw, err := os.Create(destPath)
	if err != nil {
		log.Error("Could not create the destination file")
		return err
	}
	defer fw.Close()

	if _, err = io.Copy(fw, freader); err != nil {
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
		log.Error("Could not create a file protocol stream", err.Error())
		return err
	}

	sf, err := os.Open(srcPath)
	if err != nil {
		log.Error("Could not open source file")
		return err
	}

	cid, err := fileP.RequestWrite(context.Background(), s, destPath, userDID, sf)
	if err != nil {
		log.Error("RequestWrite failed on the stream", err.Error())
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
		log.Error("Could not create a file protocol stream", err.Error())
		return err
	}

	if _, err = fileP.RequestMkDir(context.Background(), s, path, userDID); err != nil {
		log.Error("RequestMkDir failed on the stream", err.Error())
		return err
	}

	return nil
}

// Lists entries in a path on a user's drive
// Calls a RequestLs on the file protocol
func (f *Fula) Ls(userDID string, path string) ([]DirEntry, error) {
	s, err := f.NewStream()
	if err != nil {
		log.Error("Could not create a file protocol stream", err.Error())
		return nil, err
	}

	ds, err := fileP.RequestLs(context.Background(), s, path, userDID)
	if err != nil {
		log.Error("RequestLs failed on the stream", err.Error())
		return nil, err
	}

	entire := make([]DirEntry, 0)
	for _, de := range ds.Items {
		entire = append(entire, DirEntry{Name: de.Name, Size: int(de.Size), Type: EntryType(de.Type), Cid: de.Cid})
	}

	return entire, nil
}

// Deletes an entry in a path on a user's drive
// Calls a RequestDelete on the file protocol
func (f *Fula) Delete(userDID string, path string) error {
	s, err := f.NewStream()
	if err != nil {
		log.Error("Could not create a file protocol stream", err.Error())
		return err
	}

	if err = fileP.RequestDelete(context.Background(), s, path, userDID); err != nil {
		log.Error("RequestDelete failed on the stream", err.Error())
		return err
	}

	return nil
}
