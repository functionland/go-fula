# box

[![Go Test](https://github.com/functionland/go-fula/actions/workflows/go-test.yml/badge.svg)](https://github.com/functionland/go-fula/actions/workflows/go-test.yml)

Client-server stack for Web3

[Intro blog](https://dev.to/fx/google-photos-open-source-alternative-with-react-native-80c#ending-big-techs-reign-by-building-opensource-p2p-apps)

[Flagship App: Photos](https://github.com/functionland/photos)

![client-server resemblance](https://user-images.githubusercontent.com/1758649/126010892-b7bf9905-0044-472d-aeb6-1ed7e66268d8.png)

![box server demo](https://user-images.githubusercontent.com/1758649/126008513-e5f8f2eb-b931-4450-8373-6102cf47e7d1.png)

## Motivation

There are currently two ways to interact with Web3 storage solutions:

1. Through a pinning service and a gateway: the advantage is that files are served through URLs, an app can then access the files with conventional methods, e.g. simply putting a picture in `<img src="gateway.example.com/Qm...">`. The disadvantage is that there is a subscription payment associated with pinning services. Also this is not really decentralized!
2. Turn the device to a full IPFS node: this model works beautifully in Brave desktop browser as an example, and makes sense for laptop and PC since they normally have large HDDs. It's much harder on mobile devices, however, biggest hurdle is to have Apple on board with the idea of relaxing file system access in iOS! Even if all goes well, a mobile device is NOT a good candidate for hosting the future Web! They get lost easily and are resource constrained (battery, memory).

**box** aims to address these issues by creating a third alternative: **Personal Server**

A personal server is a commodity hardware (PC, Raspberry Pi, etc.) that's kept *at home* vs. *in pocket*. It helps with actual decentralization, also saves money since people pay once for HDDs and own them forever, no monthly charge! From privacy perspective, it guarantees that data doesn't leave the premise unless user specifically wants to (e.g. sharing).

To achieve this, we are developing protocols to accommodate client-server programming with minimal effort on developer's side:

- [File Protocol](protocols/file): Send and receive files in a browser or an app **(stage: prototype)**
- [Data Protocol](protocols/data): Database interface over at client-side; facilitates describing linked JSON documents and having them saved/retrieved **(stage: design draft)**
- [AI Protocol](protocols/ai): Map-Reduce stack for distributed processing **(stage: ideation)**

## Architecture

![box architecture](https://user-images.githubusercontent.com/1758649/126281564-a3a5aea9-50a6-4ae9-ae2b-439977d57980.jpg)

An app talks with the server(s) by invoking APIs from `@functionland/fula` library. The Fula library abstracts away the protocols and `libp2p` connection, instead exposes APIs similar to MongoDB for data persistence and S3 for file storage.

On the server side, data or file will be saved on a private IPFS instance. There is also a public IPFS instance which comes to play in sharing: when the end user requests to share some data, an encrypted copy will be saved on the public IPFS network, anyone with whom the key has been shared can access the data.

The **box** stack can provide backup guarantees by having the data pinned on multiple servers owned by the user. However, in cases that the user needs absolute assurance on data longevity, e.g. password records in a password manager app or scans of sensitive documents, the encrypted data can be sent over at Filecoin blockchain.

## Packages

| Name | Description |
| --- | --- |
| [protocols](protocols) | Libp2p protocols for the box stack |
| [beast](beast) | Server implementation in Go |
| [mobile](mobile) | Client library in golang with gomobile interface comptibility for generating IOS and Android apps |



## License

[MIT](LICENSE)

## Related Publications and News

- https://filecoin.io/blog/posts/249k-for-17-projects-from-dorahacks-filecoin-grant-hackathon/
- https://dev.to/fx/google-photos-open-source-alternative-with-react-native-80c
- https://hackernoon.com/were-building-an-open-source-google-photos-alternative-with-react-native-zw4537pa
- https://crustnetwork.medium.com/crust-network-and-functionland-partnering-up-on-web3-developer-tools-309e41074fc5
