package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/functionland/go-fula/blox"
	"github.com/functionland/go-fula/exchange"
	ipfsCluster "github.com/ipfs-cluster/ipfs-cluster/api/rest/client"
	ipfsPath "github.com/ipfs/boxo/path"
	blockformat "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	badger "github.com/ipfs/go-ds-badger"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/client/rpc"
	oldcmds "github.com/ipfs/kubo/commands"
	config "github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/corehttp"
	iface "github.com/ipfs/kubo/core/coreiface"
	kubolibp2p "github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipni/index-provider/engine"
	goprocess "github.com/jbenet/goprocess"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	sockets "github.com/libp2p/go-socket-activation"
	"github.com/mdp/qrterminal"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/multiformats/go-multicodec"
	bip39 "github.com/tyler-smith/go-bip39"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"gopkg.in/yaml.v3"
)

type DatastoreConfigSpec struct {
	Mounts []Mount `json:"mounts"`
	Type   string  `json:"type"`
}

type Mount struct {
	Child      Child  `json:"child"`
	Mountpoint string `json:"mountpoint"`
	Prefix     string `json:"prefix"`
	Type       string `json:"type"`
}

type Child struct {
	Path        string `json:"path"`
	ShardFunc   string `json:"shardFunc,omitempty"`
	Sync        bool   `json:"sync,omitempty"`
	Type        string `json:"type"`
	Compression string `json:"compression,omitempty"`
}

var (
	logger  = logging.Logger("fula/cmd/blox")
	baseURL = "http://localhost:5001"
	app     struct {
		cli.App
		initOnly             bool
		poolHost             bool
		blockchainEndpoint   string
		secretsPath          string
		generateNodeKey      bool
		generateSecretPhrase bool
		wireless             bool
		configPath           string
		config               struct {
			Identity                  string        `yaml:"identity"`
			StoreDir                  string        `yaml:"storeDir"`
			PoolName                  string        `yaml:"poolName"`
			LogLevel                  string        `yaml:"logLevel"`
			ListenAddrs               []string      `yaml:"listenAddrs"`
			Authorizer                string        `yaml:"authorizer"`
			AuthorizedPeers           []string      `yaml:"authorizedPeers"`
			IpfsBootstrapNodes        []string      `yaml:"ipfsBootstrapNodes"`
			StaticRelays              []string      `yaml:"staticRelays"`
			ForceReachabilityPrivate  bool          `yaml:"forceReachabilityPrivate"`
			AllowTransientConnection  bool          `yaml:"allowTransientConnection"`
			DisableResourceManger     bool          `yaml:"disableResourceManger"`
			MaxCIDPushRate            int           `yaml:"maxCIDPushRate"`
			IpniPublishDisabled       bool          `yaml:"ipniPublishDisabled"`
			IpniPublishInterval       time.Duration `yaml:"ipniPublishInterval"`
			IpniPublishDirectAnnounce []string      `yaml:"IpniPublishDirectAnnounce"`
			IpniPublisherIdentity     string        `yaml:"ipniPublisherIdentity"`

			listenAddrs        cli.StringSlice
			staticRelays       cli.StringSlice
			ipfsBootstrapNodes cli.StringSlice
			directAnnounce     cli.StringSlice
		}
	}
)

