package blockchain

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

type (
	Option  func(*options) error
	options struct {
		authorizer               peer.ID
		authorizedPeers          []peer.ID
		allowTransientConnection bool
		blockchainEndPoint       string
		timeout                  int
		wg                       *sync.WaitGroup
		minPingSuccessRate       int
		maxPingTime              int
	}
)

func newOptions(o ...Option) (*options, error) {
	var opts options
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	return &opts, nil
}

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

func WithBlockchainEndPoint(b string) Option {
	return func(o *options) error {
		o.blockchainEndPoint = b
		return nil
	}
}

func WithTimeout(to int) Option {
	return func(o *options) error {
		o.timeout = to
		return nil
	}
}

func WithWg(wg *sync.WaitGroup) Option {
	return func(o *options) error {
		o.wg = wg
		return nil
	}
}

func WithSuccessPingRate(sr int) Option {
	return func(o *options) error {
		o.minPingSuccessRate = sr
		return nil
	}
}

func WithMaxPingRate(t int) Option {
	return func(o *options) error {
		o.maxPingTime = t
		return nil
	}
}
