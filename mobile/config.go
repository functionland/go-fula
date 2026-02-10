package fulamobile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/exchange"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	dssync "github.com/ipfs/go-datastore/sync"
	badger "github.com/ipfs/go-ds-badger"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p"
	gostream "github.com/libp2p/go-libp2p-gostream"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/multiformats/go-multiaddr"
)

const (
	noopExchange = "noop"
)

var log = logging.Logger("fula/mobile")

var devRelays = []string{
	"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835",
	//"/dns/alpha-relay.dev.fx.land/tcp/4001/p2p/12D3KooWFLhr8j6LTF7QV1oGCn3DVNTs1eMz2u4KCDX6Hw3BFyag",
	//"/dns/bravo-relay.dev.fx.land/tcp/4001/p2p/12D3KooWA2JrcPi2Z6i2U8H3PLQhLYacx6Uj9MgexEsMsyX6Fno7",
	//"/dns/charlie-relay.dev.fx.land/tcp/4001/p2p/12D3KooWKaK6xRJwjhq6u6yy4Mw2YizyVnKxptoT9yXMn3twgYns",
	//"/dns/delta-relay.dev.fx.land/tcp/4001/p2p/12D3KooWDtA7kecHAGEB8XYEKHBUTt8GsRfMen1yMs7V85vrpMzC",
	//"/dns/echo-relay.dev.fx.land/tcp/4001/p2p/12D3KooWQBigsW1tvGmZQet8t5MLMaQnDJKXAP2JNh7d1shk2fb2",
}

type Config struct {
	Identity  []byte
	StorePath string
	// Exchange specifies the DAG exchange protocol for Fula mobile client. If left unspecified,
	// The default FxExchange protocol is used which will attempt to make remote connections
	// when links are stored and retrieved.
	//
	// For testing purposes, the value may be set to `noop`, in which case, no remote connections
	// will be made and the requested exchange is simply logged. When the value is set to `noop`
	// the BloxAddr may also be left empty.
	Exchange string
	BloxAddr string
	// StaticRelays specifies a list of static relays used by libp2p auto-relay.
	// Defaults to fx.land managed relay if unspecified.
	StaticRelays []string

	// ForceReachabilityPrivate configures weather the libp2p should always think that it is behind
	// NAT.
	ForceReachabilityPrivate bool

	// DisableResourceManger sets whether to disable the libp2p resource manager.
	DisableResourceManger bool

	// SyncWrites assures that writes to the local datastore are flushed to the backing store as
	// soon as they are written. By default, writes are not synchronized to disk until either the
	// client is shut down or Client.Flush is explicitly called.
	SyncWrites bool

	// AllowTransientConnection allows transient connectivity via relay when direct connection is
	// not possible. Defaults to enabled if unspecified.
	AllowTransientConnection bool
	PoolName                 string
	BlockchainEndpoint       string

	// IpfsDHTLookupDisabled disables IPFS DHT peer discovery fallback.
	// When false (default), the client will query the standard IPFS DHT to discover
	// kubo's current addresses if direct and relay connections fail.
	IpfsDHTLookupDisabled bool

	// TODO: we don't need to take BloxAddr when there is a discovery mechanism facilitated via fx.land.
	//       For now we manually take BloxAddr as config.
}

// NewConfig instantiates a new Config with default values.
func NewConfig() *Config {
	return &Config{
		StaticRelays:             devRelays,
		ForceReachabilityPrivate: true,
		AllowTransientConnection: true,
		PoolName:                 "0",
		BlockchainEndpoint:       "api.node3.functionyard.fula.network",
		DisableResourceManger:    true,
	}
}

