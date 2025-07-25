package blox

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"path"
	"sync"
	"time"

	"github.com/functionland/go-fula/exchange"
	ipfsCluster "github.com/ipfs-cluster/ipfs-cluster/api/rest/client"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/kubo/client/rpc"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type (
	Option          func(*options) error
	PoolNameUpdater func(string) error
	PoolNameGetter  func() string
	options         struct {
		h                     host.Host
		name                  string
		topicName             string
		chainName             string
		storeDir              string
		announceInterval      time.Duration
		ds                    datastore.Batching
		ls                    *ipld.LinkSystem
		authorizer            peer.ID
		authorizedPeers       []peer.ID
		exchangeOpts          []exchange.Option
		relays                []string
		updatePoolName        PoolNameUpdater
		getPoolName           PoolNameGetter
		updateChainName       func(string) error
		getChainName          func() string
		pingCount             int
		maxPingTime           int
		minSuccessRate        int
		blockchainEndpoint    string
		secretsPath           string
		poolHostMode          bool
		IPFShttpServer        *http.Server
		DefaultIPFShttpServer string
		wg                    *sync.WaitGroup
		rpc                   *rpc.HttpApi
		ipfsClusterApi        ipfsCluster.Client
	}
)

func newOptions(o ...Option) (*options, error) {
	opts := options{
		announceInterval: 5 * time.Second,
	}
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	if opts.name == "" {
		return nil, errors.New("blox pool name must be specified")
	}
	if opts.pingCount <= 0 {
		log.Warnf("ping count is not specified, using default of 5 instead of %d", opts.pingCount)
		opts.pingCount = 5
	}
	if opts.topicName == "" {
		opts.topicName = path.Clean(opts.name)
	}
	if opts.h == nil {
		var err error
		if opts.h, err = libp2p.New(); err != nil {
			return nil, err
		}
	}
	if opts.ds == nil {
		opts.ds = dssync.MutexWrap(datastore.NewMapDatastore())
	}
	if opts.ls == nil {
		ls := cidlink.DefaultLinkSystem()
		ls.StorageWriteOpener = func(ctx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
			buf := bytes.NewBuffer(nil)
			return buf, func(l ipld.Link) error {
				key := datastore.NewKey(l.Binary())
				if err := opts.ds.Put(ctx.Ctx, key, buf.Bytes()); err != nil {
					return err
				}
				return opts.ds.Sync(ctx.Ctx, key)
			}, nil
		}
		ls.StorageReadOpener = func(ctx ipld.LinkContext, l ipld.Link) (io.Reader, error) {
			val, err := opts.ds.Get(ctx.Ctx, datastore.NewKey(l.Binary()))
			if err != nil {
				return nil, err
			}
			return bytes.NewBuffer(val), nil
		}
	}
	if opts.authorizer == "" {
		opts.authorizer = opts.h.ID()
	}
	if opts.wg == nil {
		opts.wg = new(sync.WaitGroup)
	}
	return &opts, nil
}

// WithHost sets the libp2p host on which the blox is exposed.
// If unset a default host with random identity is used.
// See: libp2p.New.
func WithHost(h host.Host) Option {
	return func(o *options) error {
		o.h = h
		return nil
	}
}

// WithPoolName sets a human readable name for the pool that the blox should join or create.
// Required.
func WithPoolName(n string) Option {
	return func(o *options) error {
		o.name = n
		return nil
	}
}

// WithStoreDir sets a the store directory we are using for datastore
// Required.
func WithStoreDir(n string) Option {
	return func(o *options) error {
		o.storeDir = n
		return nil
	}
}

// WithTopicName sets the name of the topic onto which announcements are made.
// Defaults to "/explore.fula/pools/<pool-name>" if unset.
// See: WithPoolName.
func WithTopicName(n string) Option {
	return func(o *options) error {
		o.topicName = n
		return nil
	}
}

// WithAnnounceInterval sets the interval at which announcements are made on the pubsub.
// Defaults to 5 seconds if unset.
func WithAnnounceInterval(i time.Duration) Option {
	return func(o *options) error {
		o.announceInterval = i
		return nil
	}
}

func WithDatastore(ds datastore.Batching) Option {
	return func(o *options) error {
		o.ds = ds
		return nil
	}
}

func WithDefaultIPFShttpServer(n string) Option {
	return func(o *options) error {
		o.DefaultIPFShttpServer = n
		return nil
	}
}

func WithIpfsClient(n *rpc.HttpApi) Option {
	return func(o *options) error {
		o.rpc = n
		return nil
	}
}

func WithPoolHostMode(n bool) Option {
	return func(o *options) error {
		o.poolHostMode = n
		return nil
	}
}

func WithIpfsClusterAPI(n ipfsCluster.Client) Option {
	return func(o *options) error {
		o.ipfsClusterApi = n
		return nil
	}
}

func WithLinkSystem(ls *ipld.LinkSystem) Option {
	return func(o *options) error {
		o.ls = ls
		return nil
	}
}

func WithExchangeOpts(eo ...exchange.Option) Option {
	return func(o *options) error {
		o.exchangeOpts = eo
		return nil
	}
}

// WithStoreDir sets a the store directory we are using for datastore
// Required.
func WithRelays(r []string) Option {
	return func(o *options) error {
		o.relays = r
		return nil
	}
}

func WithUpdatePoolName(updatePoolName PoolNameUpdater) Option {
	return func(o *options) error {
		o.updatePoolName = updatePoolName
		return nil
	}
}

func WithGetPoolName(getPoolName PoolNameGetter) Option {
	return func(o *options) error {
		o.getPoolName = getPoolName
		return nil
	}
}

func WithPingCount(pc int) Option {
	return func(o *options) error {
		o.pingCount = pc
		return nil
	}
}

func WithMaxPingTime(pt int) Option {
	return func(o *options) error {
		o.maxPingTime = pt
		return nil
	}
}

func WithMinSuccessPingRate(sr int) Option {
	return func(o *options) error {
		o.minSuccessRate = sr
		return nil
	}
}

func WithBlockchainEndPoint(b string) Option {
	return func(o *options) error {
		if b == "" {
			b = "api.node3.functionyard.fula.network"
		}
		o.blockchainEndpoint = b
		return nil
	}
}

func WithSecretsPath(b string) Option {
	return func(o *options) error {
		o.secretsPath = b
		return nil
	}
}

func WithWg(wg *sync.WaitGroup) Option {
	return func(o *options) error {
		o.wg = wg
		return nil
	}
}

func WithChainName(n string) Option {
	return func(o *options) error {
		o.chainName = n
		return nil
	}
}

func WithUpdateChainName(updateChainName func(string) error) Option {
	return func(o *options) error {
		o.updateChainName = updateChainName
		return nil
	}
}

func WithGetChainName(getChainName func() string) Option {
	return func(o *options) error {
		o.getChainName = getChainName
		return nil
	}
}
