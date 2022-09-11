package api

import (
	"context"
	"errors"
	"fmt"
	"sync"

	pfs "github.com/functionland/go-fula/fxfs/core/pfs"
	"github.com/functionland/go-mfs"
	ft "github.com/functionland/go-unixfs"
	unixfile "github.com/functionland/go-unixfs/file"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cidutil"
	"github.com/ipfs/go-filestore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	files "github.com/ipfs/go-ipfs-files"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	dagtest "github.com/ipfs/go-merkledag/test"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type PrivateAPI CoreAPI

func (api *PrivateAPI) Add(ctx context.Context, files files.Node, opts ...options.UnixfsAddOption) (path.Resolved, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.UnixfsAPI", "Add")
	defer span.End()

	settings, prefix, err := options.UnixfsAddOptions(opts...)
	if err != nil {
		return nil, err
	}

	span.SetAttributes(
		attribute.String("chunker", settings.Chunker),
		attribute.Int("cidversion", settings.CidVersion),
		attribute.Bool("inline", settings.Inline),
		attribute.Int("inlinelimit", settings.InlineLimit),
		attribute.Bool("rawleaves", settings.RawLeaves),
		attribute.Bool("rawleavesset", settings.RawLeavesSet),
		attribute.Int("layout", int(settings.Layout)),
		attribute.Bool("pin", settings.Pin),
		attribute.Bool("onlyhash", settings.OnlyHash),
		attribute.Bool("fscache", settings.FsCache),
		attribute.Bool("nocopy", settings.NoCopy),
		attribute.Bool("silent", settings.Silent),
		attribute.Bool("progress", settings.Progress),
	)

	cfg, err := api.repo.Config()
	if err != nil {
		return nil, err
	}

	// check if repo will exceed storage limit if added
	// TODO: this doesn't handle the case if the hashed file is already in blocks (deduplicated)
	// TODO: conditional GC is disabled due to it is somehow not possible to pass the size to the daemon
	//if err := corerepo.ConditionalGC(req.Context(), n, uint64(size)); err != nil {
	//	res.SetError(err, cmds.ErrNormal)
	//	return
	//}

	if settings.NoCopy && !(cfg.Experimental.FilestoreEnabled || cfg.Experimental.UrlstoreEnabled) {
		return nil, errors.New("either the filestore or the urlstore must be enabled to use nocopy, see: https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-filestore")
	}

	addblockstore := api.blockstore
	if !(settings.FsCache || settings.NoCopy) {
		addblockstore = blockstore.NewGCBlockstore(api.baseBlocks, api.blockstore)
	}
	exch := api.exchange
	pinning := api.pinning

	if settings.OnlyHash {
		node, err := getOrCreateNilNode()
		if err != nil {
			return nil, err
		}
		addblockstore = node.Blockstore
		exch = node.Exchange
		pinning = node.Pinning
	}

	bserv := blockservice.New(addblockstore, exch) // hash security 001
	dserv := dag.NewDAGService(bserv)

	// add a sync call to the DagService
	// this ensures that data written to the DagService is persisted to the underlying datastore
	// TODO: propagate the Sync function from the datastore through the blockstore, blockservice and dagservice
	var syncDserv *syncDagService
	if settings.OnlyHash {
		syncDserv = &syncDagService{
			DAGService: dserv,
			syncFn:     func() error { return nil },
		}
	} else {
		syncDserv = &syncDagService{
			DAGService: dserv,
			syncFn: func() error {
				ds := api.repo.Datastore()
				if err := ds.Sync(ctx, blockstore.BlockPrefix); err != nil {
					return err
				}
				return ds.Sync(ctx, filestore.FilestorePrefix)
			},
		}
	}

	fileAdder, err := pfs.NewAdder(ctx, pinning, addblockstore, syncDserv)
	if err != nil {
		return nil, err
	}

	fileAdder.Chunker = settings.Chunker
	if settings.Events != nil {
		fileAdder.Out = settings.Events
		fileAdder.Progress = settings.Progress
	}
	fileAdder.Pin = settings.Pin && !settings.OnlyHash
	fileAdder.Silent = settings.Silent
	fileAdder.RawLeaves = settings.RawLeaves
	fileAdder.NoCopy = settings.NoCopy
	fileAdder.CidBuilder = prefix

	switch settings.Layout {
	case options.BalancedLayout:
		// Default
	case options.TrickleLayout:
		fileAdder.Trickle = true
	default:
		return nil, fmt.Errorf("unknown layout: %d", settings.Layout)
	}

	if settings.Inline {
		fileAdder.CidBuilder = cidutil.InlineBuilder{
			Builder: fileAdder.CidBuilder,
			Limit:   settings.InlineLimit,
		}
	}

	if settings.OnlyHash {
		md := dagtest.Mock()
		emptyDirNode := ft.EmptyDirNode()
		// Use the same prefix for the "empty" MFS root as for the file adder.
		emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
		mr, err := mfs.NewRoot(ctx, md, emptyDirNode, nil)
		if err != nil {
			return nil, err
		}

		fileAdder.SetMfsRoot(mr)
	}

	nd, err := fileAdder.AddAllAndPin(ctx, files)
	if err != nil {
		return nil, err
	}

	if !settings.OnlyHash {
		if err := api.provider.Provide(nd.Cid()); err != nil {
			return nil, err
		}
	}

	return path.IpfsPath(nd.Cid()), nil
}

func (api *PrivateAPI) Get(ctx context.Context, p path.Path) (files.Node, error) {
	ctx, span := tracing.Span(ctx, "CoreAPI.UnixfsAPI", "Get", trace.WithAttributes(attribute.String("path", p.String())))
	defer span.End()

	nd, err := api.nc.ResolveNode(ctx, p)
	if err != nil {
		return nil, err
	}

	return unixfile.NewUnixfsFile(ctx, api.core().getSession(ctx).dag, nd)
}

func (api *PrivateAPI) Ls(ctx context.Context, p path.Path, opts ...options.UnixfsLsOption) (<-chan iface.DirEntry, error) {
	return api.nc.Unixfs().Ls(ctx, p, opts...)
}

func (api *PrivateAPI) core() *CoreAPI {
	return (*CoreAPI)(api)
}

var nilNode *core.IpfsNode
var once sync.Once

func getOrCreateNilNode() (*core.IpfsNode, error) {
	once.Do(func() {
		if nilNode != nil {
			return
		}
		node, err := core.NewNode(context.Background(), &core.BuildCfg{
			//TODO: need this to be true or all files
			// hashed will be stored in memory!
			NilRepo: true,
		})
		if err != nil {
			panic(err)
		}
		nilNode = node
	})

	return nilNode, nil
}

type syncDagService struct {
	ipld.DAGService
	syncFn func() error
}
