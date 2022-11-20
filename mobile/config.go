package fulamobile

import (
	"bytes"
	"context"
	"io"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/ipfs/go-graphsync"
	gs "github.com/ipfs/go-graphsync/impl"
	gsnet "github.com/ipfs/go-graphsync/network"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Config struct {
	Identity  []byte
	StorePath string
}

func (cfg *Config) init(mc *Client) error {
	var err error
	var hopts []libp2p.Option
	if len(cfg.Identity) != 0 {
		pk, err := crypto.UnmarshalPrivateKey(cfg.Identity)
		if err != nil {
			return err
		}
		hopts = append(hopts, libp2p.Identity(pk))
	}
	if mc.h, err = libp2p.New(hopts...); err != nil {
		return err
	}
	if cfg.StorePath == "" {
		mc.ds = dssync.MutexWrap(datastore.NewMapDatastore())
	} else {
		mc.ds, err = badger.NewDatastore(cfg.StorePath, &badger.DefaultOptions)
		if err != nil {
			return err
		}
	}
	mc.ls = cidlink.DefaultLinkSystem()
	mc.ls.StorageWriteOpener = func(ctx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		buf := bytes.NewBuffer(nil)
		return buf, func(l ipld.Link) error {
			return mc.ds.Put(ctx.Ctx, datastore.NewKey(l.Binary()), buf.Bytes())
		}, nil
	}
	mc.ls.StorageReadOpener = func(ctx ipld.LinkContext, l ipld.Link) (io.Reader, error) {
		val, err := mc.ds.Get(ctx.Ctx, datastore.NewKey(l.Binary()))
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(val), nil
	}

	gsn := gsnet.NewFromLibp2pHost(mc.h)
	mc.gx = gs.New(context.Background(), gsn, mc.ls)
	mc.gx.RegisterIncomingRequestHook(
		func(p peer.ID, r graphsync.RequestData, ha graphsync.IncomingRequestHookActions) {
			// TODO only allow connections from known peer IDs, like the blox peer id
			ha.ValidateRequest()
		})

	return nil
}
