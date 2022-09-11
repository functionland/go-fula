package mobile_test

import (
	"context"
	"os"
	"testing"

	"github.com/functionland/go-fula/drive"
	"github.com/functionland/go-fula/mobile"
	fileP "github.com/functionland/go-fula/protocols/file"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/bootstrap"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
)

func initNodes() (*core.IpfsNode, *mobile.Fula, error) {
	apis, nodes, err := mobile.MakeAPISwarm(context.Background(), true, 2)
	if err != nil {
		return nil, nil, err
	}
	fapi := apis[0]
	node1 := nodes[0]
	node2 := nodes[1]
	ctx := context.Background()
	ds := drive.NewDriveStore()

	node1.PeerHost.SetStreamHandler(fileP.ProtocolId, func(s network.Stream) {
		fileP.Handle(ctx, fapi, ds, s)
	})

	bs1 := []peer.AddrInfo{node1.Peerstore.PeerInfo(node1.Identity)}
	bs2 := []peer.AddrInfo{node2.Peerstore.PeerInfo(node2.Identity)}

	if err := node2.Bootstrap(bootstrap.BootstrapConfigWithPeers(bs1)); err != nil {
		return nil, nil, err
	}
	if err := node1.Bootstrap(bootstrap.BootstrapConfigWithPeers(bs2)); err != nil {
		return nil, nil, err
	}

	fula, err := mobile.NewFula("./repo")
	if err != nil {
		return nil, nil, err
	}

	fula.Node = *routedhost.Wrap(node2.PeerHost, node2.Routing)

	err = fula.AddBox("/p2p/" + node1.Identity.Pretty())
	if err != nil {
		return nil, nil, err
	}

	return node1, fula, nil
}

func TestNew(t *testing.T) {

	_, _, err := initNodes()
	if err != nil {
		t.Error(err)
	}
}

func TestAddBox(t *testing.T) {
	node, fula, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	want, err := peer.AddrInfoFromString("/p2p/" + node.Identity.Pretty())
	if err != nil {
		t.Fatal(err)
	}
	peer, err := fula.GetBox()
	if err != nil {
		t.Error(err)
	}
	if want.ID == peer {
		return
	}
	t.Error("Peer Was Not added")
}

func TestFulaRead(t *testing.T) {
	_, fula, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	err = fula.Read("Mehdi-DID", "/DID", "./tmp/DID")
	if err != nil {
		t.Fatal(err)
	}

	rb, err := os.ReadFile("./tmp/DID")
	if err != nil {
		t.Fatal(err)
	}

	if string(rb) != "Mehdi-DID" {
		t.Fatal("Recieved content does not match")
	}
}

func TestFulaWrite(t *testing.T) {
	_, fula, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	err = fula.Write("Mehdi-DID", "./test_assets/t.test", "/my-file")
	if err != nil {
		t.Fatal(err)
	}

	err = fula.Read("Mehdi-DID", "/my-file", "./tmp/t.res")
	if err != nil {
		t.Fatal(err)
	}

	tb, err := os.ReadFile("./test_assets/t.test")
	if err != nil {
		t.Fatal(err)
	}

	rb, err := os.ReadFile("./tmp/t.res")
	if err != nil {
		t.Fatal(err)
	}

	if string(tb) != string(rb) {
		t.Fatal("Recieved content does not match")
	}

}

func TestFulaLs(t *testing.T) {
	_, fula, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	es, err := fula.Ls("Mehdi-DID", "/")
	if err != nil {
		t.Fatal(err)
	}

	if len(es) != 1 {
		t.Fatal("Recieved list has wrong len")
	}

	e := es[0]
	exp := mobile.DirEntry{Name: "DID", Type: mobile.File, Size: 9, Cid: "QmaHLcFdVDqcVEvS9LdurpApHuenXwjhRY1tSHykCzbC91"}
	if e != exp {
		t.Fatal("Recieved item in the list is not what expected")
	}
}

func TestFulaMkDir(t *testing.T) {
	_, fula, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	err = fula.MkDir("Mehdi-DID", "/photos")
	if err != nil {
		t.Fatal(err)
	}

	es, err := fula.Ls("Mehdi-DID", "/")
	if err != nil {
		t.Fatal(err)
	}

	if len(es) != 2 {
		t.Fatal("Recieved list has wrong len")
	}

	foundDID := false
	foundPhotos := false
	eDID := mobile.DirEntry{Name: "DID", Type: mobile.File, Size: 9, Cid: "QmaHLcFdVDqcVEvS9LdurpApHuenXwjhRY1tSHykCzbC91"}
	ePhotos := mobile.DirEntry{Name: "photos", Type: mobile.Directory, Size: 0, Cid: "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"}
	for _, d := range es {
		if d == eDID {
			foundDID = true
		}
		if d == ePhotos {
			foundPhotos = true
		}
	}

	if !foundDID || !foundPhotos {
		t.Fatal("Recieved items in the list are not what expected")
	}
}

func TestFulaDelete(t *testing.T) {
	_, fula, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	err = fula.MkDir("Mehdi-DID", "/photos")
	if err != nil {
		t.Fatal(err)
	}

	err = fula.Delete("Mehdi-DID", "/DID")
	if err != nil {
		t.Fatal(err)
	}

	es, err := fula.Ls("Mehdi-DID", "/")
	if err != nil {
		t.Fatal(err)
	}

	if len(es) != 1 {
		t.Fatal("Recieved list has wrong len")
	}

	ePhotos := mobile.DirEntry{Name: "photos", Type: mobile.Directory, Size: 0, Cid: "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"}
	p := es[0]

	if p != ePhotos {
		t.Fatal("Recieved items in the list are not what expected")
	}
}
