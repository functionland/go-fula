package blockchain

import (
	"sync"
	"time"

	ipfsCluster "github.com/ipfs-cluster/ipfs-cluster/api/rest/client"
	"github.com/ipfs/kubo/client/rpc"
	"github.com/libp2p/go-libp2p/core/crypto"
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
		chainName                string
		relays                   []string
		updatePoolName           func(string) error
		getPoolName              func() string
		updateChainName          func(string) error
		getChainName             func() string
		fetchFrequency           time.Duration //Hours that it should update the list of pool users and pool requests if not called through pubsub
		rpc                      *rpc.HttpApi
		ipfsClusterApi           ipfsCluster.Client
		selfPeerID               peer.ID                // Peer ID derived from private key, used for authorization checks
		clusterPeerID            peer.ID                // IPFS cluster peer ID (original identity), used for on-chain pool membership
		signingKey               crypto.PrivKey         // Private key for signing outgoing requests (mobile client)
		clientProtocolID         string                 // Protocol ID for kubo p2p forwarding (e.g. "/x/fula-blockchain")
		onPoolConfigCleared      func()                 // Called after a leave / reconcile cleared the pool from the config
		poolHost                 bool                   // Pool host (--poolHost): never leaves / reconciles its pool
		proxyListenAddr          string                 // TCP address of the kubo-forwarded blockchain proxy
		pingListenAddr           string                 // TCP address of the kubo-forwarded ping server
		chainConfigOverride      map[string]ChainConfig // Replaces GetChainConfigs() when set (tests only)
	}
)

func defaultUpdatePoolName(newPoolName string) error {
	return nil
}
func defaultGetPoolName() string {
	return "0"
}
func defaultUpdateChainName(newChainName string) error {
	return nil
}
func defaultGetChainName() string {
	return ""
}
func newOptions(o ...Option) (*options, error) {
	opts := options{
		authorizer:               "",                                    // replace with an appropriate default peer.ID
		authorizedPeers:          []peer.ID{},                           // default to an empty slice
		allowTransientConnection: true,                                  // or false, as per your default
		blockchainEndPoint:       "api.node3.functionyard.fula.network", // default endpoint
		secretsPath:              "",                                    //path to secrets dir
		timeout:                  30,                                    // default timeout in seconds
		wg:                       nil,                                   // initialized WaitGroup
		minPingSuccessCount:      7,                                     // default minimum success count
		maxPingTime:              900,                                   // default maximum ping time in miliseconds
		topicName:                "0",                                   // default topic name
		chainName:                "",                                    // default chain name (empty means auto-detect)
		relays:                   []string{},                            // default to an empty slice
		updatePoolName:           defaultUpdatePoolName,                 // set a default function or leave nil
		getPoolName:              defaultGetPoolName,
		updateChainName:          defaultUpdateChainName, // set a default function or leave nil
		getChainName:             defaultGetChainName,
		fetchFrequency:           time.Hour * 1, // default frequency, e.g., 1 hour
		rpc:                      nil,
		ipfsClusterApi:           nil,
		proxyListenAddr:          ProxyListenAddr,
		pingListenAddr:           PingListenAddr,
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
			b = "api.node3.functionyard.fula.network"
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

// WithUpdatePoolName sets the pool name setter. A nil setter keeps the default (callers such as blox.New pass
// their own option through even when it was never set).
func WithUpdatePoolName(updatePoolName func(string) error) Option {
	return func(o *options) error {
		if updatePoolName != nil {
			o.updatePoolName = updatePoolName
		}
		return nil
	}
}

// WithGetPoolName sets the pool name getter. A nil getter keeps the default.
func WithGetPoolName(getPoolName func() string) Option {
	return func(o *options) error {
		if getPoolName != nil {
			o.getPoolName = getPoolName
		}
		return nil
	}
}

func WithChainName(n string) Option {
	return func(o *options) error {
		o.chainName = n
		return nil
	}
}

// WithUpdateChainName sets the chain name setter. A nil setter keeps the default.
func WithUpdateChainName(updateChainName func(string) error) Option {
	return func(o *options) error {
		if updateChainName != nil {
			o.updateChainName = updateChainName
		}
		return nil
	}
}

// WithGetChainName sets the chain name getter. A nil getter keeps the default.
func WithGetChainName(getChainName func() string) Option {
	return func(o *options) error {
		if getChainName != nil {
			o.getChainName = getChainName
		}
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

// WithSelfPeerID sets the peer ID for the local node (derived from private key).
// Used for authorization checks (replaces h.ID() when no libp2p host is present).
func WithSelfPeerID(id peer.ID) Option {
	return func(o *options) error {
		o.selfPeerID = id
		return nil
	}
}

// WithClusterPeerID sets the IPFS cluster peer ID (original identity, not HMAC-derived).
// This is the peer ID registered on-chain for pool membership checks.
func WithClusterPeerID(id peer.ID) Option {
	return func(o *options) error {
		o.clusterPeerID = id
		return nil
	}
}

// WithRequestSigning enables signed request headers on outgoing HTTP requests.
// The private key is used to sign requests so the receiving go-fula can verify the caller.
func WithRequestSigning(key crypto.PrivKey) Option {
	return func(o *options) error {
		o.signingKey = key
		return nil
	}
}

// WithOnPoolConfigCleared sets a hook run after a pool leave or reconcile removed the pool from the config (the
// blox uses it to restart the fula services so ipfs-cluster stops following the old pool).
func WithOnPoolConfigCleared(fn func()) Option {
	return func(o *options) error {
		o.onPoolConfigCleared = fn
		return nil
	}
}

// WithPoolHost marks the node as a pool host (--poolHost): pool leave requests and pool reconcile never clear its pool.
func WithPoolHost(b bool) Option {
	return func(o *options) error {
		o.poolHost = b
		return nil
	}
}

// WithClientProtocolID sets the libp2p protocol ID used when dialing through kubo p2p.
func WithClientProtocolID(pid string) Option {
	return func(o *options) error {
		o.clientProtocolID = pid
		return nil
	}
}

// withListenAddrs moves the proxy and ping servers off ProxyListenAddr / PingListenAddr (tests use
// "127.0.0.1:0" so parallel or repeated Starts don't collide). Empty keeps the default.
func withListenAddrs(proxy, ping string) Option {
	return func(o *options) error {
		if proxy != "" {
			o.proxyListenAddr = proxy
		}
		if ping != "" {
			o.pingListenAddr = ping
		}
		return nil
	}
}

// withChainConfigs replaces the built-in chain configurations (RPC endpoints, contracts), so tests can point the
// EVM calls at a mock server.
func withChainConfigs(c map[string]ChainConfig) Option {
	return func(o *options) error {
		o.chainConfigOverride = c
		return nil
	}
}
