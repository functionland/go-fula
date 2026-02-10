package blox

import (
	"context"
	"fmt"

	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (p *Blox) Pull(ctx context.Context, from peer.ID, l ipld.Link) error {
	return fmt.Errorf("pull is no longer supported: data replication has moved to kubo/IPFS-cluster")
}

func (p *Blox) Push(ctx context.Context, from peer.ID, l ipld.Link) error {
	return fmt.Errorf("push is no longer supported: data replication has moved to kubo/IPFS-cluster")
}
