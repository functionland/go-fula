package blockchain

import (
	"sync"
	"time"

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
		topicName                string
		relays                   []string
		updatePoolName           func(string) error
		fetchFrequency           time.Duration //Hours that it should update the list of pool users and pool requests if not called through pubsub
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
		if b == "" {
			b = "127.0.0.1:4000"
		}
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

func WithFetchFrequency(t time.Duration) Option {
	return func(o *options) error {
		o.fetchFrequency = t
		return nil
	}
}

func WithTopicName(n string) Option {
	return func(o *options) error {
		o.topicName = n
		return nil
	}
}

func WithUpdatePoolName(updatePoolName func(string) error) Option {
	return func(o *options) error {
		o.updatePoolName = updatePoolName
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
