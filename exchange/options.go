package exchange

import (
	"sync"
	"time"

	"github.com/ipfs/kubo/core"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipni/index-provider/engine"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/ratelimit"
)

type (
	Option        func(*options) error
	ConfigUpdater func([]peer.ID) error
	options       struct {
		authorizer               peer.ID
		authorizedPeers          []peer.ID
		allowTransientConnection bool
		ipniPublishDisabled      bool
		ipniPublishTicker        *time.Ticker
		ipniPublishChanBuffer    int
		ipniPublishMaxBatchSize  int
		ipniGetEndpoint          string
		ipniProviderEngineOpts   []engine.Option
		dhtProviderOpts          []dht.Option
		updateConfig             ConfigUpdater
		wg                       *sync.WaitGroup
		ipfsApi                  iface.CoreAPI
		pushRateLimiter          ratelimit.Limiter
		ipfsNode                 *core.IpfsNode
	}
)

func newOptions(o ...Option) (*options, error) {
	opts := options{
		ipniPublishMaxBatchSize: 16 << 10,
		ipniPublishChanBuffer:   1,
		wg:                      nil,
		pushRateLimiter:         ratelimit.New(100), // Default of 50 per second
	}
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	if opts.ipniPublishTicker == nil {
		opts.ipniPublishTicker = time.NewTicker(10 * time.Second)
	}
	if opts.ipniGetEndpoint == "" {
		opts.ipniGetEndpoint = "https://cid.contact/cid/"
	}
	if opts.wg == nil {
		opts.wg = new(sync.WaitGroup)
	}
	return &opts, nil
}

func WithUpdateConfig(updateConfig ConfigUpdater) Option {
	return func(o *options) error {
		o.updateConfig = updateConfig
		return nil
	}
}

// WithAuthorizer sets the peer ID that has permission to configure DAG exchange authorization.
// Defaults to authorization disabled.
func WithAuthorizer(a peer.ID) Option {
	return func(o *options) error {
		o.authorizer = a
		return nil
	}
}

func WithAuthorizedPeers(l []peer.ID) Option {
	return func(o *options) error {
		o.authorizedPeers = l
		return nil
	}
}

func WithAllowTransientConnection(t bool) Option {
	return func(o *options) error {
		o.allowTransientConnection = t
		return nil
	}
}

func WithIpniPublishDisabled(d bool) Option {
	return func(o *options) error {
		o.ipniPublishDisabled = d
		return nil
	}
}

func WithIpniPublishInterval(t time.Duration) Option {
	return func(o *options) error {
		o.ipniPublishTicker = time.NewTicker(t)
		return nil
	}
}

func WithIpniGetEndPoint(l string) Option {
	return func(o *options) error {
		o.ipniGetEndpoint = l
		return nil
	}
}

func WithIpniPublishChanBuffer(s int) Option {
	return func(o *options) error {
		o.ipniPublishChanBuffer = s
		return nil
	}
}

func WithIpniPublishMaxBatchSize(s int) Option {
	return func(o *options) error {
		o.ipniPublishMaxBatchSize = s
		return nil
	}
}

func WithIpniProviderEngineOptions(e ...engine.Option) Option {
	return func(o *options) error {
		o.ipniProviderEngineOpts = e
		return nil
	}
}

func WithDhtProviderOptions(d ...dht.Option) Option {
	return func(o *options) error {
		o.dhtProviderOpts = d
		return nil
	}
}

func WithWg(wg *sync.WaitGroup) Option {
	return func(o *options) error {
		o.wg = wg
		return nil
	}
}

func WithIPFSApi(ipfsApi iface.CoreAPI) Option {
	return func(o *options) error {
		o.ipfsApi = ipfsApi
		return nil
	}
}

func WithIPFSNode(ipfsNode *core.IpfsNode) Option {
	return func(o *options) error {
		o.ipfsNode = ipfsNode
		return nil
	}
}

// WithMaxPushRate sets the maximum number of CIDs pushed per second.
// Defaults to 5 per second.
func WithMaxPushRate(r int) Option {
	return func(o *options) error {
		o.pushRateLimiter = ratelimit.New(r)
		return nil
	}
}
