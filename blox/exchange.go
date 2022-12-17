package blox

import (
	"context"
	"fmt"

	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (p *Blox) Pull(ctx context.Context, from peer.ID, l ipld.Link) error {
	if exists, err := p.Has(ctx, l); err != nil {
		return err
	} else if exists {
		return nil
	}
	return p.ex.Pull(ctx, from, l)
}

func (p *Blox) Push(ctx context.Context, from peer.ID, l ipld.Link) error {
	if exists, err := p.Has(ctx, l); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("cannot push non-existing link: %s", l.String())
	}
	return p.ex.Push(ctx, from, l)
}
