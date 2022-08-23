package drive

import (
	"context"
	"errors"
	"fmt"
	"io"

	logging "github.com/ipfs/go-log"

	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	"github.com/functionland/go-fula/fxfs/core/pfs"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/core/coreapi"
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
}

type AddOpts struct {
	parents bool
}

func NewDrive(userDID string) *UserDrive {
	return &UserDrive{UserDID: userDID, PrivateSpaceCid: "", PublicSpaceCid: "", Dirs: nil}
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

func (ud *UserDrive) SpaceDirCid(space DRIVE_SPACE_TYPE) (string, error) {
	switch space {
	case PUBLIC_DRIVE_SPACE_TYPE:
		return ud.PublicSpaceCid, nil
	case PRIVATE_DRIVE_SPACE_TYPE:
		return ud.PrivateSpaceCid, nil
	default:
		return "", errors.New("invalid space type provided")
	}
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

		rootDir := files.NewMapDirectory(map[string]files.Node{
			"public":  puDir,
			"private": prDir,
		})

		puResolved, err := api.PublicFS().Add(ctx, puDir)
		if err != nil {
			fmt.Println("error in adding puDir", err)
			return err
		}

		prResolved, err := api.PrivateFS().Add(ctx, prDir)
		if err != nil {
			fmt.Println("error in adding prDir", err)

			// @TODO remove pin for puDir
			return err
		}

		rootResolved, err := api.PublicFS().Add(ctx, rootDir)
		if err != nil {
			fmt.Println("error in adding rootDir", err)

			// @TODO remove pin for puDir
			return err
		}

		ud.PrivateSpaceCid = prResolved.Cid().String()
		ud.PublicSpaceCid = puResolved.Cid().String()

		fmt.Println("rootResolved", rootResolved.Cid().String())
		fmt.Println("puResolved", puResolved.Cid().String())
		fmt.Println("prResolved", prResolved.Cid().String())

		err = PutDrive(*ud)
		if err != nil {
			fmt.Println("error in putting drive into store", err)
			return err
		}
	} else {
		err := PutDrive(*ud)
		if err != nil {
			fmt.Println("error in publishing drive", err)
			return err
		}
	}

	return nil
}

func (ud *UserDrive) AddFile(ctx context.Context, space DRIVE_SPACE_TYPE, writePath path.Path, reader io.Reader, api *coreapi.CoreAPI, opts ...AddOpts) error {
	spaceDirCid, err := ud.SpaceDirCid(space)
	if err != nil {
		fmt.Println(err)
		return err
	}

	spaceDirPath := path.New("/ipfs/" + spaceDirCid)

	spaceDirNode, err := api.Unixfs().Get(ctx, spaceDirPath)
	if err != nil {
		fmt.Println("error in getting space dir", err)
		return err
	}

	switch spaceDirNode.(type) {
	case files.Directory:
		fmt.Println(fmt.Sprintf("found %s directory for user %s", space, ud.UserDID))
		fmt.Println(fmt.Sprintf("dir cid: %s", spaceDirCid))
	case files.File:
		fmt.Println("error: expected directory found file")
		return err
	default:
		fmt.Println("error: spaceDir node is neither directory nor file")
		return err
	}

	it := spaceDirNode.(files.Directory).Entries()
	for it.Next() {
		fmt.Println(fmt.Sprintf("Name: %s \nNode: %+v", it.Name(), it.Node()))
	}

	return nil

}

func LoadDrive(ctx context.Context, userDID string, api fxiface.CoreAPI, options map[string]interface{}) (*UserDrive, error) {

	userDrive, err := ResolveDrive(userDID)
	if err != nil {
		if userDrive == nil {
			fmt.Println("error in drive resolution", err)
			return nil, err
		}

		// drive not found creating a new one
		err := userDrive.Publish(ctx, api)
		if err != nil {
			fmt.Println("error in publishing newly created drive", err)
			return nil, err
		}
	}

	return userDrive, nil
}
