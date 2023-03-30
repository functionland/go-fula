package exchange

import (
	"time"

	"github.com/ipni/index-provider/engine"
	"github.com/libp2p/go-libp2p/core/peer"
)

type (
	Option  func(*options) error
	options struct {
		authorizer               peer.ID
		allowTransientConnection bool
		ipniPublishDisabled      bool
		ipniPublishTicker        *time.Ticker
		ipniPublishChanBuffer    int
		ipniPublishMaxBatchSize  int
		ipniProviderEngineOpts   []engine.Option
	}
)

func newOptions(o ...Option) (*options, error) {
	opts := options{
		ipniPublishMaxBatchSize: 16 << 10,
		ipniPublishChanBuffer:   1,
	}
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	if opts.ipniPublishTicker == nil {
		opts.ipniPublishTicker = time.NewTicker(10 * time.Second)
	}
	return &opts, nil
}

// WithAuthorizer sets the peer ID that has permission to configure DAG exchange authorization.
// Defaults to authorization disabled.
func WithAuthorizer(a peer.ID) Option {
	return func(o *options) error {
		o.authorizer = a
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
