package drive

import (
	"context"
	"strings"

	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

type DRIVE_SPACE_TYPE string

const (
	PUBLIC_DRIVE_SPACE_TYPE  DRIVE_SPACE_TYPE = "public"
	PRIVATE_DRIVE_SPACE_TYPE DRIVE_SPACE_TYPE = "private"
)

type DriveSpace struct {
	SpaceType DRIVE_SPACE_TYPE
	rootCid   string
	RootDir   files.Directory
}

// MkDir creates a new directory inside a DriveSpace given a path
func (ds *DriveSpace) MkDir(ctx context.Context, api fxiface.CoreAPI, p string, options MkDirOpts) (string, error) {
	newRoot, err := mkdirDAG(ds.RootDir, p)
	if err != nil {
		return "", err
	}

	n, err := api.PublicFS().Add(ctx, newRoot)
	if err != nil {
		return "", err
	}

	ds.rootCid = n.Cid().String()
	nRoot, err := api.PublicFS().Get(ctx, path.New("/ipfs/"+ds.rootCid))
	if err != nil {
		return "", err
	}
	ds.RootDir = nRoot.(files.Directory)

	return ds.rootCid, nil
}

func mkdirDAG(node files.Node, path string) (files.Node, error) {
	if files.ToFile(node) != nil {
		return node, nil
	}

	if files.ToDir(node) != nil {
		ps := PathSlice(path)
		dirname := ps[0]
		ps = ps[1:]

		entries := make([]files.DirEntry, 0)

		dit := node.(files.Directory).Entries()
		for dit.Next() {
			if dit.Name() == dirname {
				d, err := mkdirDAG(dit.Node(), strings.Join(ps, "/"))
				if err != nil {
					return nil, err
				}
				entries = append(entries, files.FileEntry(dit.Name(), d))
			} else {
				entries = append(entries, files.FileEntry(dit.Name(), dit.Node()))
			}
		}

		if len(ps) == 0 {
			entries = append(entries, files.FileEntry(dirname, files.NewMapDirectory(map[string]files.Node{})))
		}

		return files.NewSliceDirectory(entries), nil
	}

	return node, nil
}
