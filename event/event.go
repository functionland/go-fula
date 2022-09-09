package event

import (
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p-core/peer"
)

type Event struct {
	Version   string
	Previous  ipld.Link
	Peer      peer.ID
	Signature []byte
}
