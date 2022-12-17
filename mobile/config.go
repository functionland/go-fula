package fulamobile

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/functionland/go-fula/exchange"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

const noopExchange = "noop"

type Config struct {
	Identity  []byte
	StorePath string
	// Exchange specifies the DAG exchange protocol for Fula mobile client. If left unspecified,
	// The default FxExchange protocol is used which will attempt to make remote connections
	// when links are stored and retrieved.
	//
	// For testing purposes, the value may be set to `noop`, in which case, no remote connections
	// will be made and the requested exchange is simply logged. When the value is set to `noop`
	// the BloxAddr may also be left empty.
	Exchange string
	BloxAddr string

	// TODO we don't need to take BloxAddr when there is a discovery mechanism facilitated via fx.land.
	//      For now we manually take BloxAddr as config.

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
	if cfg.BloxAddr == "" {
		if cfg.Exchange != noopExchange {
			return errors.New("the BloxAddr must be specified until autodiscovery service is implemented; " +
				"for testing purposes, BloxAddr may be omitted only when Exchange is set to `noop`")
		}
	} else {
		bloxAddr, err := peer.AddrInfoFromString(cfg.BloxAddr)
		if err != nil {
			return err
		}
		mc.h.Peerstore().AddAddrs(bloxAddr.ID, bloxAddr.Addrs, peerstore.PermanentAddrTTL)
		mc.bloxPid = bloxAddr.ID
	}
	mc.ls = cidlink.DefaultLinkSystem()
	mc.ls.StorageWriteOpener = func(ctx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		buf := bytes.NewBuffer(nil)
		return buf, func(l ipld.Link) error {
			k := datastore.NewKey(l.Binary())
			if err := mc.ds.Put(ctx.Ctx, k, buf.Bytes()); err != nil {
				return err
			}
			if err := mc.ds.Sync(ctx.Ctx, k); err != nil {
				return err
			}
			return mc.ex.Push(ctx.Ctx, mc.bloxPid, l)
		}, nil
	}
	mc.ls.StorageReadOpener = func(ctx ipld.LinkContext, l ipld.Link) (io.Reader, error) {
		val, err := mc.ds.Get(ctx.Ctx, datastore.NewKey(l.Binary()))
		if err == datastore.ErrNotFound {
			// Attempt to pull missing link from blox if missing from local datastore.
			if err := mc.ex.Pull(ctx.Ctx, mc.bloxPid, l); err != nil {
				return nil, err
			}
			val, err = mc.ds.Get(ctx.Ctx, datastore.NewKey(l.Binary()))
		}
		switch err {
		case nil:
			return bytes.NewBuffer(val), nil
		default:
			return nil, err
		}
	}
	switch cfg.Exchange {
	case noopExchange:
		mc.ex = exchange.NoopExchange{}
	default:
		mc.ex = exchange.NewFxExchange(mc.h, mc.ls)
	}
	return mc.ex.Start(context.TODO())
}
