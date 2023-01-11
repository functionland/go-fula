# box

[![Go Test](https://github.com/functionland/go-fula/actions/workflows/go-test.yml/badge.svg)](https://github.com/functionland/go-fula/actions/workflows/go-test.yml) [![](https://jitpack.io/v/functionland/fula-build-aar.svg)](https://jitpack.io/#functionland/fula-build-aar)


Client-server stack for Web3

[Intro blog](https://dev.to/fx/google-photos-open-source-alternative-with-react-native-80c#ending-big-techs-reign-by-building-opensource-p2p-apps)

You can see it in action in our Flagship App: [Fx Fotos](https://github.com/functionland/fx-fotos)

[![Blox server demo](https://gateway.pinata.cloud/ipfs/QmVd3eioLfp19hG1GnfgceBsbK16i39vsQ9Lxq35aAjxnY)](https://gateway.pinata.cloud/ipfs/QmWUjQczA5jHC3ibLq4y7CizVrebr1DTFRaTdJgFyxR5Nh)

## Motivation

There are currently two ways to interact with Web3 storage solutions:

1. Through a pinning service and a gateway: the advantage is that files are served through URLs, an app can then access the files with conventional methods, e.g. simply putting a picture in `<img src="gateway.example.com/Qm...">`. The disadvantage is that there is a subscription payment associated with pinning services. Also this is not really decentralized!
2. Turn the device to a full IPFS node: this model works beautifully in Brave desktop browser as an example, and makes sense for laptop and PC since they normally have large HDDs. It's much harder on mobile devices, however, biggest hurdle is to have Apple on board with the idea of relaxing file system access in iOS! Even if all goes well, a mobile device is NOT a good candidate for hosting the future Web! They get lost easily and are resource constrained (battery, memory).

[**blox**](https://github.com/functionland/BLOX) aims to address these issues by creating a third alternative: **Personal Server**

A personal server is a commodity hardware device (such as a PC or Raspberry Pi) that is kept *at home* rather than carried with the user. It can help with actual decentralization and can save money in the long run, as there is a one-time cost for the hard drive and no monthly charges. From a privacy perspective, it also guarantees that data does not leave the premises unless the user specifically wants to share it.

To achieve this, we are developing protocols to accommodate client-server programming with minimal effort on developer's side.

## Architecture

`go-fula` is an implementation of the Fula protocol in Go (Golang). It is designed to facilitate smooth communication between clients and a mesh of backend devices (servers) and provide APIs for developers to build mobile-native decentralized applications (dApps).

![box architecture](https://gateway.pinata.cloud/ipfs/QmNkoQfCKAzQetJKWfNtLioJf6FCxzqjoDT2KshDZfsJd3)

A React Native app can communicate with servers using the `@functionland/react-native-fula` library, which abstracts the underlying protocols and `libp2p` connection and exposes APIs similar to those of MongoDB for data persistence and S3 for file storage.

Data is encrypted on the client side using [WebNative Filesystem (WNFS)](https://github.com/wnfs-wg/rs-wnfs) ( with bridges for [Android](https://github.com/functionland/wnfs-android) and [iOS](https://github.com/functionland/wnfs-ios) ). The encrypted Merkle DAG is then transferred to the blox server using Graphsync.

The **blox** stack can provide backup guarantees by having the data pinned on multiple servers owned by the user. In cases where absolute assurance of data longevity is required (e.g. password records in a password manager app or scans of sensitive documents), the cids of encrypted data can be sent to the [Fula blockchain](https://github.com/functionland/sugarfunge-node) and backed up by other blox owners, who are rewarded for their efforts.

By default, Libp2p connections are established through Functionland's libp2p relay over the internet or directly over LAN without the use of a relay.

## Packages

| Name | Description |
| --- | --- |
| [blox](blox) | Blox provides the backend to receive the DAG created by fulamobile and store it |
| [mobile](mobile) | Initiates a libp2p instance and interacts with WNFS (as its datastore) to encrypt the data and Send and receive files in a browser or an Android or iOS app. Available for [React-Native here](https://github.com/functionland/react-native-fula) and for [Android here](https://github.com/functionland/fula-build-aar) |
| [exchange](exchange) | Fula exchange protocol is responsible for the ctual transfer of data |

## Other related libraries

| Name | Description |
| --- | --- |
| [WNFS for Android](https://github.com/functionland/wnfs-android) | Android build for WNFS rust version |
| [WNFS for iOS](https://github.com/functionland/wnfs-ios) | iOS build for WNFS rust version |

## Run

```go
git clone https://github.com/functionland/go-fula.git

cd go-fula

go run ./cmd/blox --authorizer [PeerID of client allowed to write to the backend]
```

The default generated config goes to a YAML file in home directory, under /.fula/blox/config.yaml

## Build

```go
goreleaser --rm-dist --snapshot
```


## License

[MIT](LICENSE)

## Related Publications and News

- https://www.crunchbase.com/organization/functionland/signals_and_news
