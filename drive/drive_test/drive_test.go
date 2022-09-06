package drive_test

import (
	"context"
	"io"
	"testing"

	fdrive "github.com/functionland/go-fula/drive"
	fxfscore "github.com/functionland/go-fula/fxfs/core/api"
	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	pfs "github.com/functionland/go-fula/fxfs/core/pfs"
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

func newDrive(userDID string) (fdrive.UserDrive, fxiface.CoreAPI) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	ds := fdrive.NewDriveStore()

	ud, err := ds.ResolveCreate(userDID)
	if err != nil {
		panic(err)
	}

	ud.Publish(ctx, fapi)

	return ud, fapi
}

func TestPublishDrive(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"
	ds := fdrive.NewDriveStore()

	ud, err := ds.ResolveCreate(testUserDID)
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	t.Logf("%+v", ud)
}

func TestDriveSpace(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"
	ds := fdrive.NewDriveStore()

	ud, err := ds.ResolveCreate(testUserDID)
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	ps, err := ud.PublicSpace(ctx, fapi)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ps.MkDir("/data", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}

	f := files.NewBytesFile([]byte("some data to test ListEntries method"))
	_, err = ps.WriteFile("/data/test.txt", f, fdrive.WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ls, err := ps.ListEntries("/", fdrive.ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("ls for /")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		t.Logf("%+v \n", entry)
	}

	ls, err = ps.ListEntries("/data", fdrive.ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n\nls for /data")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		t.Logf("%+v \n", entry)
	}

	_, err = ps.MkDir("/data/summer", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ls, err = ps.ListEntries("/data", fdrive.ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n\nls for /data")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		t.Logf("%+v \n", entry)
	}

	_, err = ps.DeleteFile("/data/test.txt", fdrive.DeleteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ls, err = ps.ListEntries("/data", fdrive.ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("\n\nls for /data")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		t.Logf("%+v \n", entry)
	}
}

func TestMkDir(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"
	ds := fdrive.NewDriveStore()

	ud, err := ds.ResolveCreate(testUserDID)
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	ps, err := ud.PublicSpace(ctx, fapi)
	if err != nil {
		t.Fatal(err)
	}

	y, err := ps.MkDir("/photos", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("new root /photos", y)

	x, err := ps.MkDir("/photos/summer", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("new root /photos/summer", x)

	xxx, err := ps.MkDir("/photos/winter", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("new root /photos/winter", xxx)

	xx, err := ps.MkDir("/photos/summer/q1", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("new root /photos/summer/q1", xx)

}

func TestWriteFile(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"
	ds := fdrive.NewDriveStore()

	ud, err := ds.ResolveCreate(testUserDID)
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	ps, err := ud.PublicSpace(ctx, fapi)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ps.MkDir("/photos", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}

	f := files.NewBytesFile([]byte("some data to test WriteFile method"))
	_, err = ps.WriteFile("/photos/data.txt", f, fdrive.WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fn, err := ps.Api.PublicFS().Get(ps.Ctx, path.New("/ipfs/"+ps.RootCid+"/photos/data.txt"))
	if err != nil {
		t.Fatal(err)
	}

	fb, err := io.ReadAll(fn.(files.File))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(fb))
}

func TestReadFile(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"
	ds := fdrive.NewDriveStore()

	ud, err := ds.ResolveCreate(testUserDID)
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	ps, err := ud.PublicSpace(ctx, fapi)
	if err != nil {
		t.Fatal(err)
	}

	f := files.NewBytesFile([]byte("some data to test ReadFile method"))
	_, err = ps.WriteFile("/data.txt", f, fdrive.WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fn, err := ps.ReadFile("/data.txt", fdrive.ReadFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fb, err := io.ReadAll(fn.(files.File))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(fb))
}

func TestDeleteFile(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"
	ds := fdrive.NewDriveStore()

	ud, err := ds.ResolveCreate(testUserDID)
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	ps, err := ud.PublicSpace(ctx, fapi)
	if err != nil {
		t.Fatal(err)
	}

	f := files.NewBytesFile([]byte("some data to test DeleteFile method"))
	_, err = ps.WriteFile("/data.txt", f, fdrive.WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fn, err := ps.ReadFile("/data.txt", fdrive.ReadFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fb, err := io.ReadAll(fn.(files.File))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(fb))

	_, err = ps.DeleteFile("/data.txt", fdrive.DeleteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fn, err = ps.ReadFile("/data.txt", fdrive.ReadFileOpts{})
	if err == nil {
		t.Fatal("file must not exist after deletion", err)
	}
}

func TestListEntries(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"
	ds := fdrive.NewDriveStore()

	ud, err := ds.ResolveCreate(testUserDID)
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	ps, err := ud.PublicSpace(ctx, fapi)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ps.MkDir("/photos", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}

	f := files.NewBytesFile([]byte("some data to test ListEntries method"))
	_, err = ps.WriteFile("/photos/data.txt", f, fdrive.WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ls, err := ps.ListEntries("/", fdrive.ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("ls for /")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		t.Logf("%+v \n", entry)
	}

	ls, err = ps.ListEntries("/photos", fdrive.ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("ls for /photos")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		t.Logf("%+v \n", entry)
	}

	_, err = ps.MkDir("/photos/summer", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = ps.MkDir("/photos/winter", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ls, err = ps.ListEntries("/photos", fdrive.ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("ls for /photos")
	for entry := range ls {
		if entry.Err != nil {
			t.Fatal(entry.Err)
		}
		t.Logf("%+v \n", entry)
	}

}

func TestDrivePrivateSpace(t *testing.T) {
	fapi := newFxFsCoreAPI()
	ctx := context.Background()
	testUserDID := "did:fula:resolves-to-mehdi-2"
	ds := fdrive.NewDriveStore()

	ud, err := ds.ResolveCreate(testUserDID)
	if err != nil {
		t.Fatal(err)
	}

	ud.Publish(ctx, fapi)

	ps, err := ud.PrivateSpace(ctx, fapi)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ps.MkDir("/photos", fdrive.MkDirOpts{})
	if err != nil {
		t.Fatal(err)
	}

	testFileContent := "some data to test DrivePrivateSpace"
	testJWEContent := "JWE FOR DrivePrivateSpace"
	f := files.NewBytesFile([]byte(testFileContent))
	_, err = ps.WriteFile("/photos/data.txt", f, []byte(testJWEContent), fdrive.WriteFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	ef, err := ps.ReadFile("/photos/data.txt", fdrive.ReadFileOpts{})
	if err != nil {
		t.Fatal(err)
	}

	fb, err := io.ReadAll(ef.(pfs.EncodedFile))
	if err != nil {
		t.Fatal(err)
	}

	if string(fb) != testFileContent {
		t.Fatal("ReadFile returned wrong content for file")
	}

	if string(ef.JWE()) != testJWEContent {
		t.Fatal("ReadFile JWE() returned wrong content for file's jwe")
	}
}
