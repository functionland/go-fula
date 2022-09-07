package mobile_test

import (
	"context"
	"crypto/md5"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/functionland/go-fula/drive"
	"github.com/functionland/go-fula/mobile"
	fileP "github.com/functionland/go-fula/protocols/newFile"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/bootstrap"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	routedhost "github.com/libp2p/go-libp2p/p2p/host/routed"
)

const BOX = "/p2p/12D3KooWJVDdxaWYxSEC3M8oK57swu1jc36YYMZihbLmiQjQ2B26"
const BOX_LOOPBACK = "/ip4/127.0.0.1/tcp/4002/p2p/12D3KooWGrkcHUBzAAuYhMRxBreCgofKKDhLgR84FbawknJZHwK1"

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

// func TestAddBoxLoopBack(t *testing.T) {
// 	fula, err := mobile.NewFula("./repo")
// 	if err != nil {
// 		t.Error(err)
// 	}
// 	err = fula.AddBox(BOX)
// 	if err != nil {
// 		t.Error("Mobile Can not accept loopback")
// 	}
// }

func TestFileProtocol(t *testing.T) {
	fula, err := mobile.NewFula("./repo")
	if err != nil {
		t.Error(err)
	}
	err = fula.AddBox(BOX)
	if err != nil {
		t.Error("add error")
	}
	tmp := "./tmp"
	if _, err := os.Stat(tmp); os.IsNotExist(err) {
		err := os.Mkdir(tmp, 0755)
		if err != nil {
			t.Error("wired error", err)
			return
		}
	}
	t.Log("fula ready")
	files, err := ioutil.ReadDir("./test_assets")
	if err != nil {
		t.Error(err)
	}
	for _, file := range files {
		if !file.IsDir() {
			upload := "./test_assets/" + file.Name()
			cid, err := fula.Send(upload)
			if err != nil {
				t.Error("send failed", err)
				return
			}
			meta, err := fula.RecieveFileMetaInfo(cid)
			t.Log("File with CID: ", cid)
			if err != nil {
				t.Error("download Failed", err)
				return
			}
			download := tmp + "/" + meta.Name
			err = fula.ReceiveFile(cid, download)
			if err != nil {
				t.Error("Receive File failed", err)
				return
			}
			if !fileDiff(upload, download) {
				t.Error("Somthing wrong! files are not equal", err)
				return
			}
			t.Logf("successfully test send and receive of %s", upload)
			time.Sleep(time.Second)
		}

	}

}

func TestEncryption(t *testing.T) {
	fula, err := mobile.NewFula("./repo")
	if err != nil {
		t.Error(err)
	}
	err = fula.AddBox(BOX)
	if err != nil {
		t.Error("can not add box", err)
	}
	tmp := "./tmp"
	if _, err := os.Stat(tmp); os.IsNotExist(err) {
		err := os.Mkdir(tmp, 0755)
		if err != nil {
			t.Error("wired error", err)
			return
		}
	}
	files, err := ioutil.ReadDir("./test_assets")
	if err != nil {
		t.Error(err)
	}
	for _, file := range files {
		if !file.IsDir() {
			upload := "./test_assets/" + file.Name()
			ref, err := fula.EncryptSend(upload)
			if err != nil {
				t.Error("send failed", err)
				return
			}
			download := tmp + "/" + ref
			err = fula.ReceiveDecryptFile(ref, download)
			if err != nil {
				t.Error("receive File failed", err)
				return
			}
			if !fileDiff(upload, download) {
				t.Error("somthing wrong! files are not equal", err)
				return
			}
		}
		time.Sleep(time.Second)
	}

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

func fileDiff(path1 string, path2 string) bool {
	hash1 := md5File(path1)
	hash2 := md5File(path2)
	return string(hash1) == string(hash2)
}

func md5File(path string) []byte {
	file, err := os.Open(path)

	if err != nil {
		panic(err)
	}

	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)

	if err != nil {
		panic(err)
	}
	return hash.Sum(nil)
}
