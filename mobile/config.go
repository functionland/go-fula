package fulamobile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/exchange"
	ipfsPath "github.com/ipfs/boxo/path"
	blockformat "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	dssync "github.com/ipfs/go-datastore/sync"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/ipfs/kubo/client/rpc"
	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	iface "github.com/ipfs/kubo/core/coreiface"
	kubolibp2p "github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/multiformats/go-multiaddr"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	noopExchange = "noop"
)

var devRelays = []string{
	"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835",
	//"/dns/alpha-relay.dev.fx.land/tcp/4001/p2p/12D3KooWFLhr8j6LTF7QV1oGCn3DVNTs1eMz2u4KCDX6Hw3BFyag",
	//"/dns/bravo-relay.dev.fx.land/tcp/4001/p2p/12D3KooWA2JrcPi2Z6i2U8H3PLQhLYacx6Uj9MgexEsMsyX6Fno7",
	//"/dns/charlie-relay.dev.fx.land/tcp/4001/p2p/12D3KooWKaK6xRJwjhq6u6yy4Mw2YizyVnKxptoT9yXMn3twgYns",
	//"/dns/delta-relay.dev.fx.land/tcp/4001/p2p/12D3KooWDtA7kecHAGEB8XYEKHBUTt8GsRfMen1yMs7V85vrpMzC",
	//"/dns/echo-relay.dev.fx.land/tcp/4001/p2p/12D3KooWQBigsW1tvGmZQet8t5MLMaQnDJKXAP2JNh7d1shk2fb2",
}

type Config struct {
	Identity   []byte
	StorePath  string
	ConfigPath string
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
	mc.ipfsAPI = nil
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
			autorelay.WithBootDelay(30*time.Second),
			autorelay.WithMinInterval(10*time.Second),
		))
	}

	if cfg.ForceReachabilityPrivate {
		hopts = append(hopts, libp2p.ForceReachabilityPrivate())
	}
	if len(cfg.Identity) != 0 {
		pk, err := crypto.UnmarshalPrivateKey(cfg.Identity)
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
		mc.bl, err = blockchain.NewFxBlockchain(mc.h, nil, nil,
			blockchain.NewSimpleKeyStorer(""),
			blockchain.WithAuthorizer(mc.h.ID()),
			blockchain.WithAllowTransientConnection(cfg.AllowTransientConnection),
			blockchain.WithBlockchainEndPoint(cfg.BlockchainEndpoint),
			blockchain.WithRelays(cfg.StaticRelays),
			blockchain.WithTopicName(cfg.PoolName),
			blockchain.WithTimeout(65))
		if err != nil {
			return err
		}
		if mc.bloxPid != "" {
			// Explicitly authorize the Blox ID if its address is specified.
			if err := mc.SetAuth(mc.h.ID().String(), mc.bloxPid.String(), true); err != nil {
				return err
			}
		}
	}
	return mc.ex.Start(context.TODO())
}

func CustomStorageWriteOpener(ipfsNode *core.IpfsNode) linking.BlockWriteOpener {
	return func(lnkCtx linking.LinkContext) (io.Writer, linking.BlockWriteCommitter, error) {
		// The returned io.Writer is where the data will be written.
		// The BlockWriteCommitter function will be called to "commit" the data once writing is done.
		var buffer bytes.Buffer
		committer := func(lnk ipld.Link) error {
			// Convert the IPLD link to a CID.
			c, err := cid.Parse(lnk.String())
			if err != nil {
				return err
			}

			// Create an IPFS block with the data from the buffer and the CID.
			block, err := blockformat.NewBlockWithCid(buffer.Bytes(), c)
			if err != nil {
				return err
			}

			// Store the block using the IPFS node's blockstore.
			err = ipfsNode.Blockstore.Put(context.Background(), block)
			return err
		}

		return &buffer, committer, nil
	}
}
func CustomStorageReadOpener(ipfsApi iface.CoreAPI) ipld.BlockReadOpener {
	return func(lnkCtx ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
		cidLink, ok := lnk.(cidlink.Link)
		if !ok {
			return nil, fmt.Errorf("link is not a cid link")
		}
		// Convert the CID link to a path
		p, err := ipfsPath.NewPath("/ipfs/" + cidLink.Cid.String())
		if err != nil {
			return nil, err
		}
		// Use the Block API's Get method to obtain a reader for the block's data
		reader, err := ipfsApi.Block().Get(context.Background(), p)
		if err != nil {
			return nil, err
		}
		return reader, nil
	}
}

