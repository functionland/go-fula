package event

import (
	"github.com/ipld/go-ipld-prime"
	"github.com/libp2p/go-libp2p/core"
)

type Event struct {
	Version   string
	Previous  ipld.Link
	Peer      core.PeerID
	Signature []byte
}
