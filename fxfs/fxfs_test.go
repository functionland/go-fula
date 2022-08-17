package fxfs

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	fxfscore "github.com/functionland/go-fula/fxfs/core/api"
	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	pfs "github.com/functionland/go-fula/fxfs/core/pfs"
	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-filestore"
	files "github.com/ipfs/go-ipfs-files"
	keystore "github.com/ipfs/go-ipfs-keystore"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/bootstrap"
	"github.com/ipfs/kubo/core/coreapi"
	mock "github.com/ipfs/kubo/core/mock"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

const testPeerID = "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe"

type NodeProvider struct{}

func (NodeProvider) MakeAPISwarm(ctx context.Context, fullIdentity bool, n int) ([]fxiface.CoreAPI, error) {
	mn := mocknet.New()

	nodes := make([]*core.IpfsNode, n)
	apis := make([]fxiface.CoreAPI, n)

	for i := 0; i < n; i++ {
		var ident config.Identity
		if fullIdentity {
			sk, pk, err := crypto.GenerateKeyPair(crypto.RSA, 2048)
			if err != nil {
				return nil, err
			}

			id, err := peer.IDFromPublicKey(pk)
			if err != nil {
				return nil, err
			}

			kbytes, err := crypto.MarshalPrivateKey(sk)
			if err != nil {
				return nil, err
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
			return nil, err
		}
		nodes[i] = node

		capi, err := coreapi.NewCoreAPI(node)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		apis[i], err = fxfscore.NewCoreAPI(node, capi.(*coreapi.CoreAPI))
		if err != nil {
			return nil, err
		}
	}

	err := mn.LinkAll()
	if err != nil {
		return nil, err
	}

	bsinf := bootstrap.BootstrapConfigWithPeers(
		[]peer.AddrInfo{
			nodes[0].Peerstore.PeerInfo(nodes[0].Identity),
		},
	)

	for _, n := range nodes[1:] {
		if err := n.Bootstrap(bsinf); err != nil {
			return nil, err
		}
	}

	return apis, nil
}

func TestAPI(t *testing.T) {
	r := &repo.Mock{
		C: config.Config{
			Identity: config.Identity{
				PeerID: testPeerID, // required by offline node
			},
		},
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
	}
	node, err := core.NewNode(context.Background(), &core.BuildCfg{Repo: r})
	if err != nil {
		t.Fatal(err)
	}

	capi, err := coreapi.NewCoreAPI(node)
	if err != nil {
		t.Fatal(err)
	}

	fapi, err := fxfscore.NewCoreAPI(node, capi)
	if err != nil {
		t.Fatal(err)
	}

	puDir := files.NewMapDirectory(map[string]files.Node{
		"DID": files.NewBytesFile([]byte("some data")),
	})
	prDir := files.NewMapDirectory(map[string]files.Node{
		"DID": pfs.NewEncodedFileFromNode(files.NewBytesFile([]byte("some other data")), []byte("JWEEEEEEE")),
	})

	rootDir := files.NewMapDirectory(map[string]files.Node{
		"public":  puDir,
		"private": prDir,
	})

	puResolved, err := fapi.PublicFS().Add(context.Background(), puDir)
	if err != nil {
		t.Fatal("error in adding puDir", err)
	}

	prResolved, err := fapi.PrivateFS().Add(context.Background(), prDir)
	if err != nil {
		t.Fatal("error in adding prDir", err)
	}

	rootResolved, err := fapi.PublicFS().Add(context.Background(), rootDir)
	if err != nil {
		t.Fatal("error in adding rootDir", err)
	}

	pn, err := fapi.PublicFS().Get(context.Background(), path.New("/ipfs/"+puResolved.Cid().String()+"/DID"))
	if err != nil {
		t.Fatal(err)
	}
	rn, err := fapi.PrivateFS().Get(context.Background(), path.New("/ipfs/"+prResolved.Cid().String()+"/DID"))
	if err != nil {
		t.Fatal(err)
	}
	pt := rn.(pfs.EncodedFile)
	bpt, err := io.ReadAll(pt)
	pnn, err := io.ReadAll(pn.(files.File))
	fmt.Println(puResolved)
	fmt.Println(prResolved)
	fmt.Println(rootResolved)
	fmt.Println(pt.JWE())
	fmt.Println(pnn)
	fmt.Println(bpt)
}
