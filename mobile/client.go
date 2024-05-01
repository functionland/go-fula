package fulamobile

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/exchange"
	bsfetcher "github.com/ipfs/boxo/fetcher/impl/blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	badger "github.com/ipfs/go-ds-badger"
	config "github.com/ipfs/kubo/config"
	core "github.com/ipfs/kubo/core"
	iface "github.com/ipfs/kubo/core/coreiface"
	kubolibp2p "github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader"
	"github.com/ipfs/kubo/repo"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	_ "github.com/ipld/go-ipld-prime/codec/dagjson"
	_ "github.com/ipld/go-ipld-prime/codec/raw"
	"github.com/ipld/go-ipld-prime/datamodel"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	ipldmc "github.com/ipld/go-ipld-prime/multicodec"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal"
	"github.com/ipld/go-ipld-prime/traversal/selector"
	"github.com/ipld/go-ipld-prime/traversal/selector/builder"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/mr-tron/base58"
	"github.com/multiformats/go-multicodec"
)

// Note to self; copied from gomobile docs:
// All exported symbols in the package must have types that are supported. Supported types include:
//  * Signed integer and floating point types.
//  * String and boolean types.
//  * Byte slice types.
//    * Note that byte slices are passed by reference and support mutation.
//  * Any function type all of whose parameters and results have supported types.
//    * Functions must return either no results, one result, or two results where the type of the second is the built-in 'error' type.
//  * Any interface type, all of whose exported methods have supported function types.
//  * Any struct type, all of whose exported methods have supported function types and all of whose exported fields have supported types.

var rootDatastoreKey = datastore.NewKey("/")
var exploreAllRecursivelySelector selector.Selector

type Client struct {
	h        host.Host
	ds       datastore.Batching
	ls       ipld.LinkSystem
	ex       exchange.Exchange
	bl       blockchain.Blockchain
	bloxPid  peer.ID
	relays   []string
	ipfsAPI  iface.CoreAPI
	ipfsNode *core.IpfsNode
	ps       *pubsub.PubSub
	topic    *pubsub.Topic
	sub      *pubsub.Subscription
}

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

