package api

import (
	"context"
	"errors"
	"fmt"

	fxfsiface "github.com/functionland/go-fula/fxfs/core/iface"
	bserv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-fetcher"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	exchange "github.com/ipfs/go-ipfs-exchange-interface"
	offlinexch "github.com/ipfs/go-ipfs-exchange-offline"
	pin "github.com/ipfs/go-ipfs-pinner"
	provider "github.com/ipfs/go-ipfs-provider"
	offlineroute "github.com/ipfs/go-ipfs-routing/offline"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	ipfsiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	p2phost "github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	routing "github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	record "github.com/libp2p/go-libp2p-record"
	madns "github.com/multiformats/go-multiaddr-dns"

	"github.com/ipfs/go-namesys"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node"
	"github.com/ipfs/kubo/repo"
)

type CoreAPI struct {
	nctx context.Context

	identity   peer.ID
	privateKey ci.PrivKey

	repo       repo.Repo
	blockstore blockstore.GCBlockstore
	baseBlocks blockstore.Blockstore
	pinning    pin.Pinner

	blocks               bserv.BlockService
	dag                  ipld.DAGService
	ipldFetcherFactory   fetcher.Factory
	unixFSFetcherFactory fetcher.Factory
	peerstore            pstore.Peerstore
	peerHost             p2phost.Host
	recordValidator      record.Validator
	exchange             exchange.Interface

	namesys     namesys.NameSystem
	routing     routing.Routing
	dnsResolver *madns.Resolver

	provider provider.System

	pubSub *pubsub.PubSub

	checkPublishAllowed func() error
	checkOnline         func(allowOffline bool) error

	// ONLY for re-applying options in WithOptions, DO NOT USE ANYWHERE ELSE
	nd         *core.IpfsNode
	parentOpts options.ApiSettings

	// For instancing An IPFS Core API
	nc ipfsiface.CoreAPI
}

// NewCoreAPI creates new instance of IPFS CoreAPI backed by go-ipfs Node.
func NewCoreAPI(n *core.IpfsNode, c ipfsiface.CoreAPI, opts ...options.ApiOption) (fxfsiface.CoreAPI, error) {
	parentOpts, err := options.ApiOptions()
	if err != nil {
		return nil, err
	}
	return (&CoreAPI{nd: n, parentOpts: *parentOpts, nc: c}).WithIPFSOptions(opts...)
}

// PrivateFS unixfs returns the UnixfsAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) PrivateFS() fxfsiface.PrivateFS {
	return (*PrivateAPI)(api)
}

// PublicFS block returns the BlockAPI interface implementation backed by the go-ipfs node
func (api *CoreAPI) PublicFS() fxfsiface.PublicFS {
	return (*PublicAPI)(api)
}

// WithIPFSOptions withOptions returns fxCoreAPI api with global options applied
func (api *CoreAPI) WithIPFSOptions(opts ...options.ApiOption) (fxfsiface.CoreAPI, error) {
	settings := api.parentOpts // make sure to copy
	_, err := options.ApiOptionsTo(&settings, opts...)
	if err != nil {
		return nil, err
	}

	if api.nd == nil {
		return nil, errors.New("cannot apply options to api without node")
	}

	n := api.nd

	subApi := &CoreAPI{
		nctx: n.Context(),

		identity:   n.Identity,
		privateKey: n.PrivateKey,

		repo:       n.Repo,
		blockstore: n.Blockstore,
		baseBlocks: n.BaseBlocks,
		pinning:    n.Pinning,

		blocks:               n.Blocks,
		dag:                  n.DAG,
		ipldFetcherFactory:   n.IPLDFetcherFactory,
		unixFSFetcherFactory: n.UnixFSFetcherFactory,

		peerstore:       n.Peerstore,
		peerHost:        n.PeerHost,
		namesys:         n.Namesys,
		recordValidator: n.RecordValidator,
		exchange:        n.Exchange,
		routing:         n.Routing,
		dnsResolver:     n.DNSResolver,

		provider: n.Provider,

		pubSub: n.PubSub,

		nd:         n,
		parentOpts: settings,
		nc:         api.nc,
	}

	subApi.checkOnline = func(allowOffline bool) error {
		if !n.IsOnline && !allowOffline {
			return ipfsiface.ErrOffline
		}
		return nil
	}

	subApi.checkPublishAllowed = func() error {
		if n.Mounts.Ipns != nil && n.Mounts.Ipns.IsActive() {
			return errors.New("cannot manually publish while IPNS is mounted")
		}
		return nil
	}

	if settings.Offline {
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		cs := cfg.Ipns.ResolveCacheSize
		if cs == 0 {
			cs = node.DefaultIpnsCacheSize
		}
		if cs < 0 {
			return nil, errors.New("cannot specify negative resolve cache size")
		}

		subApi.routing = offlineroute.NewOfflineRouter(subApi.repo.Datastore(), subApi.recordValidator)

		subApi.namesys, err = namesys.NewNameSystem(subApi.routing,
			namesys.WithDatastore(subApi.repo.Datastore()),
			namesys.WithDNSResolver(subApi.dnsResolver),
			namesys.WithCache(cs))
		if err != nil {
			return nil, fmt.Errorf("error constructing namesys: %v", err)
		}

		subApi.provider = provider.NewOfflineProvider()

		subApi.peerstore = nil
		subApi.peerHost = nil
		subApi.recordValidator = nil
	}

	if settings.Offline || !settings.FetchBlocks {
		subApi.exchange = offlinexch.Exchange(subApi.blockstore)
		subApi.blocks = bserv.New(subApi.blockstore, subApi.exchange)
		subApi.dag = dag.NewDAGService(subApi.blocks)
	}

	return subApi, nil
}

// getSession returns new api backed by the same node with a read-only session DAG
func (api *CoreAPI) getSession(ctx context.Context) *CoreAPI {
	sesApi := *api

	// TODO: We could also apply this to api.blocks, and compose into writable api,
	// but this requires some changes in blockservice/merkledag
	sesApi.dag = dag.NewReadOnlyDagService(dag.NewSession(ctx, api.dag))

	return &sesApi
}
