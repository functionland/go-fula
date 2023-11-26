package exchange

import (
	"context"
	"errors"

	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
)

type fulaDht struct {
	*options
	h      host.Host
	dh     *dht.IpfsDHT
	ctx    context.Context
	cancel context.CancelFunc
}

func newDhtProvider(h host.Host, opts *options) (*fulaDht, error) {
	d := &fulaDht{
		h:       h,
		options: opts,
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	var err error
	d.dh, err = dht.New(d.ctx, h, opts.dhtProviderOpts...)
	if err != nil {
		return nil, err
	}
	log.Infow("Dht is initialized with this", "peers")
	return d, nil
}

func (d *fulaDht) PingDht(p peer.ID) error {
	err := d.dh.Ping(d.ctx, p)
	return err
}

func (d *fulaDht) AddPeer(p peer.ID) {
	d.dh.RoutingTable().PeerAdded(p)
	log.Infow("AddPeer", "self", d.dh.PeerID(), "added", p)
	d.dh.RefreshRoutingTable()
}

func (d *fulaDht) Provide(l ipld.Link) error {
	if l == nil {
		err := errors.New("link is nil")
		log.Errorw("Failed to Provide Dht, link is nil", "err", err)
		return err
	}
	link, ok := l.(cidlink.Link)
	if ok &&
		!cid.Undef.Equals(link.Cid) &&
		link.Cid.Prefix().MhType != multihash.IDENTITY {
		return d.dh.Provide(d.ctx, link.Cid, true)
	}
	err := errors.New("cid is undefined or invalid")
	return err
}

func (d *fulaDht) FindProviders(l ipld.Link) ([]peer.AddrInfo, error) {
	if l == nil {
		err := errors.New("link is nil")
		log.Errorw("Failed to Provide Dht, link is nil", "err", err)
		return nil, err
	}
	link, ok := l.(cidlink.Link)
	if ok &&
		!cid.Undef.Equals(link.Cid) &&
		link.Cid.Prefix().MhType != multihash.IDENTITY {
		return d.dh.FindProviders(d.ctx, link.Cid)
	}
	err := errors.New("cid is undefined or invalid")
	return nil, err
}

func (d *fulaDht) Shutdown() error {
	d.cancel()
	return d.dh.Close()
}
