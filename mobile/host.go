package fulaMobile

import (
	"context"
	"fmt"
	"io"
	"os"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"

	options "github.com/ipfs/interface-go-ipfs-core/options"
	config "github.com/ipfs/kubo/config"
)

const (
	algorithmDefault    = options.Ed25519Key
	algorithmOptionName = "algorithm"
	bitsOptionName      = "bits"
	emptyRepoOptionName = "empty-repo"
	profileOptionName   = "profile"
)

func create(ctx context.Context, configRoot string) (host.Host, error) {
	// Now, normally you do not just want a simple host, you want
	// that is fully configured to best support your p2p application.
	// Let's create a second host setting some more options.
	// Set your own keypair
	con, err := connmgr.NewConnManager(10, 100)
	if err != nil {
		// TODO: Use retry logic instead of panic
		// panic(err)
		return nil, err
	}

	if !configIsInitialized(configRoot) {
		var conf *config.Config

		if conf == nil {
			identity, err := config.CreateIdentity(os.Stdout, []options.KeyGenerateOption{
				options.Key.Type(algorithmDefault),
			})
			if err != nil {
				// TODO: Use retry logic instead of panic
				// panic(err)
				return nil, err
			}
			conf, err = config.InitWithIdentity(identity)
			if err != nil {
				// TODO: Use retry logic instead of panic
				// panic(err)
				return nil, err
			}
		}

		if err = doInit(os.Stdout, configRoot, conf); err != nil {
			// TODO: Use retry logic instead of panic
			// panic(err)
			return nil, err
		}
	}

	cfg, err := openConfig(configRoot)
	if err != nil {
		// TODO: Use retry logic instead of panic
		// panic(err)
		return nil, err
	}
	sk, err := cfg.Identity.DecodePrivateKey("passphrase todo!")
	if err != nil {
		// TODO: Use retry logic instead of panic
		// panic(err)
		return nil, err
	}

	opt := []libp2p.Option{
		libp2p.DefaultTransports,
		libp2p.DefaultSecurity,
		// Use the keypair we generated
		libp2p.Identity(sk),
		// Multiple listen addresses
		libp2p.ListenAddrStrings(
			cfg.Addresses.Swarm...,
		),
		// Let's prevent our peer from having too many
		// connections by attaching a connection manager.
		libp2p.ConnectionManager(con),
		// libp2p.DefaultMuxers,
		// Let this host use relays and advertise itself on relays if
		// it finds it is behind NAT. Use libp2p.Relay(options...) to
		// enable active relays and more.
		// libp2p.EnableAutoRelay(),
		// libp2p.EnableAutoRelay(),
		// If you want to help other peers to figure out if they are behind
		// NATs, you can launch the server-side of AutoNAT too (AutoRelay
		// already runs the client)
		//
		// This service is highly rate-limited and should not cause any
		// performance issues.
		libp2p.EnableNATService(),
		libp2p.EnableHolePunching(),
	}

	basicHost, err := libp2p.New(opt...)
	if err != nil {
		return nil, err
	}
	return basicHost, nil
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
