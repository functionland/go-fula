package mobile

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/functionland/go-fula/drive"
	fxfscore "github.com/functionland/go-fula/fxfs/core/api"
	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	"github.com/functionland/go-fula/protocols/newFile"
	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-filestore"
	keystore "github.com/ipfs/go-ipfs-keystore"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/bootstrap"
	"github.com/ipfs/kubo/core/coreapi"
	mock "github.com/ipfs/kubo/core/mock"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

var testPeerID = "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe"

func MakeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]fxiface.CoreAPI, []*core.IpfsNode, error) {
	mn := mocknet.New()

	nodes := make([]*core.IpfsNode, n)
	apis := make([]fxiface.CoreAPI, n)

	for i := 0; i < n; i++ {
		var ident config.Identity
		if fullIdentity {
			sk, pk, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
			if err != nil {
				return nil, nil, err
			}

			id, err := peer.IDFromPublicKey(pk)
			if err != nil {
				return nil, nil, err
			}

			kbytes, err := crypto.MarshalPrivateKey(sk)
			if err != nil {
				return nil, nil, err
			}

			ident = config.Identity{
				PeerID:  id.Pretty(),
				PrivKey: base64.StdEncoding.EncodeToString(kbytes),
			}
		} else {
			ident = config.Identity{
				PeerID: testPeerID,
			}
		}

		c := config.Config{}
		c.Addresses.Swarm = []string{fmt.Sprintf("/ip4/18.0.%d.1/tcp/4001", i)}
		c.Identity = ident
		c.Experimental.FilestoreEnabled = true

		ds := syncds.MutexWrap(datastore.NewMapDatastore())
		r := &repo.Mock{
			C: c,
			D: ds,
			K: keystore.NewMemKeystore(),
			F: filestore.NewFileManager(ds, filepath.Dir(os.TempDir())),
		}

		node, err := core.NewNode(ctx, &core.BuildCfg{
			Routing: libp2p.DHTServerOption,
			Repo:    r,
			Host:    mock.MockHostOption(mn),
			Online:  fullIdentity,
			ExtraOpts: map[string]bool{
				"pubsub": true,
			},
		})
		if err != nil {
			return nil, nil, err
		}
		nodes[i] = node

		capi, err := coreapi.NewCoreAPI(node)
		if err != nil {
			fmt.Println(err)
			return nil, nil, err
		}

		apis[i], err = fxfscore.NewCoreAPI(node, capi.(*coreapi.CoreAPI))
		if err != nil {
			return nil, nil, err
		}
	}

	err := mn.LinkAll()
	if err != nil {
		return nil, nil, err
	}

	bsinf := bootstrap.BootstrapConfigWithPeers(
		[]peer.AddrInfo{
			nodes[0].Peerstore.PeerInfo(nodes[0].Identity),
		},
	)

	for _, n := range nodes[1:] {
		if err := n.Bootstrap(bsinf); err != nil {
			return nil, nil, err
		}
	}

	return apis, nodes, nil
}

func initNodes() (network.Stream, error) {
	apis, nodes, err := MakeAPISwarm(context.Background(), true, 2)
	if err != nil {
		return nil, err
	}
	fapi := apis[0]
	node1 := nodes[0]
	node2 := nodes[1]
	ctx := context.Background()
	ds := drive.NewDriveStore()

	node1.PeerHost.SetStreamHandler(newFile.ProtocolId, func(s network.Stream) {
		newFile.Handle(ctx, fapi, ds, s)
	})

	bs1 := []peer.AddrInfo{node1.Peerstore.PeerInfo(node1.Identity)}
	bs2 := []peer.AddrInfo{node2.Peerstore.PeerInfo(node2.Identity)}

	if err := node2.Bootstrap(bootstrap.BootstrapConfigWithPeers(bs1)); err != nil {
		return nil, err
	}
	if err := node1.Bootstrap(bootstrap.BootstrapConfigWithPeers(bs2)); err != nil {
		return nil, err
	}

	s, err := node2.PeerHost.NewStream(ctx, node1.PeerHost.ID(), newFile.ProtocolId)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func TestNewFileProtocol(t *testing.T) {

	s, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	res, err := newFile.RequestRead(context.Background(), s, "/DID", "MEHDI_DID")
	if err != nil {
		t.Fatal(err)
	}

	file, err := io.ReadAll(res)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("RES:", string(file))

	if string(file) != "MEHDI_DID" {
		t.Fatal("result is not as expected")
	}
}

func TestMkDirAction(t *testing.T) {
	s, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	dcid, err := newFile.RequestMkDir(context.Background(), s, "/photos", "MEHDI_DID")
	if err != nil {
		t.Fatal(err)
	}

	t.Log("RES:", dcid)

}

func TestWriteAction(t *testing.T) {
	s, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	f := bytes.NewReader([]byte("some test content for file"))
	fcid, err := newFile.RequestWrite(context.Background(), s, "/data.txt", "MEHDI_DID", f)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("RES: ", fcid)
}

func TestDeleteAction(t *testing.T) {
	s, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	de, err := newFile.RequestLs(context.Background(), s, "/", "MEHDI_DID")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(de)

	err = newFile.RequestDelete(context.Background(), s, "/DID", "MEHDI_DID")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("deleted file")
}

func TestLsAction(t *testing.T) {
	s, err := initNodes()
	if err != nil {
		t.Fatal(err)
	}

	de, err := newFile.RequestLs(context.Background(), s, "/", "MEHDI_DID")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(de)

}
