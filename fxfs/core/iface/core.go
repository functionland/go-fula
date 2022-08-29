package iface

import (
	"context"

	files "github.com/ipfs/go-ipfs-files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

// Interfaces for fxfs

type CoreAPI interface {
	PrivateFS() PrivateFS

	PublicFS() PublicFS
}

type PrivateFS interface {
	Add(context.Context, files.Node, ...options.UnixfsAddOption) (path.Resolved, error)

	Get(context.Context, path.Path) (files.Node, error)

	Ls(ctx context.Context, p path.Path, opts ...options.UnixfsLsOption) (<-chan iface.DirEntry, error)
}

type PublicFS interface {
	Add(context.Context, files.Node, ...options.UnixfsAddOption) (path.Resolved, error)

	Get(context.Context, path.Path) (files.Node, error)

	Ls(ctx context.Context, p path.Path, opts ...options.UnixfsLsOption) (<-chan iface.DirEntry, error)
}
