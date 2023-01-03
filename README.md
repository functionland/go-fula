# box

[![Go Test](https://github.com/functionland/go-fula/actions/workflows/go-test.yml/badge.svg)](https://github.com/functionland/go-fula/actions/workflows/go-test.yml)

Client-server stack for Web3

[Intro blog](https://dev.to/fx/google-photos-open-source-alternative-with-react-native-80c#ending-big-techs-reign-by-building-opensource-p2p-apps)

You can see it in action in our Flagship App: [Fx Fotos](https://github.com/functionland/fx-fotos)

[![Blox server demo](https://gateway.pinata.cloud/ipfs/QmVd3eioLfp19hG1GnfgceBsbK16i39vsQ9Lxq35aAjxnY)](https://gateway.pinata.cloud/ipfs/QmWUjQczA5jHC3ibLq4y7CizVrebr1DTFRaTdJgFyxR5Nh)

## Motivation

There are currently two ways to interact with Web3 storage solutions:

1. Through a pinning service and a gateway: the advantage is that files are served through URLs, an app can then access the files with conventional methods, e.g. simply putting a picture in `<img src="gateway.example.com/Qm...">`. The disadvantage is that there is a subscription payment associated with pinning services. Also this is not really decentralized!
2. Turn the device to a full IPFS node: this model works beautifully in Brave desktop browser as an example, and makes sense for laptop and PC since they normally have large HDDs. It's much harder on mobile devices, however, biggest hurdle is to have Apple on board with the idea of relaxing file system access in iOS! Even if all goes well, a mobile device is NOT a good candidate for hosting the future Web! They get lost easily and are resource constrained (battery, memory).

[**blox**](https://github.com/functionland/BLOX) aims to address these issues by creating a third alternative: **Personal Server**

A personal server is a commodity hardware (PC, Raspberry Pi, etc.) that's kept *at home* vs. *in pocket*. It helps with actual decentralization, also saves money since people pay once for HDDs and own them forever, no monthly charge! From privacy perspective, it guarantees that data doesn't leave the premise unless user specifically wants to (e.g. sharing).

To achieve this, we are developing protocols to accommodate client-server programming with minimal effort on developer's side.

## Architecture

![box architecture](https://gateway.pinata.cloud/ipfs/QmNkoQfCKAzQetJKWfNtLioJf6FCxzqjoDT2KshDZfsJd3)

A react-native app talks with the server(s) by invoking APIs from `@functionland/react-native-fula` library. The Fula library abstracts away the protocols and `libp2p` connection, instead exposes APIs similar to MongoDB for data persistence and S3 for file storage.

The data gets encrypted on the client side using [WebNative Filesystem (WNFS)] (https://github.com/wnfs-wg/rs-wnfs) and then the encrypted Merkle DAG is transferred to the blox server using Graphsync.

The **blox** stack can provide backup guarantees by having the data pinned on multiple servers owned by the user. However, in cases that the user needs absolute assurance on data longevity, e.g. password records in a password manager app or scans of sensitive documents, the cids of encrypted data can be sent over at [Fula blockchain](https://github.com/functionland/sugarfunge-node) and other blox owners can back them up and get rewarded.

## Packages

| Name | Description |
| --- | --- |
| [blox](blox) | Blox provides the backend to receive the DAG created by fulamobile and store it |
| [mobile](mobile) | Interacts with WNFS to encrypt the data and Send and receive files in a browser or an Android or iOS app |
| [exchange](exchange) | Fula exchange protocol is responsible for the ctual transfer of data |

## Run

```go
git clone https://github.com/functionland/go-fula.git

cd go-fula

go run ./cmd/blox
```

The default generated config goes to a YAML file in home directory, under /.fula/blox/config.yaml

## Build

```go
goreleaser --rm-dist --snapshot
```


## License

[MIT](LICENSE)

## Related Publications and News

- https://filecoin.io/blog/posts/249k-for-17-projects-from-dorahacks-filecoin-grant-hackathon/
- https://dev.to/fx/google-photos-open-source-alternative-with-react-native-80c
- https://hackernoon.com/were-building-an-open-source-google-photos-alternative-with-react-native-zw4537pa
- https://crustnetwork.medium.com/crust-network-and-functionland-partnering-up-on-web3-developer-tools-309e41074fc5
