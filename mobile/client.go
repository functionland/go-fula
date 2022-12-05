package fulamobile

import (
	"bytes"
	"context"
	"errors"

	"github.com/functionland/go-fula/exchange"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	ipldmc "github.com/ipld/go-ipld-prime/multicodec"
	basicnode "github.com/ipld/go-ipld-prime/node/basic"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
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

type Client struct {
	h       host.Host
	ds      datastore.Batching
	ls      ipld.LinkSystem
	ex      exchange.Exchange
	bloxPid peer.ID
}

func NewClient(cfg *Config) (*Client, error) {
	var mc Client
	if err := cfg.init(&mc); err != nil {
		return nil, err
	}
	return &mc, nil
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
	return c.ex.Pull(context.TODO(), c.bloxPid, l)
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
	if exists, err := c.hasLink(l); err != nil {
		return err
	} else if !exists {
		return errors.New("value not found locally")
	}
	return c.ex.Push(context.TODO(), c.bloxPid, l)
}

// Put stores the given value onto the ipld.LinkSystem and returns its corresponding link.
// The value is decoded using the decoder that corresponds to the given codec. Therefore,
// the given value must be a valid ipld.Node.
// Upon successful local storage of the given value, it is automatically pushed to the blox
// at Config.BloxAddr address.
func (c *Client) Put(value []byte, codec int64) ([]byte, error) {
	ctx := context.TODO()
	decode, err := ipldmc.LookupDecoder(uint64(codec))
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(value)
	nb := basicnode.Prototype.Any.NewBuilder()
	if err := decode(nb, buf); err != nil {
		return nil, err
	}
	node := nb.Build()
	link, err := c.ls.Store(ipld.LinkContext{Ctx: ctx}, lp, node)
	if err != nil {
		return nil, err
	}
	return link.(cidlink.Link).Cid.Bytes(), nil
}

// Shutdown closes all resources used by Client.
// After calling this function Client must be discarded.
func (c *Client) Shutdown() error {
	ctx := context.TODO()
	xErr := c.ex.Shutdown(ctx)
	hErr := c.h.Close()
	if hErr != nil {
		return hErr
	}
	return xErr
}
