package mobile

import (
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
)

const BOX = "/ip4/192.168.1.10/tcp/4002/p2p/12D3KooWGrkcHUBzAAuYhMRxBreCgofKKDhLgR84FbawknJZHwK1"
const BOX_LOOPBACK = "/ip4/127.0.0.1/tcp/4002/p2p/12D3KooWGrkcHUBzAAuYhMRxBreCgofKKDhLgR84FbawknJZHwK1"


func TestNew(t *testing.T) {
	_, err := NewFula()
	if err != nil {
		t.Error(err)
	}
}

func TestAddBox(t *testing.T) {
	fula, err := NewFula()
	if err != nil {
		t.Error(err)
	}
	err = fula.AddBox(BOX)
	if err != nil {
		t.Error("Fail to adidng peer: \n", err)
		return
	}
	want, _ := peer.AddrInfoFromString(BOX)
	peers := fula.node.Peerstore().PeersWithAddrs()
	for _, id := range peers {
		if id == want.ID {
			return
		}
	}
	t.Error("Peer Was Not added")
}

func TestAddBoxLoopBack(t *testing.T){
	fula, err := NewFula()
	if err != nil {
		t.Error(err)
	}
	err = fula.AddBox(BOX)
	if err != nil {
		t.Error("Mobile Can not accept loopback")
	}
}

func TestAddBoxSendFile(t *testing.T){
	fula, err := NewFula()
	if err != nil {
		t.Error(err)
	}
	err = fula.AddBox(BOX)
	if err != nil {
		t.Error("Mobile Can not accept loopback")
	}
}