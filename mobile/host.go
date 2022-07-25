package mobile

import (
	"context"
	"fmt"
	"io"
	"os"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"

	config "github.com/ipfs/go-ipfs/config"
	"github.com/ipfs/go-ipfs/core/bootstrap"
	options "github.com/ipfs/interface-go-ipfs-core/options"
)

const (
	algorithmDefault    = options.Ed25519Key
	algorithmOptionName = "algorithm"
	bitsOptionName      = "bits"
	emptyRepoOptionName = "empty-repo"
	profileOptionName   = "profile"
)

func create(ctx context.Context, configRoot string) (*rhost.RoutedHost, error) {
	// Now, normally you do not just want a simple host, you want
	// that is fully configured to best support your p2p application.
	// Let's create a second host setting some more options.
	// Set your own keypair
	con, err := connmgr.NewConnManager(10, 100)
	if err != nil {
		panic(err)
	}

	if configIsInitialized(configRoot) {
		var conf *config.Config

		if conf == nil {
			identity, err := config.CreateIdentity(os.Stdout, []options.KeyGenerateOption{
				options.Key.Type(algorithmDefault),
			})
			if err != nil {
				panic(err)
			}
			conf, err = config.InitWithIdentity(identity)
			if err != nil {
				panic(err)
			}
		}
		err = doInit(os.Stdout, configRoot, conf)
		if err != nil {
			panic(err)
		}
	}

	cfg, err := openConfig(configRoot)
	if err != nil {
		panic(err)
	}
	sk, err := cfg.Identity.DecodePrivateKey("passphrase todo!")
	if err != nil {
		panic(err)
	}

	opt := []libp2p.Option{
		libp2p.DefaultTransports,
		libp2p.DefaultSecurity,
		// Use the keypair we generated
		libp2p.Identity(sk),
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/9000",      // regular tcp connections
			"/ip4/0.0.0.0/udp/9000/quic", // a UDP endpoint for the QUIC transport
		),

		// Let's prevent our peer from having too many
		// connections by attaching a connection manager.
		libp2p.ConnectionManager(con),
		libp2p.DefaultMuxers,
		// Let this host use relays and advertise itself on relays if
		// it finds it is behind NAT. Use libp2p.Relay(options...) to
		// enable active relays and more.
		// libp2p.EnableAutoRelay(),
		libp2p.EnableAutoRelay(),
		// If you want to help other peers to figure out if they are behind
		// NATs, you can launch the server-side of AutoNAT too (AutoRelay
		// already runs the client)
		//
		// This service is highly rate-limited and should not cause any
		// performance issues.
		libp2p.EnableNATService(),
	}

	basicHost, err := libp2p.New(opt...)
	if err != nil {
		return nil, err
	}

	// Construct a datastore (needed by the DHT). This is just a simple, in-memory thread-safe datastore.
	dstore := dsync.MutexWrap(ds.NewMapDatastore())

	// Make the DHT
	kDht := dht.NewDHT(ctx, basicHost, dstore)
	bootstrapPeers, _ := cfg.BootstrapPeers()

	// connect to the chosen ipfs nodes
	_, err = bootstrap.Bootstrap(peer.ID(cfg.Identity.PeerID), basicHost, kDht, bootstrap.BootstrapConfigWithPeers(bootstrapPeers))
	if err != nil {
		log.Error("bootstrap failed. ", err)
		return nil, err
	}

	// Make the routed host
	routedHost := rhost.Wrap(basicHost, kDht)

	log.Infof("Fula Bootsraped and ready with ID:", routedHost.ID())
	return routedHost, nil
}

func doInit(out io.Writer, repoRoot string, conf *config.Config) error {
	if _, err := fmt.Fprintf(out, "initializing IPFS node at %s\n", repoRoot); err != nil {
		return err
	}

	if err := checkWritable(repoRoot); err != nil {
		return err
	}

	if err := initConfig(repoRoot, conf); err != nil {
		return err
	}

	return nil
}
