package pool

import (
	"bytes"
	"context"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multicodec"
)

var lp = cidlink.LinkPrototype{
	Prefix: cid.Prefix{
		Version:  1,
		Codec:    uint64(multicodec.DagCbor),
		MhType:   uint64(multicodec.Sha2_256),
		MhLength: -1,
	},
}

func (p *Pool) Store(ctx context.Context, n ipld.Node) (ipld.Link, error) {
	return p.ls.Store(ipld.LinkContext{Ctx: ctx}, lp, n)
}

func (p *Pool) Load(ctx context.Context, l ipld.Link, np ipld.NodePrototype) (ipld.Node, error) {
	return p.ls.Load(ipld.LinkContext{Ctx: ctx}, l, np)
}

func (p *Pool) Has(ctx context.Context, l ipld.Link) (bool, error) {
	return p.ds.Has(ctx, toDatastoreKey(l))
}

func (p *Pool) blockWriteOpener(ctx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
	buf := bytes.NewBuffer(nil)
	return buf, func(l ipld.Link) error {
		return p.ds.Put(ctx.Ctx, toDatastoreKey(l), buf.Bytes())
	}, nil
}

func (p *Pool) blockReadOpener(ctx ipld.LinkContext, l ipld.Link) (io.Reader, error) {
	val, err := p.ds.Get(ctx.Ctx, toDatastoreKey(l))
	switch err {
	case nil:
		return bytes.NewBuffer(val), nil
	case datastore.ErrNotFound:
		return nil, format.ErrNotFound{Cid: l.(cidlink.Link).Cid}
	default:
		return nil, err
	}
}

func toDatastoreKey(l ipld.Link) datastore.Key {
	return datastore.NewKey(l.Binary())
}
