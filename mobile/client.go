package fulamobile

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-graphsync"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multicodec"
)

var (
	lp = cidlink.LinkPrototype{
		Prefix: cid.Prefix{
			Version:  1,
			Codec:    uint64(multicodec.DagCbor),
			MhType:   uint64(multicodec.Sha2_256),
			MhLength: -1,
		},
	}
	pushExtensionData = graphsync.ExtensionData{
		Name: "fx.land/v0/push",
	}
)

// Note to self; copied from gomobile docs:
// All exported symbols in the package must have types that are supported. Supported types include:
//  * Signed integer and floating point types.
//  * String and boolean types.
//  * Byte slice types.
//    * Note that byte slices are passed by reference, and support mutation.
//  * Any function type all of whose parameters and results have supported types.
//    * Functions must return either no results, one result, or two results where the type of the second is the built-in 'error' type.
//  * Any interface type, all of whose exported methods have supported function types.
//  * Any struct type, all of whose exported methods have supported function types and all of whose exported fields have supported types.

var logger = logging.Logger("fula/mobile")

type Client struct {
	h  host.Host
	ds datastore.Batching
	ls ipld.LinkSystem
	gx graphsync.GraphExchange
}

func NewClient(cfg *Config) (*Client, error) {
	var mc Client
	if err := cfg.init(&mc); err != nil {
		return nil, err
	}
	return &mc, nil
}

// Get gets the value corresponding to the given key from the local datastore.
// The key must be a valid ipld.Link.
func (c *Client) Get(key []byte) ([]byte, error) {
	l, err := toLink(key)
	if err != nil {
		return nil, err
	}
	exists, err := c.hasLink(l)
	switch {
	case err != nil:
		return nil, err
	case !exists:
		return nil, datastore.ErrNotFound
	default:
		return c.ds.Get(context.Background(), datastore.NewKey(l.Binary()))
	}
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

func toLink(key []byte) (ipld.Link, error) {
	_, cc, err := cid.CidFromBytes(key)
	if err != nil {
		return nil, err
	}
	return cidlink.Link{Cid: cc}, nil
}

// Pull downloads the data corresponding to the given key from the given addr.
// The key must be a valid ipld.Link, and the addr must be a valid multiaddr that includes peer ID.
// See peer.AddrInfoFromString.
func (c *Client) Pull(addr string, key []byte) error {
	// TODO we don't need to take addr when there is a discovery mechanism facilitated via fx.land.
	//      For now we manually take addr, the blox address, as an argument.

	l, err := toLink(key)
	if err != nil {
		return err
	}
	if exists, err := c.hasLink(l); err != nil {
		return err
	} else if exists {
		return nil
	}

	from, err := peer.AddrInfoFromString(addr)
	if err != nil {
		return err
	}
	c.h.Peerstore().AddAddrs(from.ID, from.Addrs, peerstore.TempAddrTTL)
	ctx := context.TODO()
	resps, errs := c.gx.Request(ctx, from.ID, l, selectorparse.CommonSelector_ExploreAllRecursively)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case resp, ok := <-resps:
			if !ok {
				return nil
			}
			logger.Infow("synced node", "node", resp.Node)
		case err, ok := <-errs:
			if !ok {
				return nil
			}
			logger.Warnw("sync failed", "err", err)
		}
	}
}

// Push requests the given addr to download the given key from this node.
// The key must be a valid ipld.Link, and the addr must be a valid multiaddr that includes peer ID.
// The value corresponding to the given key must be stored in the local datastore prior to calling
// this function.
// See: Client.Put, peer.AddrInfoFromString.
func (c *Client) Push(addr string, key []byte) error {
	l, err := toLink(key)
	if err != nil {
		return err
	}
	if exists, err := c.hasLink(l); err != nil {
		return err
	} else if !exists {
		return errors.New("value not found locally")
	}
	to, err := peer.AddrInfoFromString(addr)
	if err != nil {
		return err
	}
	c.h.Peerstore().AddAddrs(to.ID, to.Addrs, peerstore.TempAddrTTL)
	ctx := context.TODO()
	resps, errs := c.gx.Request(ctx, to.ID, l, selectorparse.CommonSelector_ExploreAllRecursively, pushExtensionData)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case resp, ok := <-resps:
			if !ok {
				return nil
			}
			logger.Infow("synced node", "node", resp.Node)
		case err, ok := <-errs:
			if !ok {
				return nil
			}
			logger.Warnw("sync failed", "err", err)
		}
	}
}

// Put stores the given key value onto the local datastore.
// The key must be a valid ipld.Link and the value must be the valid encoded ipld.Node corresponding
// to the given key.
func (c *Client) Put(key, value []byte) error {
	ctx := context.TODO()
	link, err := toLink(key)
	if err != nil {
		return err
	}
	return c.ds.Put(ctx, datastore.NewKey(link.Binary()), value)
}

// Shutdown closes all resources used by Client.
// After calling this function Client must be discarded.
func (c *Client) Shutdown() error {
	return c.h.Close()
}
