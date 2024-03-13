package blockchain

import (
	"sync"
	"time"

	ipfsCluster "github.com/ipfs-cluster/ipfs-cluster/api/rest/client"
	"github.com/ipfs/kubo/client/rpc"
	"github.com/libp2p/go-libp2p/core/peer"
)

type (
	Option  func(*options) error
	options struct {
		authorizer               peer.ID
		authorizedPeers          []peer.ID
		allowTransientConnection bool
		blockchainEndPoint       string
		secretsPath              string
		timeout                  int
		wg                       *sync.WaitGroup
		minPingSuccessCount      int
		maxPingTime              int
		topicName                string
		relays                   []string
		updatePoolName           func(string) error
		fetchFrequency           time.Duration //Hours that it should update the list of pool users and pool requests if not called through pubsub
		rpc                      *rpc.HttpApi
		ipfsClusterApi           ipfsCluster.Client
	}
)

func defaultUpdatePoolName(newPoolName string) error {
	return nil
}
func newOptions(o ...Option) (*options, error) {
	opts := options{
		authorizer:               "",                    // replace with an appropriate default peer.ID
		authorizedPeers:          []peer.ID{},           // default to an empty slice
		allowTransientConnection: true,                  // or false, as per your default
		blockchainEndPoint:       "127.0.0.1:4000",      // default endpoint
		secretsPath:              "",                    //path to secrets dir
		timeout:                  30,                    // default timeout in seconds
		wg:                       nil,                   // initialized WaitGroup
		minPingSuccessCount:      7,                     // default minimum success count
		maxPingTime:              900,                   // default maximum ping time in miliseconds
		topicName:                "0",                   // default topic name
		relays:                   []string{},            // default to an empty slice
		updatePoolName:           defaultUpdatePoolName, // set a default function or leave nil
		fetchFrequency:           time.Hour * 1,         // default frequency, e.g., 1 hour
		rpc:                      nil,
		ipfsClusterApi:           nil,
	}
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

func WithSecretsPath(b string) Option {
	return func(o *options) error {
		o.secretsPath = b
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

func WithMinSuccessPingCount(sr int) Option {
	return func(o *options) error {
		o.minPingSuccessCount = sr
		return nil
	}
}

func WithIpfsClusterAPI(n ipfsCluster.Client) Option {
	return func(o *options) error {
		o.ipfsClusterApi = n
		return nil
	}
}

func WithMaxPingTime(t int) Option {
	return func(o *options) error {
		o.maxPingTime = t
		return nil
	}
}

func WithIpfsClient(n *rpc.HttpApi) Option {
	return func(o *options) error {
		o.rpc = n
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
