package api

import (
	"context"

	files "github.com/ipfs/go-ipfs-files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
)

type PublicAPI CoreAPI

func (api *PublicAPI) Add(ctx context.Context, files files.Node, opts ...options.UnixfsAddOption) (path.Resolved, error) {
	return api.nc.Unixfs().Add(ctx, files, opts...)
}

func (api *PublicAPI) Get(ctx context.Context, p path.Path) (files.Node, error) {
	return api.nc.Unixfs().Get(ctx, p)
}

func (api *PublicAPI) Ls(ctx context.Context, p path.Path, opts ...options.UnixfsLsOption) (<-chan iface.DirEntry, error) {
	return api.nc.Unixfs().Ls(ctx, p, opts...)
}