func DefaultDatastoreConfig(options *badger.Options, dsPath string, storageMax string) config.Datastore {
	spec := IPFSSpec(dsPath)

	// Marshal the DatastoreConfigSpec to JSON
	specJSON, err := json.Marshal(spec)
	if err != nil {
		panic(err) // Handle the error appropriately
	}

	// Unmarshal JSON into a map[string]interface{}
	var specMap map[string]interface{}
	if err := json.Unmarshal(specJSON, &specMap); err != nil {
		panic(err) // Handle the error appropriately
	}

	return config.Datastore{
		StorageMax:         storageMax,
		StorageGCWatermark: 90, // 90%
		GCPeriod:           "1h",
		BloomFilterSize:    0,
		Spec:               BadgerSpec(options, dsPath),
	}
}
func BadgerSpec(options *badger.Options, dsPath string) map[string]interface{} {
	return map[string]interface{}{
		"type":   "measure",
		"prefix": "badger.datastore",
		"child": map[string]interface{}{
			"type":           "badgerds",
			"path":           dsPath,
			"syncWrites":     options.SyncWrites,
			"truncate":       options.Truncate,
			"gcDiscardRatio": options.GcDiscardRatio,
			"gcSleep":        options.GcSleep.String(),
			"gcInterval":     options.GcInterval.String(),
		},
	}
}
func IPFSSpec(dsPath string) DatastoreConfigSpec {
	return DatastoreConfigSpec{
		Mounts: []Mount{
			{
				Child: Child{
					Path: filepath.Join(dsPath, "blocks"),
					Sync: true,
					Type: "badgerds",
				},
				Mountpoint: "/blocks",
				Prefix:     "badger.blocks",
				Type:       "measure",
			},
			{
				Child: Child{
					Path: filepath.Join(dsPath, "keystore"),
					Sync: true,
					Type: "badgerds",
				},
				Mountpoint: "/keystore",
				Prefix:     "badgerds.keystore",
				Type:       "measure",
			},
			{
				Child: Child{
					Path:        dsPath,
					Type:        "badgerds",
					Compression: "none",
				},
				Mountpoint: "/",
				Prefix:     "badgerds.datastore",
				Type:       "measure",
			},
		},
		Type: "mount",
	}
}
func CreateCustomRepo(ctx context.Context, basePath string, h host.Host, options *badger.Options, dsPath string, storageMax string) (repo.Repo, error) {
	// Path to the repository
	repoPath := basePath

	if !fsrepo.IsInitialized(repoPath) {
		// Create the repository if it doesn't exist

		versionFilePath := filepath.Join(repoPath, "version")
		versionContent := strconv.Itoa(15)
		if err := os.WriteFile(versionFilePath, []byte(versionContent), 0644); err != nil {
			return nil, err
		}
		dataStoreFilePath := filepath.Join(repoPath, "datastore_spec")
		datastoreContent := map[string]interface{}{
			"type": "badgerds",
			"path": dsPath,
		}
		datastoreContentBytes, err := json.Marshal(datastoreContent)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(dataStoreFilePath, []byte(datastoreContentBytes), 0644); err != nil {
			return nil, err
		}
		// Extract private key and peer ID from the libp2p host
		privKey := h.Peerstore().PrivKey(h.ID())
		if privKey == nil {
			return nil, fmt.Errorf("private key for host not found")
		}
		privKeyBytes, err := crypto.MarshalPrivateKey(privKey)
		if err != nil {
			return nil, err
		}

		peerID := h.ID().String()
		conf := config.Config{
			Identity: config.Identity{
				PeerID:  peerID,
				PrivKey: base64.StdEncoding.EncodeToString(privKeyBytes),
			},
		}

		// Set the custom configuration
		conf.Bootstrap = append(conf.Bootstrap, app.config.IpfsBootstrapNodes...)
		logger.Debugw("IPFS Bootstrap nodes added", "bootstrap nodes", conf.Bootstrap)

		conf.Datastore = DefaultDatastoreConfig(options, dsPath, storageMax)

		conf.Addresses.Swarm = app.config.ListenAddrs
		conf.Identity.PeerID = h.ID().String()
		conf.Identity.PrivKey = app.config.Identity
		conf.Swarm.RelayService.Enabled = 1
		conf.Addresses.API = []string{"/ip4/127.0.0.1/tcp/5001"}

		// Initialize the repo with the configuration
		if err := fsrepo.Init(repoPath, &conf); err != nil {
			logger.Error("Error happened in fsrepo.Init")
			return nil, err
		}
	}

	// Open the repo
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		logger.Error("Error happened in fsrepo.Open")
		return nil, err
	}

	return repo, nil
}

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
				Value:       "0",
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
				Value:       cli.NewStringSlice("/ip4/0.0.0.0/tcp/40001", "/ip4/0.0.0.0/udp/40001/quic", "/ip4/0.0.0.0/udp/40001/quic-v1", "/ip4/0.0.0.0/udp/40001/quic-v1/webtransport"),
			}),
			altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
				Name:        "staticRelays",
				Destination: &app.config.staticRelays,
				Value: cli.NewStringSlice(
					"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835",
					//"/dns/alpha-relay.dev.fx.land/tcp/4001/p2p/12D3KooWFLhr8j6LTF7QV1oGCn3DVNTs1eMz2u4KCDX6Hw3BFyag",
					//"/dns/bravo-relay.dev.fx.land/tcp/4001/p2p/12D3KooWA2JrcPi2Z6i2U8H3PLQhLYacx6Uj9MgexEsMsyX6Fno7",
					//"/dns/charlie-relay.dev.fx.land/tcp/4001/p2p/12D3KooWKaK6xRJwjhq6u6yy4Mw2YizyVnKxptoT9yXMn3twgYns",
					//"/dns/delta-relay.dev.fx.land/tcp/4001/p2p/12D3KooWDtA7kecHAGEB8XYEKHBUTt8GsRfMen1yMs7V85vrpMzC",
					//"/dns/echo-relay.dev.fx.land/tcp/4001/p2p/12D3KooWQBigsW1tvGmZQet8t5MLMaQnDJKXAP2JNh7d1shk2fb2",
				),
			}),
			altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
				Name:        "ipfsBootstrapNodes",
				Destination: &app.config.ipfsBootstrapNodes,
				Value: cli.NewStringSlice(
					"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
					"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
					"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
					"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
				),
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
				Name:        "disableResourceManger",
				Aliases:     []string{"drm"},
				Usage:       "Weather to disable the libp2p resource manager.",
				Destination: &app.config.DisableResourceManger,
				Value:       true,
			}),
			altsrc.NewIntFlag(&cli.IntFlag{
				Name:        "maxCIDPushRate",
				Aliases:     []string{"mcidpr"},
				Usage:       "Maximum number of CIDs pushed per second.",
				Destination: &app.config.MaxCIDPushRate,
				Value:       20,
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
			&cli.BoolFlag{
				Name:        "poolHost",
				Usage:       "Weather to run the blox as a pool host (needs public IP)",
				Destination: &app.poolHost,
				Value:       false,
			},
			&cli.BoolFlag{
				Name:        "generateNodeKey",
				Usage:       "Generate node key from identity",
				Destination: &app.generateNodeKey,
			},
			&cli.BoolFlag{
				Name:        "generateSecretPhrase",
				Usage:       "Generate 12-word Secret Phrase from identity",
				Destination: &app.generateSecretPhrase,
			},
			&cli.StringFlag{
				Name:        "blockchainEndpoint",
				Usage:       "Change the blockchain APIs endpoint",
				Destination: &app.blockchainEndpoint,
				Value:       "127.0.0.1:4000",
			},
			&cli.StringFlag{
				Name:        "secretsPath",
				Usage:       "Change the path for storing secret words",
				Destination: &app.secretsPath,
				Value:       "",
			},
		},
		Before:    before,
		Action:    action,
		Copyright: "fx.land",
	}
}

