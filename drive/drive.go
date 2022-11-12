package drive

import (
	"context"

	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	"github.com/functionland/go-fula/fxfs/core/pfs"
	files "github.com/ipfs/go-ipfs-files"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula-drive")

// UserDrive holds necessary info to identify a user's drive on the network
// It holds info about user's ID, root cids for each space and a DriveStore instance
type UserDrive struct {
	UserDID         string
	PrivateSpaceCid string
	PublicSpaceCid  string
	Dirs            map[string]string
	ds              DriveStore
}

// NewDrive create a new null Drive, the only field set is the UserDID, other fields are initialized by first call to Publish
func NewDrive(userDID string, ds DriveStore) UserDrive {
	return UserDrive{
		UserDID: userDID,
		ds:      ds,
	}
}

// IsNull check if a drive is null
func (ud *UserDrive) IsNull() bool {
	return ud.PrivateSpaceCid == "" && ud.PublicSpaceCid == "" && ud.Dirs == nil
}

// PublicSpace create a DrivePublicSpace struct. PublicSpace creates a Directory instance by getting the root cid of the drive using FS API
func (ud *UserDrive) PublicSpace(ctx context.Context, api fxiface.CoreAPI) (*DrivePublicSpace, error) {
	rpath := makePath(ud.PublicSpaceCid)
	rootDir, err := api.PublicFS().Get(ctx, rpath)
	if err != nil {
		log.Error("error in getting root dir for private space")
		return nil, err
	}

	return &DrivePublicSpace{DriveSpace: DriveSpace{
		Ctx:       ctx,
		Api:       api,
		SpaceType: PublicDriveSpaceType,
		RootCid:   ud.PublicSpaceCid,
		RootDir:   rootDir.(files.Directory),
		Drive:     ud}}, err
}

// PrivateSpace create a DrivePrivateSpace struct. PrivateSpace creates a Directory instance by getting the root cid of the drive using FS API
func (ud *UserDrive) PrivateSpace(ctx context.Context, api fxiface.CoreAPI) (*DrivePrivateSpace, error) {
	rpath := makePath(ud.PrivateSpaceCid)
	rootDir, err := api.PrivateFS().Get(ctx, rpath)
	if err != nil {
		log.Error("error in getting root dir for private space")
		return nil, err
	}

	return &DrivePrivateSpace{DriveSpace: DriveSpace{
		Ctx:       ctx,
		Api:       api,
		SpaceType: PrivateDriveSpaceType,
		RootCid:   ud.PrivateSpaceCid,
		RootDir:   rootDir.(files.Directory),
		Drive:     ud}}, err
}

// Publish a Drive
// If the drive is null, it will create two directories for Private and Public space, stores a file containing UserDID in them
// Publish uses DriveStore to update the drive reference
func (ud *UserDrive) Publish(ctx context.Context, api fxiface.CoreAPI) error {
	if ud.IsNull() {
		puDir := files.NewMapDirectory(map[string]files.Node{
			"DID": files.NewBytesFile([]byte(ud.UserDID)),
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

		err = ud.ds.Update(*ud)
		if err != nil {
			log.Error("error in putting drive into store", err)
			return err
		}
	} else {
		err := ud.ds.Update(*ud)
		if err != nil {
			log.Error("error in publishing drive", err)
			return err
		}
	}

	return nil
}