func (cfg *Config) init(mc *Client) error {
	// Apply defaults for zero-value Config created via gomobile constructors
	if cfg.StaticRelays == nil {
		cfg.StaticRelays = devRelays
	}

	var err error
	hopts := []libp2p.Option{
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
	}
	cfg.DisableResourceManger = true
	if cfg.DisableResourceManger {
		hopts = append(hopts, libp2p.ResourceManager(&network.NullResourceManager{}))
	}
	mc.relays = cfg.StaticRelays
	if len(cfg.StaticRelays) != 0 {
		sr := make([]peer.AddrInfo, 0, len(cfg.StaticRelays))
		for _, relay := range cfg.StaticRelays {
			rma, err := multiaddr.NewMultiaddr(relay)
			if err != nil {
				return err
			}
			rai, err := peer.AddrInfoFromP2pAddr(rma)
			if err != nil {
				return err
			}
			sr = append(sr, *rai)
		}
		hopts = append(hopts, libp2p.EnableAutoRelayWithStaticRelays(sr,
			autorelay.WithMinCandidates(1),
			autorelay.WithNumRelays(1),
			autorelay.WithBootDelay(15*time.Second),
			autorelay.WithMinInterval(10*time.Second),
		))
	}

	if cfg.ForceReachabilityPrivate {
		hopts = append(hopts, libp2p.ForceReachabilityPrivate())
	}
	var pk crypto.PrivKey
	if len(cfg.Identity) != 0 {
		var err error
		pk, err = crypto.UnmarshalPrivateKey(cfg.Identity)
		if err != nil {
			return err
		}
		hopts = append(hopts, libp2p.Identity(pk))
	}
	if mc.h, err = libp2p.New(hopts...); err != nil {
		return err
	}

	if cfg.StorePath == "" {
		mc.ds = dssync.MutexWrap(datastore.NewMapDatastore())
	} else {
		options := &badger.DefaultOptions
		options.SyncWrites = cfg.SyncWrites
		mc.ds, err = badger.NewDatastore(cfg.StorePath, options)
		if err != nil {
			return err
		}
	}
	if cfg.BloxAddr == "" {
		if cfg.Exchange != noopExchange {
			return errors.New("the BloxAddr must be specified until autodiscovery service is implemented; " +
				"for testing purposes, BloxAddr may be omitted only when Exchange is set to `noop`")
		}
	} else {
		bloxAddr, err := peer.AddrInfoFromString(cfg.BloxAddr)
		if err != nil {
			return err
		}
		mc.h.Peerstore().AddAddrs(bloxAddr.ID, bloxAddr.Addrs, peerstore.PermanentAddrTTL)
		mc.bloxPid = bloxAddr.ID

		// When using direct IP, also add relay circuit addresses as fallback
		// so libp2p can transparently try relay if direct connection fails
		if !strings.Contains(cfg.BloxAddr, "p2p-circuit") && len(cfg.StaticRelays) > 0 {
			for _, relay := range cfg.StaticRelays {
				circuitMA, err := multiaddr.NewMultiaddr(relay + "/p2p-circuit")
				if err != nil {
					continue
				}
				mc.h.Peerstore().AddAddr(bloxAddr.ID, circuitMA, time.Hour)
			}
		}
	}
	mc.ls = cidlink.DefaultLinkSystem()
	mc.ls.StorageWriteOpener = func(ctx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		buf := bytes.NewBuffer(nil)
		return buf, func(l ipld.Link) error {
			ctx := ctx.Ctx
			k := datastore.NewKey(l.Binary())
			if err := mc.ds.Put(ctx, k, buf.Bytes()); err != nil {
				return err
			}
			if err := mc.ex.Push(ctx, mc.bloxPid, l); err != nil {
				return mc.markAsFailedPush(ctx, l)
			}
			return mc.markAsPushedSuccessfully(ctx, l)
		}, nil
	}
	mc.ls.StorageReadOpener = func(ctx ipld.LinkContext, l ipld.Link) (io.Reader, error) {
		val, err := mc.ds.Get(ctx.Ctx, datastore.NewKey(l.Binary()))
		if err == datastore.ErrNotFound {
			// Attempt to pull missing link from blox if missing from local datastore.
			if err := mc.ex.Pull(ctx.Ctx, mc.bloxPid, l); err != nil {
				return nil, err
			}
			val, err = mc.ds.Get(ctx.Ctx, datastore.NewKey(l.Binary()))
		}
		switch err {
		case nil:
			return bytes.NewBuffer(val), nil
		default:
			return nil, err
		}
	}
	switch cfg.Exchange {
	case noopExchange:
		mc.ex = exchange.NoopExchange{}
	default:
		mc.ex, err = exchange.NewFxExchange(mc.h, mc.ls,
			exchange.WithAuthorizer(mc.h.ID()),
			exchange.WithAllowTransientConnection(cfg.AllowTransientConnection),
			exchange.WithIpniPublishDisabled(true),
			exchange.WithMaxPushRate(50),
			exchange.WithDhtProviderOptions(
				dht.Datastore(namespace.Wrap(mc.ds, datastore.NewKey("dht"))),
				dht.ProtocolExtension(protocol.ID("/"+cfg.PoolName)),
				dht.ProtocolPrefix("/fula"),
				dht.Resiliency(1),
			),
		)
		if err != nil {
			return err
		}
		blcOpts := []blockchain.Option{
			blockchain.WithAuthorizer(mc.h.ID()),
			blockchain.WithSelfPeerID(mc.h.ID()),
			blockchain.WithAllowTransientConnection(cfg.AllowTransientConnection),
			blockchain.WithBlockchainEndPoint(cfg.BlockchainEndpoint),
			blockchain.WithRelays(cfg.StaticRelays),
			blockchain.WithTopicName(cfg.PoolName),
			blockchain.WithTimeout(65),
		}
		if pk != nil {
			blcOpts = append(blcOpts, blockchain.WithRequestSigning(pk))
		}
		blc, err := blockchain.NewFxBlockchain(
			blockchain.NewSimpleKeyStorer(""),
			blcOpts...)
		if err != nil {
			return err
		}
		// Initialize IPFS DHT for peer discovery fallback (non-fatal if it fails)
		if !cfg.IpfsDHTLookupDisabled && mc.bloxPid != "" {
			mc.ipfsDHTCtx, mc.ipfsDHTCancel = context.WithCancel(context.Background())
			mc.ipfsDHTReady = make(chan struct{})
			ipfsDHT, dhtErr := dht.New(mc.ipfsDHTCtx, mc.h,
				dht.Mode(dht.ModeClient),
				dht.BootstrapPeers(dht.GetDefaultBootstrapPeerAddrInfos()...),
				dht.Datastore(namespace.Wrap(mc.ds, datastore.NewKey("ipfs-dht"))),
			)
			if dhtErr != nil {
				log.Warnf("Failed to create IPFS DHT for peer discovery fallback: %v", dhtErr)
				close(mc.ipfsDHTReady) // Signal ready immediately so DialContext doesn't block
			} else {
				mc.ipfsDHT = ipfsDHT
				go func() {
					defer close(mc.ipfsDHTReady)
					if bErr := mc.ipfsDHT.Bootstrap(mc.ipfsDHTCtx); bErr != nil {
						log.Warnf("IPFS DHT bootstrap failed: %v", bErr)
						return
					}
					// Allow some time for the DHT to warm up and populate its routing table
					select {
					case <-time.After(10 * time.Second):
					case <-mc.ipfsDHTCtx.Done():
					}
				}()
			}
		}

		// Create gostream-based HTTP client that dials through kubo's p2p protocol
		p2pClient := &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					pid, err := peer.Decode(strings.TrimSuffix(addr, ".invalid:80"))
					if err != nil {
						return nil, err
					}

					// Try direct dial first (uses peerstore addresses — direct IP + relay circuits)
					conn, dialErr := gostream.Dial(ctx, mc.h, pid, "/x/fula-blockchain")
					if dialErr == nil {
						return conn, nil
					}

					// If DHT is not available, return the original error
					if mc.ipfsDHT == nil {
						return nil, dialErr
					}

					log.Infof("Direct dial failed for peer %s, attempting IPFS DHT peer discovery: %v", pid, dialErr)

					// Wait for DHT to be ready (up to 15s)
					readyCtx, readyCancel := context.WithTimeout(ctx, 15*time.Second)
					defer readyCancel()
					select {
					case <-mc.ipfsDHTReady:
					case <-readyCtx.Done():
						return nil, fmt.Errorf("direct dial failed: %w; IPFS DHT not ready in time", dialErr)
					}

					// Query DHT for peer addresses (up to 30s)
					findCtx, findCancel := context.WithTimeout(ctx, 30*time.Second)
					defer findCancel()
					addrInfo, findErr := mc.ipfsDHT.FindPeer(findCtx, pid)
					if findErr != nil {
						return nil, fmt.Errorf("direct dial failed: %w; IPFS DHT FindPeer also failed: %v", dialErr, findErr)
					}

					if len(addrInfo.Addrs) == 0 {
						return nil, fmt.Errorf("direct dial failed: %w; IPFS DHT found peer but no addresses", dialErr)
					}

					log.Infof("IPFS DHT discovered %d addresses for peer %s: %v", len(addrInfo.Addrs), pid, addrInfo.Addrs)

					// Add discovered addresses to peerstore and retry
					mc.h.Peerstore().AddAddrs(pid, addrInfo.Addrs, peerstore.TempAddrTTL)
					retryConn, retryErr := gostream.Dial(ctx, mc.h, pid, "/x/fula-blockchain")
					if retryErr != nil {
						return nil, fmt.Errorf("direct dial failed: %w; IPFS DHT retry also failed: %v", dialErr, retryErr)
					}
					return retryConn, nil
				},
			},
		}
		blc.SetP2PClient(p2pClient) // auto-wraps with signing transport if key is set
		mc.bl = blc
		if mc.bloxPid != "" {
			// Explicitly authorize the Blox ID if its address is specified.
			if err := mc.SetAuth(mc.h.ID().String(), mc.bloxPid.String(), true); err != nil {
				return err
			}
		}
	}
	return mc.ex.Start(context.TODO())
}