func CustomHostOption(h host.Host) kubolibp2p.HostOption {
	return func(id peer.ID, ps peerstore.Peerstore, options ...libp2p.Option) (host.Host, error) {
		return h, nil
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

func CreateCustomRepo(ctx context.Context, cfg *Config, basePath string, h host.Host, options *badger.Options, dsPath string, storageMax string, refresh bool) (repo.Repo, error) {
	// Path to the repository
	log.Print("CreateCustomRepo started")
	repoPath := basePath

	if !fsrepo.IsInitialized(repoPath) || refresh {
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
		log.Print("customRepo identity set")

		// Set the custom configuration
		conf.Bootstrap = append(conf.Bootstrap, conf.Bootstrap...)

		conf.Datastore = DefaultDatastoreConfig(options, dsPath, storageMax)

		conf.Addresses.Swarm = []string{
			"/ip4/0.0.0.0/tcp/4001",
			"/ip6/::/tcp/4001",
			"/ip4/0.0.0.0/udp/4001/quic-v1",
			"/ip4/0.0.0.0/udp/4001/quic-v1/webtransport",
			"/ip6/::/udp/4001/quic-v1",
			"/ip6/::/udp/4001/quic-v1/webtransport",
		}
		conf.Swarm.RelayService.Enabled = 1
		conf.Addresses.API = []string{"/ip4/127.0.0.1/tcp/5001"}
		conf.Bootstrap = []string{
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
			"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
			"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
			"/ip4/104.131.131.82/udp/4001/quic-v1/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
			"/dns4/1.pools.functionyard.fula.network/tcp/9096/p2p/12D3KooWS79EhkPU7ESUwgG4vyHHzW9FDNZLoWVth9b5N5NSrvaj",
		}
		conf.Swarm.RelayService.Enabled = 1
		conf.Discovery.MDNS.Enabled = true
		conf.Pubsub.Enabled = 1

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
	log.Println(repoPath)
	log.Println(repo.GetConfigKey("Identity"))

	return repo, nil
}

func NewIpfsClient(ctx context.Context, cfg *Config) (*Client, error) {
	var mc Client
	if err := cfg.initIpfs(ctx, &mc); err != nil {
		return nil, err
	}
	return &mc, nil
}
func NewClient(cfg *Config) (*Client, error) {
	var mc Client
	if err := cfg.init(&mc); err != nil {
		return nil, err
	}
	return &mc, nil
}

// ConnectToBlox attempts to connect to blox via the configured address. This function can be used
// to check if blox is currently accessible.
func (c *Client) ConnectToBlox() error {
	if _, ok := c.ex.(exchange.NoopExchange); ok {
		return nil
	}
	return c.h.Connect(context.TODO(), c.h.Peerstore().PeerInfo(c.bloxPid))
}

func (c *Client) ConnectToBloxIpfs() error {
	ctx := context.TODO()
	if _, ok := c.ex.(exchange.NoopExchange); ok {
		return nil
	}

	err := c.ipfsAPI.Swarm().Connect(ctx, c.h.Peerstore().PeerInfo(c.bloxPid))
	return err

}

// ID returns the libp2p peer ID of the client.
func (c *Client) ID() string {
	return c.h.ID().String()
}

// Get gets the value corresponding to the given key from the local ipld.LinkSystem
// The key must be a valid ipld.Link and the value returned is encoded ipld.Node.
// If data is not found locally, an attempt is made to automatically fetch the data
// from blox at Config.BloxAddr address.
func (c *Client) Get(key []byte) ([]byte, error) {
	l, err := toLink(key)
	if err != nil {
		return nil, err
	}
	ctx := context.TODO()
	node, err := c.ls.Load(ipld.LinkContext{Ctx: ctx}, l, basicnode.Prototype.Any)
	if err != nil {
		return nil, err
	}
	encoder, err := ipldmc.LookupEncoder(l.Cid.Prefix().GetCodec())
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := encoder(node, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Has checks whether the value corresponding to the given key is present in the local datastore.
// The key must be a valid ipld.Link.
func (c *Client) Has(key []byte) (bool, error) {
	link, err := toLink(key)
	if err != nil {
		return false, err
	}
	return c.hasLink(link)
}

func (c *Client) hasLink(l ipld.Link) (bool, error) {
	return c.ds.Has(context.Background(), datastore.NewKey(l.Binary()))
}

func toLink(key []byte) (cidlink.Link, error) {
	_, cc, err := cid.CidFromBytes(key)
	if err != nil {
		return cidlink.Link{}, err
	}
	return cidlink.Link{Cid: cc}, nil
}

func toLinkFromString(l string) (cidlink.Link, error) {
	decodedBytes, err := base58.Decode(l)
	if err != nil {
		return cidlink.Link{}, err
	}
	return toLink(decodedBytes)
}

// Pull downloads the data corresponding to the given key from blox at Config.BloxAddr.
// The key must be a valid ipld.Link.
func (c *Client) Pull(key []byte) error {
	l, err := toLink(key)
	if err != nil {
		return err
	}
	if exists, err := c.hasLink(l); err != nil {
		return err
	} else if exists {
		return nil
	}
	err = c.ex.Pull(context.TODO(), c.bloxPid, l)
	if err != nil {
		providers, err := c.ex.FindProvidersIpni(l, c.relays)
		if err != nil {
			return err
		}
		for _, provider := range providers {
			if provider.ID != c.h.ID() {
				//Found a storer, now pull the cid
				err = c.ex.Pull(context.TODO(), provider.ID, l)
				if err != nil {
					continue
				}
				return nil
			} else {
				continue
			}
		}
	}
	return err
}

// Push requests blox at Config.BloxAddr to download the given key from this node.
// The key must be a valid ipld.Link, and the addr must be a valid multiaddr that includes peer ID.
// The value corresponding to the given key must be stored in the local datastore prior to calling
// this function.
// See: Client.Put.
func (c *Client) Push(key []byte) error {
	l, err := toLink(key)
	if err != nil {
		return err
	}
	return c.pushLink(context.TODO(), l)
}

func (c *Client) pushLink(ctx context.Context, l ipld.Link) error {
	if exists, err := c.hasLink(l); err != nil {
		return err
	} else if !exists {
		return errors.New("value not found locally")
	}
	if err := c.ex.Push(ctx, c.bloxPid, l); err != nil {
		return err
	}
	return c.markAsPushedSuccessfully(ctx, l)
}

// Put stores the given value onto the ipld.LinkSystem and returns its corresponding link.
// The value is decoded using the decoder that corresponds to the given codec. Therefore,
// the given value must be a valid ipld.Node.
// Upon successful local storage of the given value, it is automatically pushed to the blox
// at Config.BloxAddr address.
func (c *Client) Put(value []byte, codec int64) ([]byte, error) {
	ctx := context.TODO()
	ucodec := uint64(codec)
	decode, err := ipldmc.LookupDecoder(ucodec)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(value)
	nb := basicnode.Prototype.Any.NewBuilder()
	if err := decode(nb, buf); err != nil {
		return nil, err
	}
	node := nb.Build()
	link, err := c.ls.Store(ipld.LinkContext{Ctx: ctx},
		cidlink.LinkPrototype{
			Prefix: cid.Prefix{
				Version:  1,
				Codec:    ucodec,
				MhType:   uint64(multicodec.Blake3),
				MhLength: -1,
			},
		},
		node)
	if err != nil {
		return nil, err
	}
	if err := c.markAsRecentCid(ctx, link); err != nil {
		return nil, err
	}
	return link.(cidlink.Link).Cid.Bytes(), nil
}

func (c *Client) ListFailedPushes() (*LinkIterator, error) {
	links, err := c.listFailedPushes(context.TODO())
	if err != nil {
		return nil, err
	}
	return &LinkIterator{links: links}, nil
}

func (c *Client) ListFailedPushesAsString() (*StringIterator, error) {
	links, err := c.listFailedPushesAsString(context.TODO())
	if err != nil {
		return nil, err
	}
	return &StringIterator{links: links}, nil
}

func (c *Client) ListRecentCidsAsString() (*StringIterator, error) {
	links, err := c.listRecentCidsAsString(context.TODO())
	if err != nil {
		return nil, err
	}
	return &StringIterator{links: links}, nil
}

func (c *Client) ListRecentCidsAsStringWithChildren() (*StringIterator, error) {
	ctx := context.TODO()
	recentLinks, err := c.listRecentCids(ctx)
	if err != nil {
		return nil, err
	}

	ssb := builder.NewSelectorSpecBuilder(basicnode.Prototype.Any)
	ss := ssb.ExploreRecursive(selector.RecursionLimitNone(), ssb.ExploreUnion(
		ssb.Matcher(),
		ssb.ExploreAll(ssb.ExploreRecursiveEdge()),
	))
	exploreAllRecursivelySelector, err := ss.Selector()
	if err != nil {
		return nil, fmt.Errorf("failed to parse IPLD built-in selector: %v", err)
	}

	cidSet := make(map[string]struct{}) // Use a map to track unique CIDs
	for _, link := range recentLinks {
		node, err := c.ls.Load(ipld.LinkContext{Ctx: ctx}, link, basicnode.Prototype.Any)
		if err != nil {
			return nil, err
		}

		progress := traversal.Progress{
			Cfg: &traversal.Config{
				Ctx:                            ctx,
				LinkSystem:                     c.ls,
				LinkTargetNodePrototypeChooser: bsfetcher.DefaultPrototypeChooser,
				LinkVisitOnlyOnce:              true,
			},
		}

		err = progress.WalkMatching(node, exploreAllRecursivelySelector, func(progress traversal.Progress, visitedNode datamodel.Node) error {
			link, err := c.ls.ComputeLink(link.Prototype(), visitedNode)
			if err != nil {
				return err
			}
			cidStr := link.String()
			cidSet[cidStr] = struct{}{} // Add CID string to the set
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Convert the map keys to a slice
	var uniqueCidStrings []string
	for cidStr := range cidSet {
		uniqueCidStrings = append(uniqueCidStrings, cidStr)
	}

	return &StringIterator{links: uniqueCidStrings}, nil // Return StringIterator with unique CIDs
}

func (c *Client) ClearCidsFromRecent(cidsBytes []byte) error {
	ctx := context.TODO()

	// Convert byte slice back into a slice of strings
	cidStrs := strings.Split(string(cidsBytes), "|")

	for _, cidStr := range cidStrs {
		// Decode the CID from the string
		cid, err := cid.Decode(cidStr)
		if err != nil {
			continue
		}

		// Generate the datastore key for this CID
		l := cidlink.Link{Cid: cid}

		// Delete the key from the datastore
		if err := c.clearRecentCid(ctx, l); err != nil {
			return err
		}
	}
	return nil
}

// RetryFailedPushes retries pushing all links that failed to push.
// The retry is disrupted as soon as a failure occurs.
// See ListFailedPushes.
func (c *Client) RetryFailedPushes() error {
	ctx := context.TODO()
	links, err := c.listFailedPushes(ctx)
	if err != nil {
		return err
	}
	for _, link := range links {
		if err := c.pushLink(ctx, link); err != nil {
			return err
		}
	}
	return nil
}

// Flush guarantees that all values stored locally are synced to the baking local storage.
func (c *Client) Flush() error {
	return c.ds.Sync(context.TODO(), rootDatastoreKey)
}

// SetAuth sets authorization on the given peer ID for the given subject.
func (c *Client) SetAuth(on string, subject string, allow bool) error {
	onp, err := peer.Decode(on)
	if err != nil {
		return err
	}
	subp, err := peer.Decode(subject)
	if err != nil {
		return err
	}
	return c.ex.SetAuth(context.TODO(), onp, subp, allow)
}

func (c *Client) IpniNotifyLink(l string) {
	link, err := toLinkFromString(l)
	if err == nil {
		c.ex.IpniNotifyLink(link)
	}
}

// Shutdown closes all resources used by Client.
// After calling this function Client must be discarded.
func (c *Client) Shutdown() error {
	ctx := context.TODO()
	xErr := c.ex.Shutdown(ctx)
	hErr := c.h.Close()
	fErr := c.Flush()
	dsErr := c.ds.Close()
	switch {
	case hErr != nil:
		return hErr
	case fErr != nil:
		return fErr
	case dsErr != nil:
		return dsErr
	default:
		return xErr
	}
}

func (c *Client) ShutdownIpfs() error {
	ctx := context.TODO()
	xErr := c.ex.ShutdownIpfs(ctx)
	hErr := c.h.Close()
	fErr := c.Flush()
	dsErr := c.ds.Close()
	psErr := c.ShutdownPubSub()
	switch {
	case hErr != nil:
		return hErr
	case fErr != nil:
		return fErr
	case dsErr != nil:
		return dsErr
	case psErr != nil:
		return psErr
	default:
		return xErr
	}
}
