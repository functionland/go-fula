package drive

import (
	"context"
	"fmt"
	"testing"

	fxfscore "github.com/functionland/go-fula/fxfs/core/api"
	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/repo"
)

const testPeerID = "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe"

func newFxFsCoreAPI() fxiface.CoreAPI {
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
		panic(err)
	}

	capi, err := coreapi.NewCoreAPI(node)
	if err != nil {
		panic(err)
	}

	fapi, err := fxfscore.NewCoreAPI(node, capi)
	if err != nil {
		panic(err)
	}

	return fapi
}

func newDrive(userDID string) (*UserDrive, fxiface.CoreAPI) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()

	ud, err := LoadDrive(ctx, userDID, fapi, map[string]interface{}{})
	if err != nil {
		panic(err)
	}

	ud.Publish(ctx, fapi)

	return ud, fapi
}

func TestLoadDrive(t *testing.T) {

	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi"

	ud, err := LoadDrive(ctx, testUserDID, fapi, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}

	if ud.IsNull() {
		t.Fatal("LoadDrive returned an empty drive")
	}

	if ud.UserDID != testUserDID {
		t.Fatal("LoadDrive returned a drive with wrong UserDID")
	}

}

func TestPublishDrive(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"

	ud, err := LoadDrive(ctx, testUserDID, fapi, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	fmt.Printf("%+v", ud)
}

func TestDriveSpace(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-4"

	ud, err := LoadDrive(ctx, testUserDID, fapi, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	ps, err := ud.PublicSpace(ctx, fapi)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%+v", ps)
}

func TestMkDir(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"

	ud, err := LoadDrive(ctx, testUserDID, fapi, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	ps, err := ud.PublicSpace(ctx, fapi)
	if err != nil {
		t.Fatal(err)
	}

	x, err := ps.MkDir(ctx, fapi, "/photos/summer", MkDirOpts{recursive: false})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("new root /photos/summer", x)

	xxx, err := ps.MkDir(ctx, fapi, "/photos/winter", MkDirOpts{recursive: false})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("new root /photos/winter", xxx)

	xx, err := ps.MkDir(ctx, fapi, "/photos/summer/q1", MkDirOpts{recursive: false})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("new root /photos/summer/q1", xx)

}
