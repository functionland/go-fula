package fulamobile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/exchange"
	bsfetcher "github.com/ipfs/boxo/fetcher/impl/blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
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
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
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
	h       host.Host
	ds      datastore.Batching
	ls      ipld.LinkSystem
	ex      exchange.Exchange
	bl      blockchain.Blockchain
	bloxPid peer.ID
	relays  []string

	ipfsDHT       *dht.IpfsDHT   // Standard IPFS DHT for peer discovery fallback
	ipfsDHTReady  chan struct{}   // Closed when DHT bootstrap completes
	ipfsDHTCtx    context.Context
	ipfsDHTCancel context.CancelFunc

	streams map[string]*blockchain.StreamBuffer // Map of active streams
	mu      sync.Mutex                          // Mutex for thread-safe access
}

func NewClient(cfg *Config) (*Client, error) {
	var mc Client
	if err := cfg.init(&mc); err != nil {
		return nil, err
	}

	// Initialize the streams map for managing active streaming sessions
	mc.streams = make(map[string]*blockchain.StreamBuffer)

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
	// Shut down IPFS DHT before closing the host
	if c.ipfsDHTCancel != nil {
		c.ipfsDHTCancel()
	}
	var dhtErr error
	if c.ipfsDHT != nil {
		dhtErr = c.ipfsDHT.Close()
	}
	hErr := c.h.Close()
	fErr := c.Flush()
	dsErr := c.ds.Close()
	switch {
	case hErr != nil:
		return hErr
	case dhtErr != nil:
		return dhtErr
	case fErr != nil:
		return fErr
	case dsErr != nil:
		return dsErr
	default:
		return xErr
	}
}