func (cfg *Config) initIpfs(ctx context.Context, mc *Client) error {
	var err error
	nodeMultiAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5001")
	if err != nil {
		panic(fmt.Errorf("invalid multiaddress: %w", err))
	}
	log.Print("nodeMultiAddr created")
	nodeApi, err := rpc.NewApi(nodeMultiAddr)
	if err != nil {
		panic(fmt.Errorf("invalid nodeApi: %w", err))
	}
	log.Print("nodeApi created")
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
			autorelay.WithBootDelay(30*time.Second),
			autorelay.WithMinInterval(10*time.Second),
		))
	}

	if cfg.ForceReachabilityPrivate {
		hopts = append(hopts, libp2p.ForceReachabilityPrivate())
	}
	log.Print("ForceReachabilityPrivate set")
	if len(cfg.Identity) != 0 {
		pk, err := crypto.UnmarshalPrivateKey(cfg.Identity)
		if err != nil {
			return err
		}
		log.Println(pk)
		hopts = append(hopts, libp2p.Identity(pk))
	}
	log.Print("Identity set")
	if mc.h, err = libp2p.New(hopts...); err != nil {
		return err
	}
	log.Print("mc libp2p created")
	log.Println(mc.h.ID())
	repo, err := CreateCustomRepo(ctx, cfg, cfg.ConfigPath, mc.h, &badger.DefaultOptions, cfg.StorePath, "90%", false)
	if err != nil {
		return err
	}
	log.Print("mc repo created")
	ipfsConfig := &core.BuildCfg{
		Online:    true,
		Permanent: false,
		Host:      CustomHostOption(mc.h),
		Routing:   kubolibp2p.DHTOption,
		Repo:      repo,
	}
	log.Print("mc ipfsConfig created")
	ipfsNode, err := core.NewNode(ctx, ipfsConfig)
	if err != nil {
		return err
	}
	log.Print("mc ipfsNode created")
	// ipfsHostId := ipfsNode.PeerHost.ID()
	// ipfsId := ipfsNode.Identity.String()

	ipfsAPI, err := coreapi.NewCoreAPI(ipfsNode)
	if err != nil {
		panic(fmt.Errorf("failed to create IPFS API: %w", err))
	}

	mc.ipfsNode = ipfsNode
	mc.ipfsAPI = ipfsAPI
	log.Print("mc ipfsAPI created")
	if cfg.StorePath == "" {
		mc.ds = dssync.MutexWrap(datastore.NewMapDatastore())
	} else {
		options := &badger.DefaultOptions
		options.SyncWrites = cfg.SyncWrites
		log.Print("mc ds options created")
		mc.ds = ipfsNode.Repo.Datastore()
	}
	log.Print("mc ds created")
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
	}
	log.Print("mc BloxAddr set")
	mc.ls = cidlink.DefaultLinkSystem()
	mc.ls.StorageWriteOpener = CustomStorageWriteOpener(ipfsNode)
	mc.ls.StorageReadOpener = CustomStorageReadOpener(ipfsAPI)
	switch cfg.Exchange {
	case noopExchange:
		mc.ex = exchange.NoopExchange{}
	default:
		mc.ex, err = exchange.NewFxExchange(mc.h, mc.ls,
			exchange.WithAuthorizer(mc.h.ID()),
			exchange.WithAllowTransientConnection(cfg.AllowTransientConnection),
			exchange.WithIpniPublishDisabled(true),
			exchange.WithMaxPushRate(50),
			exchange.WithIPFSApi(ipfsAPI),
		)
		if err != nil {
			return err
		}
		mc.bl, err = blockchain.NewFxBlockchain(mc.h, nil, nil,
			blockchain.NewSimpleKeyStorer(""),
			blockchain.WithAuthorizer(mc.h.ID()),
			blockchain.WithAllowTransientConnection(cfg.AllowTransientConnection),
			blockchain.WithBlockchainEndPoint(cfg.BlockchainEndpoint),
			blockchain.WithRelays(cfg.StaticRelays),
			blockchain.WithTopicName(cfg.PoolName),
			blockchain.WithIpfsClient(nodeApi),
			blockchain.WithTimeout(65))
		if err != nil {
			return err
		}
		if mc.bloxPid != "" {
			// Explicitly authorize the Blox ID if its address is specified.
			if err := mc.SetAuth(mc.h.ID().String(), mc.bloxPid.String(), true); err != nil {
				return err
			}
		}
	}

	ps, err := pubsub.NewGossipSub(context.Background(), mc.h)
	if err != nil {
		return fmt.Errorf("failed to create pubsub: %w", err)
	}
	mc.ps = ps

	ctx3, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel() // Ensure the context cancel function is called to free resources

	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()

	// This loop will repeatedly check if the node is online until the context is done (timeout)
	for {
		select {
		case <-ctx3.Done():
			// Context expired
			if ctx3.Err() == context.DeadlineExceeded {
				return fmt.Errorf("timeout waiting for IPFS node to come online")
			}
			return ctx3.Err()
		case <-ticker.C:
			if ipfsNode.IsOnline {
				// Proceed if the node is online
				log.Println("IPFS node is online, proceeding with exchange start")
				return mc.ex.Start(context.Background())
			} else {
				log.Println("Waiting for IPFS node to come online...")
			}
		}
	}
}