func ConvertBase64PrivateKeyToHexNodeKey(base64PrivKey string) (string, error) {
	// Decode the base64 string to get the private key bytes
	privKeyBytes, err := base64.StdEncoding.DecodeString(base64PrivKey)
	if err != nil {
		return "", err
	}

	// Unmarshal the private key
	privKey, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		return "", err
	}

	// Use the Raw() method to get the raw private key bytes
	rawPrivKey, err := privKey.Raw()
	if err != nil {
		return "", err
	}

	// Extract the seed (first 32 bytes of the raw private key)
	seed := rawPrivKey[:32]

	// Convert the seed bytes to a hexadecimal string
	hexString := hex.EncodeToString(seed)
	return hexString, nil
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
	app.config.IpfsBootstrapNodes = app.config.ipfsBootstrapNodes.Value()
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
	if app.config.PoolName != newPoolName {
		app.config.PoolName = newPoolName

		logger.Infof("Updated pool name to: %s", app.config.PoolName)

		// Marshal the updated config back to YAML
		configData, err = yaml.Marshal(app.config)
		if err != nil {
			return err
		}

		// Write the updated config back to the file
		if err := os.WriteFile(app.configPath, configData, 0760); err != nil {
			return err
		}
	}

	return nil
}

func CustomHostOption(h host.Host) kubolibp2p.HostOption {
	return func(id peer.ID, ps peerstore.Peerstore, options ...libp2p.Option) (host.Host, error) {
		return h, nil
	}
}

// THE TWO BELOW ARE IF YOU CHOOSE internal AS IPFS HOST

