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
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/functionland/go-fula/blox"
	"github.com/functionland/go-fula/exchange"
	ipfsPath "github.com/ipfs/boxo/path"
	blockformat "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	badger "github.com/ipfs/go-ds-badger"
	logging "github.com/ipfs/go-log/v2"
	config "github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	coreapi "github.com/ipfs/kubo/core/coreapi"
	iface "github.com/ipfs/kubo/core/coreiface"
	kubolibp2p "github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
	ipld "github.com/ipld/go-ipld-prime"
	linking "github.com/ipld/go-ipld-prime/linking"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipni/index-provider/engine"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/mdp/qrterminal"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
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
	logger = logging.Logger("fula/cmd/blox")
	app    struct {
		cli.App
		initOnly             bool
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
			StaticRelays              []string      `yaml:"staticRelays"`
			ForceReachabilityPrivate  bool          `yaml:"forceReachabilityPrivate"`
			AllowTransientConnection  bool          `yaml:"allowTransientConnection"`
			DisableResourceManger     bool          `yaml:"disableResourceManger"`
			MaxCIDPushRate            int           `yaml:"maxCIDPushRate"`
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
		conf.Bootstrap = append(conf.Bootstrap, app.config.StaticRelays...)

		conf.Datastore = DefaultDatastoreConfig(options, dsPath, storageMax)

		conf.Addresses.Swarm = app.config.ListenAddrs
		conf.Identity.PeerID = h.ID().String()
		conf.Identity.PrivKey = app.config.Identity
		conf.Swarm.RelayService.Enabled = 1
		conf.Addresses.API = []string{"/ip4/127.0.0.1/tcp/5001"}

		// Initialize the repo with the configuration
		if err := fsrepo.Init(repoPath, &conf); err != nil {
			return nil, err
		}
	}
	plugins, err := loader.NewPluginLoader(repoPath)
	if err != nil {
		panic(fmt.Errorf("error loading plugins: %s", err))
	}

	if err := plugins.Initialize(); err != nil {
		panic(fmt.Errorf("error initializing plugins: %s", err))
	}

	if err := plugins.Inject(); err != nil {
		panic(fmt.Errorf("error initializing plugins: %s", err))
	}

	// Open the repo
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
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
				Value:       100,
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

// CustomStorageWriteOpener creates a StorageWriteOpener using the IPFS blockstore
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

	listenAddrs := make([]multiaddr.Multiaddr, 0, len(app.config.ListenAddrs)+1)
	// Convert string addresses to multiaddr and append to listenAddrs
	for _, addrString := range app.config.ListenAddrs {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			panic(fmt.Errorf("invalid multiaddress: %w", err))
		}
		listenAddrs = append(listenAddrs, addr)
	}
	// Add the relay multiaddress
	relayAddr2, err := multiaddr.NewMultiaddr("/p2p-circuit")
	if err != nil {
		panic(fmt.Errorf("error creating relay multiaddress: %w", err))
	}
	listenAddrs = append(listenAddrs, relayAddr2)

	hopts := []libp2p.Option{
		libp2p.ListenAddrs(listenAddrs...),
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
		hopts = append(hopts, libp2p.EnableAutoRelayWithStaticRelays(sr,
			autorelay.WithMinCandidates(1),
			autorelay.WithNumRelays(1),
			autorelay.WithBootDelay(30*time.Second),
			autorelay.WithMinInterval(10*time.Second),
		))
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
	/*ds, err := badger.NewDatastore(app.config.StoreDir, &badger.DefaultOptions)
	if err != nil {
		return err
	}*/

	dirPath := filepath.Dir(app.configPath)
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

	/*logger.Debug("called wg.Add in blox start")
	opts := []corehttp.ServeOption{
		// Add necessary handlers, CORS, etc.
		corehttp.GatewayOption("127.0.0.1:5001"),
		corehttp.CheckVersionOption(),
		corehttp.HostnameOption(),
		corehttp.RoutingOption(),
		corehttp.LogOption(),
		corehttp.Libp2pGatewayOption(),
	}

	wg.Add(1)
	go func() {
		logger.Debug("called wg.Done in Start")
		defer wg.Done()
		defer logger.Debug("Start go routine is ending")
		if ipfsNode.IsOnline {
			logger.Infow("IPFS RPC server started on address http://127.0.0.1:5001")
			err = corehttp.ListenAndServe(ipfsNode, "/ip4/127.0.0.1/tcp/5001", opts...)
			if err != nil {
				fmt.Println("Error setting up HTTP API server:", err)
			}
		}
		select {}
	}()*/

	ds := ipfsNode.Repo.Datastore()
	linkSystem := cidlink.DefaultLinkSystem()
	linkSystem.StorageReadOpener = CustomStorageReadOpener(ipfsAPI)
	linkSystem.StorageWriteOpener = CustomStorageWriteOpener(ipfsNode)

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
		blox.WithExchangeOpts(
			exchange.WithUpdateConfig(updateConfig),
			exchange.WithWg(&wg),
			exchange.WithIPFSApi(ipfsAPI),
			exchange.WithAuthorizer(authorizer),
			exchange.WithAuthorizedPeers(authorizedPeers),
			exchange.WithAllowTransientConnection(app.config.AllowTransientConnection),
			exchange.WithMaxPushRate(app.config.MaxCIDPushRate),
			exchange.WithIpniPublishDisabled(app.config.IpniPublishDisabled),
			exchange.WithIpniPublishInterval(app.config.IpniPublishInterval),
			exchange.WithIpniGetEndPoint("https://cid.contact/cid/"),
			exchange.WithIpniProviderEngineOptions(
				engine.WithHost(ipnih),
				engine.WithDatastore(namespace.Wrap(ds, datastore.NewKey("ipni/ads"))),
				engine.WithPublisherKind(engine.DataTransferPublisher),
				engine.WithDirectAnnounce(app.config.IpniPublishDirectAnnounce...),
			),
			exchange.WithDhtProviderOptions(
				dht.Datastore(namespace.Wrap(ds, datastore.NewKey("dht"))),
				dht.ProtocolExtension(protocol.ID("/"+app.config.PoolName)),
				dht.ProtocolPrefix("/fula"),
				dht.Resiliency(1),
				dht.Mode(dht.ModeAutoServer),
			),
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
	var addr multiaddr.Multiaddr
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
