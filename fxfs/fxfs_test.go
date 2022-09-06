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
	iface "github.com/ipfs/interface-go-ipfs-core"
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

	ctx := context.Background()

	node, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
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
		"DIDP": pfs.NewEncodedFileFromNode(files.NewBytesFile([]byte("some other data")), []byte("JWEEEEEEE")),
	})

	rootDir := files.NewMapDirectory(map[string]files.Node{
		"public":  puDir,
		"private": prDir,
	})

	puResolved, err := fapi.PublicFS().Add(ctx, puDir)
	if err != nil {
		t.Fatal("error in adding puDir", err)
	}

	prResolved, err := fapi.PrivateFS().Add(ctx, prDir)
	if err != nil {
		t.Fatal("error in adding prDir", err)
	}

	rootResolved, err := fapi.PublicFS().Add(ctx, rootDir)
	if err != nil {
		t.Fatal("error in adding rootDir", err)
	}

	didp := path.New("/ipfs/" + puResolved.Cid().String() + "/DID")
	didpp := path.New("/ipfs/" + prResolved.Cid().String() + "/DIDP")

	pn, err := fapi.PublicFS().Get(ctx, didp)
	if err != nil {
		t.Fatal(err)
	}
	rn, err := fapi.PrivateFS().Get(ctx, didpp)
	if err != nil {
		t.Fatal(err)
	}
	pt := rn.(pfs.EncodedFile)
	bpt, err := io.ReadAll(pt)
	pnn, err := io.ReadAll(pn.(files.File))
	t.Log(puResolved)
	t.Log(prResolved)
	t.Log(rootResolved)
	t.Log(pt.JWE())
	t.Log(pnn)
	t.Log(bpt)

	// "Testing PublicFS Ls
	pudirp := path.New("/ipfs/" + puResolved.Cid().String())
	lsres, err := fapi.PublicFS().Ls(ctx, pudirp)
	did := <-lsres

	if did.Name != "DID" || did.Size != 9 {
		t.Log("The file inside public directory is not what expected")
		t.Fail()
	}

	x := <-lsres
	if x.Cid.Defined() || x.Name != "" || x.Size != 0 || x.Type != iface.TUnknown {
		t.Log("More files in directory than expected")
		t.Fail()
	}

	// Testing PrivateFS Ls
	prdirp := path.New("/ipfs/" + prResolved.Cid().String())
	plsres, err := fapi.PrivateFS().Ls(ctx, prdirp)
	pdid := <-plsres

	//@TODO add size check for pdid
	if pdid.Name != "DIDP" {
		t.Log("The file inside private directory is not what expected")
		t.Fail()
	}

	x = <-lsres
	if x.Cid.Defined() || x.Name != "" || x.Size != 0 || x.Type != iface.TUnknown {
		t.Log("More files in directory than expected")
		t.Fail()
	}
}
