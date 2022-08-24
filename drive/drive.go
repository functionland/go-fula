package drive

import (
	"context"
	"fmt"

	logging "github.com/ipfs/go-log"

	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	"github.com/functionland/go-fula/fxfs/core/pfs"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

var log = logging.Logger("fula-drive")

type Drives struct {
	Drives []UserDrive `json:"drives"`
}

type UserDrive struct {
	UserDID         string
	PrivateSpaceCid string
	PublicSpaceCid  string
	Dirs            map[string]string
	ds              DriveStore
}

func NewDrive(userDID string, ds DriveStore) UserDrive {
	return UserDrive{UserDID: userDID,
		PrivateSpaceCid: "",
		PublicSpaceCid:  "",
		Dirs:            nil,
		ds:              ds,
	}
}

func (ud *UserDrive) IsNull() bool {
	return ud.PrivateSpaceCid == "" && ud.PublicSpaceCid == "" && ud.Dirs == nil
}

func (ud *UserDrive) PublicSpace(ctx context.Context, api fxiface.CoreAPI) (*DriveSpace, error) {
	rpath := path.New("/ipfs/" + ud.PublicSpaceCid)
	rootDir, err := api.PublicFS().Get(ctx, rpath)
	if err != nil {
		log.Error("error in getting root dir for private space")
		return nil, err
	}

	return &DriveSpace{
		ctx:       ctx,
		api:       api,
		SpaceType: PUBLIC_DRIVE_SPACE_TYPE,
		rootCid:   ud.PublicSpaceCid,
		RootDir:   rootDir.(files.Directory)}, err
}

func (ud *UserDrive) PrivateSpace(ctx context.Context, api fxiface.CoreAPI) (*DriveSpace, error) {
	rpath := path.New("/ipfs/" + ud.PrivateSpaceCid)
	rootDir, err := api.PrivateFS().Get(ctx, rpath)
	if err != nil {
		log.Error("error in getting root dir for private space")
		return nil, err
	}

	return &DriveSpace{
		ctx:       ctx,
		api:       api,
		SpaceType: PRIVATE_DRIVE_SPACE_TYPE,
		rootCid:   ud.PrivateSpaceCid,
		RootDir:   rootDir.(files.Directory)}, err
}

func (ud *UserDrive) Publish(ctx context.Context, api fxiface.CoreAPI) error {
	if ud.IsNull() {
		puDir := files.NewMapDirectory(map[string]files.Node{
			"photos": files.NewMapDirectory(map[string]files.Node{
				"DID": files.NewBytesFile([]byte(ud.UserDID)),
			}),
		})
		prDir := files.NewMapDirectory(map[string]files.Node{
			"DIDP": pfs.NewEncodedFileFromNode(files.NewBytesFile([]byte(ud.UserDID)), []byte("JWE FOR FILE")),
		})

		puResolved, err := api.PublicFS().Add(ctx, puDir)
		if err != nil {
			log.Error("error in adding puDir", err)
			return err
		}

		prResolved, err := api.PrivateFS().Add(ctx, prDir)
		if err != nil {
			log.Error("error in adding prDir", err)

			// @TODO remove pin for puDir
			return err
		}

		ud.PrivateSpaceCid = prResolved.Cid().String()
		ud.PublicSpaceCid = puResolved.Cid().String()
		fmt.Println("puResolved", puResolved.Cid().String())
		fmt.Println("prResolved", prResolved.Cid().String())

		err = ud.ds.Update(*ud)
		if err != nil {
			log.Error("error in putting drive into store", err)
			return err
		}
	} else {
		err := ud.ds.Put(*ud)
		if err != nil {
			log.Error("error in publishing drive", err)
			return err
		}
	}

	return nil
}
