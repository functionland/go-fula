package fulamobile

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"fmt"
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
	// IdentitySeed is the seed that is used to generate Ed25519 libp2p key-pair. If unspecified, a
	// random seed is used, otherwise, it must be 32 bytes long. Longer seed value is accepted but
	// only the first 32 bytes are read. See: ed25519.SeedSize.
	IdentitySeed []byte
	// StorePath is the path used to persist data. If unspecified, an in-memory store is used.
	StorePath string
}

func (cfg *Config) init(mc *Client) error {
	seedLen := len(cfg.IdentitySeed)
	var seed io.Reader
	switch {
	case seedLen == 0:
		// No seed is given; generate identity at random by leaving seed to be nil.
		break
	case seedLen < ed25519.SeedSize:
		return fmt.Errorf("identity seed too short; it must be at least 32 bytes but got %d", seedLen)
	default:
		seed = bytes.NewBuffer(cfg.IdentitySeed)
	}
	pk, _, err := crypto.GenerateEd25519Key(seed)
	if err != nil {
		return err
	}
	if mc.h, err = libp2p.New(libp2p.Identity(pk)); err != nil {
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
