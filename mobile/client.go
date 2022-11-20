package fulamobile

import (
	"bytes"
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-graphsync"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	ipldmc "github.com/ipld/go-ipld-prime/multicodec"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
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

// NewClient instantiates a new Fula mobile Client.
func NewClient(cfg *Config) (*Client, error) {
	var mc Client
	if err := cfg.init(&mc); err != nil {
		return nil, err
	}
	return &mc, nil
}

// ID returns the libp2p host peer ID of this client.
func (c Client) ID() peer.ID {
	return c.h.ID()
}

// Get gets the value corresponding to the given key from the local ipld.LinkSystem
// The key must be a valid ipld.Link and the value returned is encoded ipld.Node.
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

func toLink(key []byte) (*cidlink.Link, error) {
	_, cc, err := cid.CidFromBytes(key)
	if err != nil {
		return nil, err
	}
	return &cidlink.Link{Cid: cc}, nil
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

// Put stores the given value onto the ipld.LinkSystem and returns its corresponding link.
// The value is decoded using the decoder that corresponds to the given codec. Therefore, the given
// value must be a valid ipld.Node.
func (c *Client) Put(value []byte, codec uint64) ([]byte, error) {
	ctx := context.TODO()
	decode, err := ipldmc.LookupDecoder(codec)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(value)
	nb := basicnode.Prototype.Any.NewBuilder()
	if err := decode(nb, buf); err != nil {
		return nil, err
	}
	node := nb.Build()
	link, err := c.ls.Store(ipld.LinkContext{Ctx: ctx}, cidlink.LinkPrototype{
		Prefix: cid.Prefix{
			Version:  1,
			Codec:    codec,
			MhType:   uint64(multicodec.Sha2_256),
			MhLength: -1,
		},
	}, node)
	if err != nil {
		return nil, err
	}
	return link.(cidlink.Link).Cid.Bytes(), nil
}

// Shutdown closes all resources used by Client.
// After calling this function Client must be discarded.
func (c *Client) Shutdown() error {
	return c.h.Close()
}
