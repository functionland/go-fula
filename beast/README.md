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

## Running a Kubo instance
In order to run a Kubo instance with the beast plugin installed, you can use the Dockerfile provided in `/docker` directory.

Since the beast plugin imports File Protocol for handling the stream, you need to build the docker image in a context that includes both beast and file protocol.

First create your `go.mod` file. Inside the `go-fula/beast` directory:
```
go mod tidy
```

Next, you need to make the beast plugin. If you look at the Make instructions, you can see that it needs to put make results in a directory named `kubo` which is a sibling to `go-fula` directory:
```bash
cd ../../ #parent directory for go-fula
git clone https://github.com/ipfs/kubo.git
cd go-fula/beast
make install
```

Now that you have the plugin compiled, you can proceed and build a docker image containing Kubo and the beast plugin:
```bash
cd ../ #go-fula directory
docker build -t go-fula -f beast/docker/Dockerfile .
```

If everything goes right, you can verify your image being built by looking into docker images list:
```bash
docker images
```
you should see a line indicating that you have a `go-fula` image on your host. Something like this:
```
go-fula                        latest    efec5df92839   About an hour ago   94.8MB
```
The final step is to run your recently built Kubo image, go ahead and do that with docker command:
```bash
docker run -p 4001:4001 go-fula
```

You should see Kubo getting started and listening on port 4001. Also, it outputs you peer identity which you can use to connect to this IPFS instance.

If you want to debug your Kubo instance, you can set the log level using environment variables:
```
docker run -p 4001:4001 -e GOLOG_LOG_LEVEL="error,fula:filePL=debug,plugin/beast=debug,p2p-holepunch=debug" go-fula
```

## License

MIT
