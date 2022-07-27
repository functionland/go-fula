# Beast

[![GoDoc](https://godoc.org/github.com/ipfs/go-ipfs-example-plugin?status.svg)](https://godoc.org/github.com/ipfs/go-ipfs-example-plugin)

> Inject fula protocol to an IPFS node.

This repository contains a plugin that inject fula protocols and services in to an IPFS deamon.


**NOTE 1:** Plugins only work on Linux and MacOS at the moment. You can track the progress of this issue here: https://github.com/golang/go/issues/19282

**NOTE 2:** This plugin exists as an *example* and a starting point for new plugins. It isn't particularly useful by itself.

## Building and Installing

You can *build* the example plugin by running `make build`. You can then install it into your local IPFS repo by running `make install`.

Plugins need to be built against the correct version of go-ipfs. This package generally tracks the latest go-ipfs release but if you need to build against a different version, please set the `IPFS_VERSION` environment variable.

You can set `IPFS_VERSION` to:

* `vX.Y.Z` to build against that version of IPFS.
* `$commit` or `$branch` to build against a specific go-ipfs commit or branch.
   * Note: if building against a commit or branch make sure to build that commit/branch using the -trimpath flag. For example getting the binary via `go get -trimpath github.com/ipfs/go-ipfs/cmd/ipfs@COMMIT`
* `/absolute/path/to/source` to build against a specific go-ipfs checkout.

To update the go-ipfs, run:

```bash
> make go.mod IPFS_VERSION=version
```

## License

MIT