// CustomStorageWriteOpener creates a StorageWriteOpener using the IPFS blockstore
func CustomStorageWriteOpenerInternal(ipfsNode *core.IpfsNode) linking.BlockWriteOpener {
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
			logger.Debugw("block was created with cid", "block", block, "cid", c)

			// Store the block using the IPFS node's blockstore.
			err = ipfsNode.Blockstore.Put(context.Background(), block)
			if err != nil {
				logger.Errorw("Error in ipfs store", "block", block, "cid", c)
			}
			return err
		}

		return &buffer, committer, nil
	}
}

func CustomStorageReadOpenerInternal(ipfsApi iface.CoreAPI) ipld.BlockReadOpener {
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

// END OF INTERNAL

func CustomStorageReadOpenerNone(ctx ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
	cidStr := lnk.String()
	url := fmt.Sprintf("%s/api/v0/block/get?arg=%s", baseURL, cidStr)

	// Making a POST request, assuming the IPFS node accepts POST for this endpoint.
	// Adjust if your node's configuration or version requires otherwise.
	req, err := http.NewRequestWithContext(ctx.Ctx, "POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request failed: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close() // Ensure we don't leak resources
		return nil, fmt.Errorf("failed to get block: CID %s, HTTP status %d", cidStr, resp.StatusCode)
	}

	return resp.Body, nil
}

// CustomWriter captures written data and allows it to be retrieved.
type CustomWriter struct {
	Buffer *bytes.Buffer
}

// NewCustomWriter creates a new CustomWriter instance.
func NewCustomWriter() *CustomWriter {
	return &CustomWriter{
		Buffer: new(bytes.Buffer),
	}
}

// Write captures data written to the writer.
func (cw *CustomWriter) Write(p []byte) (n int, err error) {
	return cw.Buffer.Write(p)
}

// CustomStorageWriteOpener opens a block for writing to an external IPFS node via HTTP API.
func CustomStorageWriteOpenerNone(lctx linking.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
	cw := NewCustomWriter()

	committer := func(lnk ipld.Link) error {
		cidLink, ok := lnk.(cidlink.Link)
		if !ok {
			logger.Errorf("link is not a CID link")
			return fmt.Errorf("link is not a CID link")
		}

		c := cidLink.Cid
		cidCodec := c.Type()
		mhType := c.Prefix().MhType

		pin := !app.poolHost
		pinValue := strconv.FormatBool(pin)

		// Dynamically construct the URL with the cid-codec and mhtype
		url := fmt.Sprintf("%s/api/v0/block/put?cid-codec=%s&mhtype=%s&mhlen=-1&pin=%s&allow-big-block=false", baseURL, multicodec.Code(cidCodec), multicodec.Code(mhType), pinValue)

		formData := &bytes.Buffer{}
		writer := multipart.NewWriter(formData)
		part, err := writer.CreateFormFile("file", "block.data")
		if err != nil {
			return err
		}

		if _, err := part.Write(cw.Buffer.Bytes()); err != nil {
			return err
		}

		if err := writer.Close(); err != nil {
			return err
		}

		req, reqErr := http.NewRequestWithContext(lctx.Ctx, "POST", url, formData)
		if reqErr != nil {
			logger.Errorf("Failed to NewRequestWithContext: %s", reqErr)
			return reqErr
		}

		// Set the Content-Type header to multipart/form-data with the boundary parameter
		req.Header.Set("Content-Type", writer.FormDataContentType())

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP request failed: status %d", resp.StatusCode)
		}

		// Optionally read response to get the CID from the response if necessary
		var respData struct {
			Key  string `json:"Key"`
			Size int    `json:"Size"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
			logger.Errorf("Failed to NewDecoder: %s", err)
			return err
		}

		// Verify the CID matches if necessary
		if respData.Key != c.String() {
			return fmt.Errorf("mismatched CIDs: expected %s, got %s", c.String(), respData.Key)
		}
		logger.Debugw("Write cid finished", "cid", c)
		return nil
	}

	return cw, committer, nil
}

// defaultMux tells mux to serve path using the default muxer. This is
// mostly useful to hook up things that register in the default muxer,
// and don't provide a convenient http.Handler entry point, such as
// expvar and http/pprof.
func defaultMux(path string) corehttp.ServeOption {
	return func(node *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.Handle(path, http.DefaultServeMux)
		return mux, nil
	}
} // serveHTTPApi collects options, creates listener, prints status message and starts serving requests.
func rewriteMaddrToUseLocalhostIfItsAny(maddr ma.Multiaddr) ma.Multiaddr {
	first, rest := ma.SplitFirst(maddr)

	switch {
	case first.Equal(manet.IP4Unspecified):
		return manet.IP4Loopback.Encapsulate(rest)
	case first.Equal(manet.IP6Unspecified):
		return manet.IP6Loopback.Encapsulate(rest)
	default:
		return maddr // not ip
	}
}
func serveHTTPApi(cctx *oldcmds.Context) (<-chan error, error) {
	cfg, err := cctx.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: GetConfig() failed: %s", err)
	}

	listeners, err := sockets.TakeListeners("io.ipfs.api")
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: socket activation failed: %s", err)
	}

	apiAddrs := make([]string, 0, 2)
	apiAddr := ""
	if apiAddr == "" {
		apiAddrs = cfg.Addresses.API
	} else {
		apiAddrs = append(apiAddrs, apiAddr)
	}

	listenerAddrs := make(map[string]bool, len(listeners))
	for _, listener := range listeners {
		listenerAddrs[string(listener.Multiaddr().Bytes())] = true
	}

	for _, addr := range apiAddrs {
		apiMaddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPApi: invalid API address: %q (err: %s)", addr, err)
		}
		if listenerAddrs[string(apiMaddr.Bytes())] {
			continue
		}

		apiLis, err := manet.Listen(apiMaddr)
		if err != nil {
			return nil, fmt.Errorf("serveHTTPApi: manet.Listen(%s) failed: %s", apiMaddr, err)
		}

		listenerAddrs[string(apiMaddr.Bytes())] = true
		listeners = append(listeners, apiLis)
	}

	if len(cfg.API.Authorizations) > 0 && len(listeners) > 0 {
		fmt.Printf("RPC API access is limited by the rules defined in API.Authorizations\n")
	}

	for _, listener := range listeners {
		// we might have listened to /tcp/0 - let's see what we are listing on
		fmt.Printf("RPC API server listening on %s\n", listener.Multiaddr())
		// Browsers require TCP.
		switch listener.Addr().Network() {
		case "tcp", "tcp4", "tcp6":
			fmt.Printf("WebUI: http://%s/webui\n", listener.Addr())
		}
	}

	// by default, we don't let you load arbitrary ipfs objects through the api,
	// because this would open up the api to scripting vulnerabilities.
	// only the webui objects are allowed.
	// if you know what you're doing, go ahead and pass --unrestricted-api.
	unrestricted := false
	gatewayOpt := corehttp.GatewayOption(corehttp.WebUIPaths...)
	if unrestricted {
		gatewayOpt = corehttp.GatewayOption("/ipfs", "/ipns")
	}

	opts := []corehttp.ServeOption{
		corehttp.MetricsCollectionOption("api"),
		corehttp.MetricsOpenCensusCollectionOption(),
		corehttp.MetricsOpenCensusDefaultPrometheusRegistry(),
		corehttp.CheckVersionOption(),
		corehttp.CommandsOption(*cctx),
		corehttp.WebUIOption,
		gatewayOpt,
		corehttp.VersionOption(),
		defaultMux("/debug/vars"),
		defaultMux("/debug/pprof/"),
		defaultMux("/debug/stack"),
		corehttp.MutexFractionOption("/debug/pprof-mutex/"),
		corehttp.BlockProfileRateOption("/debug/pprof-block/"),
		corehttp.MetricsScrapingOption("/debug/metrics/prometheus"),
		corehttp.LogOption(),
	}

	if len(cfg.Gateway.RootRedirect) > 0 {
		opts = append(opts, corehttp.RedirectOption("", cfg.Gateway.RootRedirect))
	}

	node, err := cctx.ConstructNode()
	if err != nil {
		return nil, fmt.Errorf("serveHTTPApi: ConstructNode() failed: %s", err)
	}

	if len(listeners) > 0 {
		// Only add an api file if the API is running.
		if err := node.Repo.SetAPIAddr(rewriteMaddrToUseLocalhostIfItsAny(listeners[0].Multiaddr())); err != nil {
			return nil, fmt.Errorf("serveHTTPApi: SetAPIAddr() failed: %w", err)
		}
	}

	errc := make(chan error)
	var wg sync.WaitGroup
	for _, apiLis := range listeners {
		wg.Add(1)
		go func(lis manet.Listener) {
			defer wg.Done()
			errc <- corehttp.Serve(node, manet.NetListener(lis), opts...)
		}(apiLis)
	}

	go func() {
		wg.Wait()
		close(errc)
	}()

	return errc, nil
}

func action(ctx *cli.Context) error {
	var wg sync.WaitGroup
	ctx2, cancel := context.WithCancel(context.Background())
	defer cancel()

	if app.generateNodeKey {
		// Execute ConvertBase64PrivateKeyToHexNodeKey with the identity as input
		nodeKey, err := ConvertBase64PrivateKeyToHexNodeKey(app.config.Identity)
		if err != nil {
			return fmt.Errorf("error converting node key: %v", err)
		}
		fmt.Print(nodeKey)
		return nil // Exit after generating the node key
	}
	if app.generateSecretPhrase {
		// Execute ConvertBase64PrivateKeyToHexNodeKey with the identity as input
		hash := sha256.Sum256([]byte(app.config.Identity))

		// Convert the first 128 bits of the hash to entropy for a 12-word mnemonic.
		// BIP39 expects entropy to be a multiple of 32 bits, so we use the first 16 bytes of the hash.
		entropy := hash[:16]

		// Generate the mnemonic from the entropy
		mnemonic, err := bip39.NewMnemonic(entropy)
		if err != nil {
			return fmt.Errorf("error NewMnemonic: %v", err)
		}
		fmt.Print(mnemonic)
		return nil // Exit after generating the node key
	}
	authorizer, err := peer.Decode(app.config.Authorizer)
	if err != nil {
		return err
	}
	if app.poolHost {
		authorizer = ""
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

	listenAddrs := make([]ma.Multiaddr, 0, len(app.config.ListenAddrs)+1)
	// Convert string addresses to multiaddr and append to listenAddrs
	for _, addrString := range app.config.ListenAddrs {
		addr, err := ma.NewMultiaddr(addrString)
		if err != nil {
			panic(fmt.Errorf("invalid multiaddress: %w", err))
		}
		listenAddrs = append(listenAddrs, addr)
	}
	// Add the relay multiaddress
	relayAddr2, err := ma.NewMultiaddr("/p2p-circuit")
	if err != nil {
		panic(fmt.Errorf("error creating relay multiaddress: %w", err))
	}
	listenAddrs = append(listenAddrs, relayAddr2)

	hopts := []libp2p.Option{
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
	}

	if app.config.ForceReachabilityPrivate {
		hopts = append(hopts, libp2p.ForceReachabilityPrivate())
	}
	if app.config.DisableResourceManger {
		hopts = append(hopts, libp2p.ResourceManager(&network.NullResourceManager{}))
	}

	sr := make([]peer.AddrInfo, 0, len(app.config.StaticRelays))
	for _, relay := range app.config.StaticRelays {
		rma, err := ma.NewMultiaddr(relay)
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
		hopts = append(hopts, libp2p.EnableAutoRelayWithStaticRelays(sr,
			autorelay.WithMinCandidates(1),
			autorelay.WithNumRelays(1),
			autorelay.WithBootDelay(30*time.Second),
			autorelay.WithMinInterval(10*time.Second),
		))
	}

	bopts := append(hopts, libp2p.Identity(k), libp2p.ListenAddrs(listenAddrs...))
	h, err := libp2p.New(bopts...)
	if err != nil {
		return err
	}

	// Create a new slice for updated listen addresses
	ipniListenAddrs := make([]ma.Multiaddr, len(listenAddrs))
	// Update each address in the slice
	for i, addr := range listenAddrs {
		// Convert the multiaddr to a string for manipulation
		addrStr := addr.String()

		// Replace "40001" with "40002" in the string representation
		updatedAddrStr := strings.Replace(addrStr, "40001", "40002", -1)

		// Parse the updated string back into a multiaddr.Multiaddr
		updatedAddr, err := ma.NewMultiaddr(updatedAddrStr)
		if err != nil {
			log.Fatalf("Failed to parse updated multiaddr '%s': %v", updatedAddrStr, err)
		}

		// Store the updated multiaddr in the slice
		ipniListenAddrs[i] = updatedAddr
	}

	iopts := append(hopts, libp2p.Identity(ipnik), libp2p.ListenAddrs(ipniListenAddrs...))
	ipnih, err := libp2p.New(iopts...)
	if err != nil {
		return err
	}
	ds, err := badger.NewDatastore(app.config.StoreDir, &badger.DefaultOptions)
	if err != nil {
		return err
	}

	dirPath := filepath.Dir(app.configPath)
	linkSystem := cidlink.DefaultLinkSystem()
	linkSystem.StorageReadOpener = CustomStorageReadOpenerNone
	linkSystem.StorageWriteOpener = CustomStorageWriteOpenerNone
	nodeMultiAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5001")
	if err != nil {
		panic(fmt.Errorf("invalid multiaddress: %w", err))
	}
	node, err := rpc.NewApi(nodeMultiAddr)
	if err != nil {
		logger.Fatal(err)
		return err
	}

	const useIPFSServer = "none" //internal: runs local ipfs instance, none requires an external one and fula runs the mock server on 5001
	if useIPFSServer == "internal" {
		repo, err := CreateCustomRepo(ctx2, dirPath, h, &badger.DefaultOptions, app.config.StoreDir, "90%")
		if err != nil {
			logger.Fatal(err)
			return err
		}

		ipfsConfig := &core.BuildCfg{
			Online:    true,
			Permanent: true,
			Host:      CustomHostOption(h),
			Routing:   kubolibp2p.DHTOption,
			Repo:      repo,
		}

		ipfsNode, err := core.NewNode(ctx2, ipfsConfig)
		if err != nil {
			logger.Fatal(err)
			return err
		}
		ipfsHostId := ipfsNode.PeerHost.ID()
		ipfsId := ipfsNode.Identity.String()
		logger.Infow("ipfscore successfully instantiated", "host", ipfsHostId, "peer", ipfsId)
		ipfsAPI, err := coreapi.NewCoreAPI(ipfsNode)
		if err != nil {
			panic(fmt.Errorf("failed to create IPFS API: %w", err))
		}
		if ipfsAPI != nil {
			logger.Info("ipfscoreapi successfully instantiated")
		}

		ipfsNode.IsDaemon = true
		logger.Debug("called wg.Add in blox start")
		logger.Debugw("ipfs started", "condifg path", dirPath)

		cctx := oldcmds.Context{
			ConfigRoot:    dirPath,
			ConstructNode: func() (*core.IpfsNode, error) { return ipfsNode, nil },
			Gateway:       false,
			// Other necessary fields...
		}

		err = cctx.Plugins.Start(ipfsNode)
		if err != nil {
			return err
		}
		ipfsNode.Process.AddChild(goprocess.WithTeardown(cctx.Plugins.Close))

		wg.Add(1)
		go func() {
			logger.Debug("called wg.Done in Start")
			defer wg.Done()
			defer logger.Debug("Start go routine is ending")
			defer func() {
				// We wait for the node to close first, as the node has children
				// that it will wait for before closing, such as the API server.
				ipfsNode.Close()

				select {
				case <-ctx2.Done():
					logger.Info("Gracefully shut down daemon")
				default:
				}
			}()
			if ipfsNode.IsOnline {
				logger.Infow("IPFS RPC server started on address http://127.0.0.1:5001")

				/*err = corehttp.ListenAndServe(ipfsNode, "/ip4/127.0.0.1/tcp/5001", opts...)
				if err != nil {
					fmt.Println("Error setting up HTTP API server:", err)
				}*/
				err = cctx.Plugins.Start(ipfsNode)
				if err != nil {
					logger.Errorw("Error in Plugins Start", "err", err)
				}
				ipfsNode.Process.AddChild(goprocess.WithTeardown(cctx.Plugins.Close))
				_, err := serveHTTPApi(&cctx)
				if err != nil {
					logger.Errorw("Error setting up HTTP API server:", "err", err)
				}
			}
			select {}
		}()
		//You need to correct the below as well. This commit was working https://github.com/functionland/go-fula/commit/34e39f76ec8f7e8d2175283eff9c2725c98ec059#diff-b1997e94b91855c9f21a9d9e5fb5aba0fc5ac30765046e3c5972aad2cc5cbcd3
		//ds = ipfsNode.Repo.Datastore()
		// linkSystem.StorageReadOpener = CustomStorageReadOpenerInternal
		// linkSystem.StorageWriteOpener = CustomStorageWriteOpenerInternal
	}

	ipfsClusterConfig := ipfsCluster.Config{}
	ipfsClusterApi, err := ipfsCluster.NewDefaultClient(&ipfsClusterConfig)
	if err != nil {
		logger.Errorw("Error in setting ipfs cluster api", "err", err)
	}

	bb, err := blox.New(
		blox.WithHost(h),
		blox.WithWg(&wg),
		blox.WithDatastore(ds),
		blox.WithLinkSystem(&linkSystem),
		blox.WithPoolName(app.config.PoolName),
		blox.WithTopicName(app.config.PoolName),
		blox.WithStoreDir(app.config.StoreDir),
		blox.WithRelays(app.config.StaticRelays),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithBlockchainEndPoint(app.blockchainEndpoint),
		blox.WithSecretsPath(app.secretsPath),
		blox.WithPingCount(5),
		blox.WithDefaultIPFShttpServer(useIPFSServer),
		blox.WithIpfsClient(node),
		blox.WithPoolHostMode(app.poolHost),
		blox.WithIpfsClusterAPI(ipfsClusterApi),
		blox.WithExchangeOpts(
			exchange.WithUpdateConfig(updateConfig),
			exchange.WithWg(&wg),
			//exchange.WithIPFSApi(ipfsAPI),
			exchange.WithAuthorizer(authorizer),
			exchange.WithAuthorizedPeers(authorizedPeers),
			exchange.WithAllowTransientConnection(app.config.AllowTransientConnection),
			exchange.WithMaxPushRate(app.config.MaxCIDPushRate),
			exchange.WithIpniPublishDisabled(app.config.IpniPublishDisabled),
			exchange.WithIpniPublishInterval(app.config.IpniPublishInterval),
			exchange.WithIpniGetEndPoint("https://cid.contact/cid/"),
			exchange.WithPoolHostMode(app.poolHost),
			exchange.WithIpniProviderEngineOptions(
				engine.WithHost(ipnih),
				engine.WithDatastore(namespace.Wrap(ds, datastore.NewKey("ipni/ads"))),
				engine.WithPublisherKind(engine.DataTransferPublisher),
				engine.WithDirectAnnounce(app.config.IpniPublishDirectAnnounce...),
			),
			/*exchange.WithDhtProviderOptions(
				//dht.Datastore(namespace.Wrap(ds, datastore.NewKey("dht"))),
				dht.ProtocolExtension(protocol.ID("/"+app.config.PoolName)),
				dht.ProtocolPrefix("/fula"),
				dht.Resiliency(1),
				dht.Mode(dht.ModeAutoServer),
			),*/
		),
	)
	if err != nil {
		return err
	}

	if err := bb.Start(ctx2); err != nil {
		return err
	}

	logger.Info("Started blox", "addrs", h.Addrs())
	////printMultiaddrAsQR(h)

	// Setup signal handling
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		logger.Info("Signal received, initiating shutdown...")
		if err := bb.Shutdown(ctx2); err != nil {
			logger.Error("Error during blox shutdown:", err)
		}
		cancel() // Trigger context cancellation
	}()

	<-ctx2.Done()
	wg.Wait()
	return nil
}

func printMultiaddrAsQR(h host.Host) {
	var addr ma.Multiaddr
	addrs := h.Addrs()

	switch len(addrs) {
	case 0:
		// No addresses to process
		return
	case 1:
		addr = addrs[0]
	default:
		// Select the best address to use
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
		fmt.Println("blox has no multiaddrs")
		return
	}

	fullAddr := fmt.Sprintf("%s/p2p/%s", addr.String(), h.ID().String())
	fmt.Printf(">>> blox multiaddr: %s\n", fullAddr)

	// Configure QR code generation based on OS
	config := qrterminal.Config{
		Level:     qrterminal.M,
		Writer:    os.Stdout,
		QuietZone: qrterminal.QUIET_ZONE,
	}

	// Characters for other terminals (e.g., Unix/Linux)
	config.BlackChar = qrterminal.BLACK
	config.WhiteChar = qrterminal.WHITE

	// Generate QR code
	qrterminal.GenerateWithConfig(fullAddr, config)
}

func main() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
