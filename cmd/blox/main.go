package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/functionland/go-fula/blox"
	"github.com/functionland/go-fula/exchange"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	badger "github.com/ipfs/go-ds-badger"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipni/index-provider/engine"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/mdp/qrterminal"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"gopkg.in/yaml.v3"
)

var (
	logger = logging.Logger("fula/cmd/blox")
	app    struct {
		cli.App
		initOnly   bool
		wireless   bool
		configPath string
		config     struct {
			Identity                  string        `yaml:"identity"`
			StoreDir                  string        `yaml:"storeDir"`
			PoolName                  string        `yaml:"poolName"`
			LogLevel                  string        `yaml:"logLevel"`
			ListenAddrs               []string      `yaml:"listenAddrs"`
			Authorizer                string        `yaml:"authorizer"`
			AuthorizedPeers           []string      `yaml:"authorizedPeers"`
			StaticRelays              []string      `yaml:"staticRelays"`
			ForceReachabilityPrivate  bool          `yaml:"forceReachabilityPrivate"`
			AllowTransientConnection  bool          `yaml:"allowTransientConnection"`
			IpniPublishDisabled       bool          `yaml:"ipniPublishDisabled"`
			IpniPublishInterval       time.Duration `yaml:"ipniPublishInterval"`
			IpniPublishDirectAnnounce []string      `yaml:"IpniPublishDirectAnnounce"`
			IpniPublisherIdentity     string        `yaml:"ipniPublisherIdentity"`

			listenAddrs    cli.StringSlice
			staticRelays   cli.StringSlice
			directAnnounce cli.StringSlice
		}
	}
)

func init() {
	app.App = cli.App{
		Name:  "blox",
		Usage: "Start a new blox instance",
		Flags: []cli.Flag{
			&cli.PathFlag{
				Name:        "config",
				DefaultText: ".flua/blox/config.yaml under current user home directory.",
				EnvVars:     []string{"FULA_BLOX_CONFIG"},
			},
			altsrc.NewStringFlag(&cli.StringFlag{
				Name:        "identity",
				DefaultText: "Randomly generated lib2p identity.",
				Destination: &app.config.Identity,
				EnvVars:     []string{"FULA_BLOX_IDENTITY"},
			}),
			altsrc.NewPathFlag(&cli.PathFlag{
				Name:        "storeDir",
				DefaultText: ".flua/blox/store/ under current user home directory.",
				Destination: &app.config.StoreDir,
				EnvVars:     []string{"FULA_BLOX_STORE_DIR"},
			}),
			altsrc.NewStringFlag(&cli.StringFlag{
				Name:        "poolName",
				Destination: &app.config.PoolName,
				EnvVars:     []string{"FULA_BLOX_POOL_NAME"},
				Value:       "",
			}),
			altsrc.NewStringFlag(&cli.StringFlag{
				Name:        "logLevel",
				Destination: &app.config.LogLevel,
				EnvVars:     []string{"FULA_BLOX_LOG_LEVEL"},
				Value:       "info",
			}),
			altsrc.NewStringFlag(&cli.StringFlag{
				Name:        "authorizer",
				Usage:       "Peer ID that is allowed to manage authorization",
				DefaultText: "Peer ID of Blox itself, i.e. --identity",
				Destination: &app.config.Authorizer,
				EnvVars:     []string{"FULA_BLOX_AUTHORIZER"},
			}),
			altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
				Name:        "listenAddr",
				Destination: &app.config.listenAddrs,
				Value:       cli.NewStringSlice("/ip4/0.0.0.0/tcp/40001", "/ip4/0.0.0.0/udp/40001/quic"),
			}),
			altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
				Name:        "staticRelays",
				Destination: &app.config.staticRelays,
				Value:       cli.NewStringSlice("/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"),
			}),
			altsrc.NewBoolFlag(&cli.BoolFlag{
				Name:        "forceReachabilityPrivate",
				Aliases:     []string{"frp"},
				Usage:       "Weather to force the local libp2p node to think it is behind NAT.",
				Destination: &app.config.ForceReachabilityPrivate,
				Value:       true,
			}),
			altsrc.NewBoolFlag(&cli.BoolFlag{
				Name:        "allowTransientConnection",
				Aliases:     []string{"atc"},
				Usage:       "Weather to allow transient connection to other participants.",
				Destination: &app.config.AllowTransientConnection,
				Value:       true,
			}),
			altsrc.NewBoolFlag(&cli.BoolFlag{
				Name:        "ipniPublisherDisabled",
				Usage:       "Weather to disable IPNI publisher.",
				Destination: &app.config.IpniPublishDisabled,
			}),
			altsrc.NewDurationFlag(&cli.DurationFlag{
				Name:        "ipniPublishInterval",
				Usage:       "The interval at which IPNI advertisements are published.",
				Destination: &app.config.IpniPublishInterval,
				Value:       10 * time.Second,
			}),
			altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
				Name:        "ipniPublishDirectAnnounce",
				Usage:       "The list of IPNI URLs to which make direct announcements.",
				Destination: &app.config.directAnnounce,
				Value:       cli.NewStringSlice("https://cid.contact/ingest/announce"),
			}),
			altsrc.NewStringFlag(&cli.StringFlag{
				Name:        "ipniPublisherIdentity",
				DefaultText: "Randomly generated lib2p identity.",
				Destination: &app.config.IpniPublisherIdentity,
			}),
			&cli.BoolFlag{
				Name:        "initOnly",
				Usage:       "Weather to only initialise config and quit.",
				Destination: &app.initOnly,
				Value:       false,
			},
		},
		Before:    before,
		Action:    action,
		Copyright: "fx.land",
	}
}

