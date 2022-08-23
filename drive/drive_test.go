package drive

import (
	"context"
	"fmt"
	"io"
	"testing"

	fxfscore "github.com/functionland/go-fula/fxfs/core/api"
	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core/path"
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

	x, err := ps.MkDir("/photos/summer", MkDirOpts{recursive: false})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("new root /photos/summer", x)

	xxx, err := ps.MkDir("/photos/winter", MkDirOpts{recursive: false})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("new root /photos/winter", xxx)

	xx, err := ps.MkDir("/photos/summer/q1", MkDirOpts{recursive: false})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("new root /photos/summer/q1", xx)

}

func TestWriteFile(t *testing.T) {
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

	_, err = ps.MkDir("/photos/mehdi", MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}

	f := files.NewBytesFile([]byte("some data to test WriteFile method"))
	_, err = ps.WriteFile("/photos/mehdi/data.txt", f, WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fn, err := ps.api.PublicFS().Get(ps.ctx, path.New("/ipfs/"+ps.rootCid+"/photos/mehdi/data.txt"))
	if err != nil {
		t.Fatal(err)
	}

	fb, err := io.ReadAll(fn.(files.File))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(fb))
}

func TestReadFile(t *testing.T) {
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

	f := files.NewBytesFile([]byte("some data to test ReadFile method"))
	_, err = ps.WriteFile("/photos/data.txt", f, WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fn, err := ps.ReadFile("/photos/data.txt", ReadFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fb, err := io.ReadAll(fn.(files.File))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(fb))
}

func TestDeleteFile(t *testing.T) {
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

	f := files.NewBytesFile([]byte("some data to test DeleteFile method"))
	_, err = ps.WriteFile("/photos/data.txt", f, WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fn, err := ps.ReadFile("/photos/data.txt", ReadFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fb, err := io.ReadAll(fn.(files.File))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(fb))

	_, err = ps.DeleteFile("/photos/data.txt", DeleteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fn, err = ps.ReadFile("/photos/data.txt", ReadFileOpts{})
	if err == nil {
		t.Fatal("file must not exist after deletion", err)
	}
}

func TestListEntries(t *testing.T) {
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

	f := files.NewBytesFile([]byte("some data to test ListEntries method"))
	_, err = ps.WriteFile("/photos/data.txt", f, WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ls, err := ps.ListEntries("/", ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("ls for /")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		fmt.Printf("%+v \n", entry)
	}

	ls, err = ps.ListEntries("/photos", ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("ls for /photos")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		fmt.Printf("%+v \n", entry)
	}

	_, err = ps.MkDir("/photos/summer", MkDirOpts{recursive: false})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ps.MkDir("/photos/winter", MkDirOpts{recursive: false})
	if err != nil {
		t.Fatal(err)
	}

	ls, err = ps.ListEntries("/photos", ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("ls for /photos")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		fmt.Printf("%+v \n", entry)
	}

}
