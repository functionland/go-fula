package blox

import (
	"bytes"
	"errors"
	"io"
	"path"
	"time"

	"github.com/functionland/go-fula/exchange"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type (
	Option          func(*options) error
	PoolNameUpdater func(string) error
	options         struct {
		h                  host.Host
		name               string
		topicName          string
		storeDir           string
		announceInterval   time.Duration
		ds                 datastore.Batching
		ls                 *ipld.LinkSystem
		authorizer         peer.ID
		authorizedPeers    []peer.ID
		exchangeOpts       []exchange.Option
		relays             []string
		updatePoolName     PoolNameUpdater
		pingCount          int
		blockchainEndpoint string
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

func WithPingCount(pc int) Option {
	return func(o *options) error {
		o.pingCount = pc
		return nil
	}
}

func WithBlockchainEndPoint(b string) Option {
	return func(o *options) error {
		if b == "" {
			b = "127.0.0.1:4000"
		}
		o.blockchainEndpoint = b
		return nil
	}
}