func before(ctx *cli.Context) error {
	_ = logging.SetLogLevelRegex("fula/.*", app.config.LogLevel)
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	app.configPath = ctx.Path("config")
	if app.configPath == "" {
		app.configPath = path.Join(homeDir, ".fula", "blox", "config.yaml")
		if err := ctx.Set("config", app.configPath); err != nil {
			return err
		}
	}
	if stats, err := os.Stat(app.configPath); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path.Dir(app.configPath), 0700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if stats.IsDir() {
		return errors.New("config cannot be a directory")
	} else {
		ff := altsrc.NewYamlSourceFromFlagFunc("config")
		if err := altsrc.InitInputSourceWithContext(app.Flags, ff)(ctx); err != nil {
			return err
		}
	}

	// Generate identity at random if not set.
	if app.config.Identity == "" {
		k, _, err := crypto.GenerateEd25519Key(nil)
		if err != nil {
			return err
		}
		km, err := crypto.MarshalPrivateKey(k)
		if err != nil {
			return err
		}
		app.config.Identity = base64.StdEncoding.EncodeToString(km)
	}
	if app.config.Authorizer == "" {
		km, err := base64.StdEncoding.DecodeString(app.config.Identity)
		if err != nil {
			return err
		}
		key, err := crypto.UnmarshalPrivateKey(km)
		if err != nil {
			return err
		}
		pid, err := peer.IDFromPrivateKey(key)
		if err != nil {
			return err
		}
		app.config.Authorizer = pid.String()
	}
	if app.config.IpniPublisherIdentity == "" {
		k, _, err := crypto.GenerateEd25519Key(nil)
		if err != nil {
			return err
		}
		km, err := crypto.MarshalPrivateKey(k)
		if err != nil {
			return err
		}
		app.config.IpniPublisherIdentity = base64.StdEncoding.EncodeToString(km)
	}
	// Initialize store directory if not set.
	if app.config.StoreDir == "" {
		app.config.StoreDir = path.Join(homeDir, ".fula", "blox", "store")
	}
	if stats, err := os.Stat(app.config.StoreDir); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(app.config.StoreDir, 0700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !stats.IsDir() {
		return errors.New("storeDir must be a directory")
	}
	app.config.ListenAddrs = app.config.listenAddrs.Value()
	app.config.StaticRelays = app.config.staticRelays.Value()
	app.config.IpniPublishDirectAnnounce = app.config.directAnnounce.Value()
	yc, err := yaml.Marshal(app.config)
	if err != nil {
		return err
	}
	return os.WriteFile(app.configPath, yc, 0700)
}

func updateConfig(p []peer.ID) error {
	// Load existing config file
	configData, err := os.ReadFile(app.configPath)
	if err != nil {
		return err
	}

	// Parse the existing config file
	if err := yaml.Unmarshal(configData, &app.config); err != nil {
		return err
	}

	// Create a map to hold unique peer IDs
	uniquePeers := make(map[string]bool)

	// Add existing AuthorizedPeers to the map
	for _, pidStr := range app.config.AuthorizedPeers {
		uniquePeers[pidStr] = true
	}

	// Convert the slice of peer.ID to a slice of strings
	for _, pid := range p {
		// Convert the peer.ID to string
		pidStr := pid.String()
		// Check if the peer.ID is already in the map
		if !uniquePeers[pidStr] {
			// If it's not in the map, add it to the map and the slice
			uniquePeers[pidStr] = true
			app.config.AuthorizedPeers = append(app.config.AuthorizedPeers, pidStr)
		}
	}

	logger.Infof("Authorized peers: %v", app.config.AuthorizedPeers)
	// Write back the updated config to the file
	configData, err = yaml.Marshal(app.config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(app.configPath, configData, 0700); err != nil {
		return err
	}

	return nil
}

func updatePoolName(newPoolName string) error {
	// Load existing config file
	configData, err := os.ReadFile(app.configPath)
	if err != nil {
		return err
	}

	// Parse the existing config file
	if err := yaml.Unmarshal(configData, &app.config); err != nil {
		return err
	}

	// Update the pool name
	app.config.PoolName = newPoolName

	logger.Infof("Updated pool name to: %s", app.config.PoolName)

	// Marshal the updated config back to YAML
	configData, err = yaml.Marshal(app.config)
	if err != nil {
		return err
	}

	// Write the updated config back to the file
	if err := os.WriteFile(app.configPath, configData, 0700); err != nil {
		return err
	}

	return nil
}

func action(ctx *cli.Context) error {
	authorizer, err := peer.Decode(app.config.Authorizer)
	if err != nil {
		return err
	}
	// Decode the authorized peers from strings to peer.IDs
	authorizedPeers := make([]peer.ID, len(app.config.AuthorizedPeers))
	for i, authorizedPeer := range app.config.AuthorizedPeers {
		id, err := peer.Decode(authorizedPeer)
		if err != nil {
			return fmt.Errorf("unable to decode authorized peer: %w", err)
		}
		authorizedPeers[i] = id
	}
	km, err := base64.StdEncoding.DecodeString(app.config.Identity)
	if err != nil {
		return err
	}
	k, err := crypto.UnmarshalPrivateKey(km)
	if err != nil {
		return err
	}

	ipnikm, err := base64.StdEncoding.DecodeString(app.config.IpniPublisherIdentity)
	if err != nil {
		return err
	}
	ipnik, err := crypto.UnmarshalPrivateKey(ipnikm)
	if err != nil {
		return err
	}

	if app.initOnly {
		fmt.Printf("Application config initialised at: %s\n", app.configPath)
		pid, err := peer.IDFromPrivateKey(k)
		if err != nil {
			return err
		}
		ipnipid, err := peer.IDFromPrivateKey(ipnik)
		if err != nil {
			return err
		}
		fmt.Printf("   blox peer ID: %s\n", pid.String())
		fmt.Printf("   blox IPNI publisher peer ID: %s\n", ipnipid.String())
		return nil
	}

	hopts := []libp2p.Option{
		libp2p.ListenAddrStrings(app.config.ListenAddrs...),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
	}

	if app.config.ForceReachabilityPrivate {
		hopts = append(hopts, libp2p.ForceReachabilityPrivate())
	}

	sr := make([]peer.AddrInfo, 0, len(app.config.StaticRelays))
	for _, relay := range app.config.StaticRelays {
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
	if len(sr) != 0 {
		hopts = append(hopts, libp2p.EnableAutoRelayWithStaticRelays(sr, autorelay.WithNumRelays(1)))
	}

	bopts := append(hopts, libp2p.Identity(k))
	h, err := libp2p.New(bopts...)
	if err != nil {
		return err
	}
	iopts := append(hopts, libp2p.Identity(ipnik))
	ipnih, err := libp2p.New(iopts...)
	if err != nil {
		return err
	}
	ds, err := badger.NewDatastore(app.config.StoreDir, &badger.DefaultOptions)
	if err != nil {
		return err
	}
	bb, err := blox.New(
		blox.WithHost(h),
		blox.WithDatastore(ds),
		blox.WithPoolName(app.config.PoolName),
		blox.WithStoreDir(app.config.StoreDir),
		blox.WithRelays(app.config.StaticRelays),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithExchangeOpts(
			exchange.WithUpdateConfig(updateConfig),
			exchange.WithAuthorizer(authorizer),
			exchange.WithAuthorizedPeers(authorizedPeers),
			exchange.WithAllowTransientConnection(app.config.AllowTransientConnection),
			exchange.WithIpniPublishDisabled(app.config.IpniPublishDisabled),
			exchange.WithIpniPublishInterval(app.config.IpniPublishInterval),
			exchange.WithIpniProviderEngineOptions(
				engine.WithHost(ipnih),
				engine.WithDatastore(namespace.Wrap(ds, datastore.NewKey("ipni/ads"))),
				engine.WithPublisherKind(engine.DataTransferPublisher),
				engine.WithDirectAnnounce(app.config.IpniPublishDirectAnnounce...),
			),
		),
	)
	if err != nil {
		return err
	}
	if err := bb.Start(ctx.Context); err != nil {
		return err
	}
	logger.Info("Started blox", "addrs", h.Addrs())
	printMultiaddrAsQR(h)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	logger.Info("Shutting down blox")
	return bb.Shutdown(context.Background())
}

func printMultiaddrAsQR(h host.Host) {
	var addr multiaddr.Multiaddr
	addrs := h.Addrs()
	switch len(addrs) {
	case 0:
	case 1:
		addr = addrs[0]
	default:
		// Prefer printing public address if there is one.
		// Otherwise, fallback on a non-loopback address.
		for _, a := range addrs {
			if manet.IsPublicAddr(a) {
				addr = a
				break
			}
			if !manet.IsIPLoopback(a) {
				addr = a
			}
		}
	}
	if addr == nil {
		logger.Warn("blox has no multiaddrs")
		return
	}
	as := fmt.Sprintf("%s/p2p/%s", addr.String(), h.ID().String())
	fmt.Printf(">>> blox multiaddr: %s\n", as)
	qrterminal.GenerateWithConfig(as, qrterminal.Config{
		Level:      qrterminal.L,
		Writer:     os.Stdout,
		HalfBlocks: false,
		BlackChar:  "%%",
		WhiteChar:  "  ",
		QuietZone:  qrterminal.QUIET_ZONE,
	})
}

func main() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
