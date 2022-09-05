package drive

import (
	"context"
	"errors"
	"strings"

	fxiface "github.com/functionland/go-fula/fxfs/core/iface"
	"github.com/functionland/go-fula/fxfs/core/pfs"
	files "github.com/ipfs/go-ipfs-files"
	ipfsifsce "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

type DRIVE_SPACE_TYPE string

const (
	PUBLIC_DRIVE_SPACE_TYPE  DRIVE_SPACE_TYPE = "public"
	PRIVATE_DRIVE_SPACE_TYPE DRIVE_SPACE_TYPE = "private"
)

type MkDirOpts struct {
	recursive bool
}

type WriteFileOpts struct {
	parents   bool
	overwrite bool
}

type ReadFileOpts struct {
}

type DeleteFileOpts struct {
}

type ListEntriesOpts struct {
}

type ListEntry struct {
	ipfsifsce.DirEntry
}

type DriveSpace struct {
	ctx       context.Context
	api       fxiface.CoreAPI
	SpaceType DRIVE_SPACE_TYPE
	rootCid   string
	RootDir   files.Directory
}

type DrivePublicSpace struct {
	DriveSpace
}

type DrivePrivateSpace struct {
	DriveSpace
}

func makePath(dirs ...string) path.Path {
	return path.Join(path.New("/ipfs"), dirs...)
}

// MkDir creates a new directory inside a DriveSpace given a path
func (ds *DriveSpace) MkDir(p string, options MkDirOpts) (string, error) {
	newRoot, err := mkdirDAG(ds.RootDir, p)
	if err != nil {
		return "", err
	}

	n, err := ds.api.PublicFS().Add(ds.ctx, newRoot)
	if err != nil {
		return "", err
	}

	ds.rootCid = n.Cid().String()
	nRoot, err := ds.api.PublicFS().Get(ds.ctx, makePath(ds.rootCid))
	if err != nil {
		return "", err
	}
	ds.RootDir = nRoot.(files.Directory)

	return ds.rootCid, nil
}

// Writes a file into drive at a given location (in public space).
func (ds *DrivePublicSpace) WriteFile(p string, file files.File, options WriteFileOpts) (string, error) {
	// @TODO handle options.parents = true (create the directories in the path)

	newRoot, err := writefileDAG(ds.RootDir, p, file)
	if err != nil {
		return "", err
	}

	n, err := ds.api.PublicFS().Add(ds.ctx, newRoot)
	if err != nil {
		return "", err
	}

	ds.rootCid = n.Cid().String()
	nRoot, err := ds.api.PublicFS().Get(ds.ctx, makePath(ds.rootCid))
	if err != nil {
		return "", err
	}
	ds.RootDir = nRoot.(files.Directory)

	return ds.rootCid, nil
}

// Writes a file into private space in a drive at a given location, it takes a byte array called JWE in addition to DrivePublicSpace.WriteFile
func (ds *DrivePrivateSpace) WriteFile(p string, file files.File, jwe []byte, options WriteFileOpts) (string, error) {
	// @TODO handle options.parents = true (create the not-existing directories in the path)

	encFile := pfs.NewEncodedFileFromNode(file, jwe)

	newRoot, err := writefileDAG(ds.RootDir, p, encFile)
	if err != nil {
		return "", err
	}

	n, err := ds.api.PrivateFS().Add(ds.ctx, newRoot)
	if err != nil {
		return "", err
	}

	ds.rootCid = n.Cid().String()
	nRoot, err := ds.api.PrivateFS().Get(ds.ctx, makePath(ds.rootCid))
	if err != nil {
		return "", err
	}
	ds.RootDir = nRoot.(files.Directory)

	return ds.rootCid, nil
}

// Reads a file from the drive at a given location
func (ds *DrivePublicSpace) ReadFile(p string, options ReadFileOpts) (files.File, error) {
	file, err := ds.api.PublicFS().Get(ds.ctx, makePath(ds.rootCid, p))
	if err != nil {
		return nil, err
	}

	if files.ToFile(file) == nil {
		return nil, errors.New("specified path does not point to a file")
	}

	return file.(files.File), nil
}

// Reads a file from private space in a drive at a give location, it returns and additional JWE byte array
func (ds *DrivePrivateSpace) ReadFile(p string, options ReadFileOpts) (pfs.EncodedFile, error) {
	file, err := ds.api.PrivateFS().Get(ds.ctx, makePath(ds.rootCid, p))
	if err != nil {
		return nil, err
	}

	if files.ToFile(file) == nil {
		return nil, errors.New("specified path does not point to a file")
	}

	return file.(pfs.EncodedFile), nil
}

// Deletes a file at a given location on the drive
func (ds *DriveSpace) DeleteFile(p string, options DeleteFileOpts) (string, error) {
	newRoot, err := deletefileDAG(ds.RootDir, p)
	if err != nil {
		return "", err
	}

	n, err := ds.api.PublicFS().Add(ds.ctx, newRoot)
	if err != nil {
		return "", err
	}

	ds.rootCid = n.Cid().String()
	nRoot, err := ds.api.PublicFS().Get(ds.ctx, makePath(ds.rootCid))
	if err != nil {
		return "", err
	}
	ds.RootDir = nRoot.(files.Directory)

	return ds.rootCid, nil
}

// List all of entries in a path
func (ds *DriveSpace) ListEntries(p string, options ListEntriesOpts) (<-chan ipfsifsce.DirEntry, error) {
	ls, err := ds.api.PublicFS().Ls(ds.ctx, makePath(ds.rootCid, p))
	if err != nil {
		return nil, err
	}

	return ls, nil
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

func writefileDAG(node files.Node, path string, file files.File) (files.Node, error) {
	if files.ToFile(node) != nil {
		// @TODO this edge case needs more consideration. is it even possible for this to happen?
		return node, nil
	}

	if files.ToDir(node) != nil {
		ps := PathSlice(path)
		dirname := ps[0]
		ps = ps[1:]

		if len(ps) == 0 {
			entries := make([]files.DirEntry, 0)
			dit := node.(files.Directory).Entries()
			for dit.Next() {
				if dit.Name() == dirname {
					// the file already exists
					// @TODO handle options.overwrite (whether replace the existing file or not)
					//

				} else {
					entries = append(entries, files.FileEntry(dit.Name(), dit.Node()))
				}
			}

			entries = append(entries, files.FileEntry(dirname, file))

			return files.NewSliceDirectory(entries), nil
		}

		entries := make([]files.DirEntry, 0)
		dit := node.(files.Directory).Entries()
		for dit.Next() {
			if dit.Name() == dirname {
				d, err := writefileDAG(dit.Node(), strings.Join(ps, "/"), file)
				if err != nil {
					return nil, err
				}
				entries = append(entries, files.FileEntry(dit.Name(), d))
			} else {
				entries = append(entries, files.FileEntry(dit.Name(), dit.Node()))
			}
		}
		return files.NewSliceDirectory(entries), nil
	}

	return node, nil
}

func deletefileDAG(node files.Node, path string) (files.Node, error) {
	if files.ToFile(node) != nil {
		// @TODO this edge case needs more consideration. is it even possible for this to happen?
		return node, nil
	}

	if files.ToDir(node) != nil {
		ps := PathSlice(path)
		dirname := ps[0]
		ps = ps[1:]

		if len(ps) == 0 {
			entries := make([]files.DirEntry, 0)
			dit := node.(files.Directory).Entries()
			exists := false
			for dit.Next() {
				if dit.Name() == dirname {
					exists = true
				} else {
					entries = append(entries, files.FileEntry(dit.Name(), dit.Node()))
				}
			}

			if !exists {
				return nil, errors.New("file not found")
			}

			return files.NewSliceDirectory(entries), nil
		}

		entries := make([]files.DirEntry, 0)
		dit := node.(files.Directory).Entries()
		for dit.Next() {
			if dit.Name() == dirname {
				d, err := deletefileDAG(dit.Node(), strings.Join(ps, "/"))
				if err != nil {
					return nil, err
				}
				entries = append(entries, files.FileEntry(dit.Name(), d))
			} else {
				entries = append(entries, files.FileEntry(dit.Name(), dit.Node()))
			}
		}
		return files.NewSliceDirectory(entries), nil
	}

	return node, nil
}
