package blox

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"path"
	"sync"
	"time"

	ipfsCluster "github.com/ipfs-cluster/ipfs-cluster/api/rest/client"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/kubo/client/rpc"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

type (
	Option          func(*options) error
	PoolNameUpdater func(string) error
	PoolNameGetter  func() string
	options         struct {
		selfPeerID            peer.ID
		name                  string
		topicName             string
		chainName             string
		storeDir              string
		announceInterval      time.Duration
		ds                    datastore.Batching
		ls                    *ipld.LinkSystem
		authorizer            peer.ID
		authorizedPeers       []peer.ID
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
		kuboAPIAddr           string // Address of kubo's HTTP API (default: 127.0.0.1:5001)
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
	if opts.authorizer == "" && opts.selfPeerID != "" {
		opts.authorizer = opts.selfPeerID
	}
	if opts.wg == nil {
		opts.wg = new(sync.WaitGroup)
	}
	if opts.kuboAPIAddr == "" {
		opts.kuboAPIAddr = defaultKuboAPIAddr
	}
	return &opts, nil
}

// WithAuthorizer sets the peer ID that is allowed to manage authorization.
// If unset, defaults to the blox's own peer ID (selfPeerID).
func WithAuthorizer(id peer.ID) Option {
	return func(o *options) error {
		o.authorizer = id
		return nil
	}
}

// WithSelfPeerID sets the local peer ID (derived from the private key).
// This replaces the libp2p host's ID for authorization and pool discovery.
func WithSelfPeerID(id peer.ID) Option {
	return func(o *options) error {
		o.selfPeerID = id
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

// WithStoreDir sets the store directory used for datastore.
func WithStoreDir(n string) Option {
	return func(o *options) error {
		o.storeDir = n
		return nil
	}
}

// WithTopicName sets the name of the topic onto which announcements are made.
// Defaults to "/explore.fula/pools/<pool-name>" if unset.
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

// WithRelays sets the relay addresses.
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

// WithKuboAPIAddr sets the address of kubo's HTTP API.
// Defaults to "127.0.0.1:5001".
func WithKuboAPIAddr(addr string) Option {
	return func(o *options) error {
		o.kuboAPIAddr = addr
		return nil
	}
}
