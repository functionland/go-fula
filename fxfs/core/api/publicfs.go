package api

import (
	"context"

	files "github.com/ipfs/go-ipfs-files"
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
