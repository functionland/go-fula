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

	"github.com/functionland/go-fula/blox"
	badger "github.com/ipfs/go-ds-badger"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
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
		config struct {
			Identity    string   `yaml:"identity"`
			StoreDir    string   `yaml:"storeDir"`
			PoolName    string   `yaml:"poolName"`
			LogLevel    string   `yaml:"logLevel"`
			ListenAddrs []string `yaml:"listenAddrs"`
			Authorizer  string   `yaml:"authorizer"`

			listenAddrs cli.StringSlice
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
				Value:       "my-pool",
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
				Value:       cli.NewStringSlice("/ip4/0.0.0.0/tcp/40001"),
			}),
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
	cp := ctx.Path("config")
	if cp == "" {
		cp = path.Join(homeDir, ".fula", "blox", "config.yaml")
		if err := ctx.Set("config", cp); err != nil {
			return err
		}
	}
	if stats, err := os.Stat(cp); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path.Dir(cp), 0700); err != nil {
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
	yc, err := yaml.Marshal(app.config)
	if err != nil {
		return err
	}
	return os.WriteFile(cp, yc, 0700)
}

func action(ctx *cli.Context) error {
	authorizer, err := peer.Decode(app.config.Authorizer)
	if err != nil {
		return err
	}
	km, err := base64.StdEncoding.DecodeString(app.config.Identity)
	if err != nil {
		return err
	}
	k, err := crypto.UnmarshalPrivateKey(km)
	if err != nil {
		return err
	}
	h, err := libp2p.New(
		libp2p.Identity(k),
		libp2p.ListenAddrs(),
		libp2p.ListenAddrStrings(app.config.ListenAddrs...))
	if err != nil {
		return err
	}
	ds, err := badger.NewDatastore(app.config.StoreDir, &badger.DefaultOptions)
	if err != nil {
		return err
	}
	bb, err := blox.New(blox.WithHost(h), blox.WithDatastore(ds), blox.WithPoolName(app.config.PoolName), blox.WithAuthorizer(authorizer))
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
