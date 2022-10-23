package fulaMobile_test

import (
	"context"
	"math/rand"
	"os"
	"testing"

	"github.com/functionland/go-fula/drive"
	fulaMobile "github.com/functionland/go-fula/mobile"
	fileP "github.com/functionland/go-fula/protocols/file"

	// "github.com/ipfs/kubo/core"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
)

func initNodes() (host.Host, *fulaMobile.Fula, error) {
	apis, _, err := fulaMobile.MakeAPISwarm(context.Background(), true, 2)
	if err != nil {
		return nil, nil, err
	}
	fapi := apis[0]
	// node1 := nodes[0]
	// node2 := nodes[1]
	ctx := context.Background()
	ds := drive.NewDriveStore()

	rng := rand.New(rand.NewSource(42))
	pid1, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h1, err := libp2p.New(libp2p.Identity(pid1))
	if err != nil {
		panic(err)
	}

	h1.SetStreamHandler(fileP.ProtocolId, func(s network.Stream) {
		fileP.Handle(ctx, fapi, ds, s)
	})

	// bs1 := []peer.AddrInfo{node1.Peerstore.PeerInfo(node1.Identity)}
	// bs2 := []peer.AddrInfo{node2.Peerstore.PeerInfo(node2.Identity)}

	// if err := node2.Bootstrap(bootstrap.BootstrapConfigWithPeers(bs1)); err != nil {
	// 	return nil, nil, err
	// }
	// if err := node1.Bootstrap(bootstrap.BootstrapConfigWithPeers(bs2)); err != nil {
	// 	return nil, nil, err
	// }

	fula, err := fulaMobile.NewFula("./repo")
	if err != nil {
		return nil, nil, err
	}

	// fula.Node = *routedhost.Wrap(node2.PeerHost, node2.Routing)

	// err = fula.AddBox("/p2p/" + node1.Identity.Pretty())
	err = fula.AddBox(h1.ID(), h1.Addrs())
	if err != nil {
		return nil, nil, err
	}
	h1.Peerstore().AddAddrs(fula.Node.ID(), fula.Node.Addrs(), peerstore.PermanentAddrTTL)
	if err = h1.Connect(ctx, peer.AddrInfo{ID: fula.Node.ID(), Addrs: fula.Node.Addrs()}); err != nil {
		panic(err)
	}

	return h1, fula, nil
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

	want, err := peer.AddrInfoFromString("/p2p/" + node.ID().Pretty())
	// want, err := peer.AddrInfoFromString("/p2p/" + node.Identity.Pretty())
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
	exp := fulaMobile.DirEntry{Name: "DID", Type: fulaMobile.File, Size: 9, Cid: "QmaHLcFdVDqcVEvS9LdurpApHuenXwjhRY1tSHykCzbC91"}
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
	eDID := fulaMobile.DirEntry{Name: "DID", Type: fulaMobile.File, Size: 9, Cid: "QmaHLcFdVDqcVEvS9LdurpApHuenXwjhRY1tSHykCzbC91"}
	ePhotos := fulaMobile.DirEntry{Name: "photos", Type: fulaMobile.Directory, Size: 0, Cid: "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"}
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

	ePhotos := fulaMobile.DirEntry{Name: "photos", Type: fulaMobile.Directory, Size: 0, Cid: "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn"}
	p := es[0]

	if p != ePhotos {
		t.Fatal("Recieved items in the list are not what expected")
	}
}
