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
| [exchange](exchange) | Fula exchange protocol is responsible for the actual transfer of data |
| [blockchain](blockchain) | On-chain interactions for pool management, manifests, and account operations |
| [wap](wap) | Wireless Access Point server — provides HTTP endpoints (`/properties`, `/readiness`, `/wifi/*`, `/peer/*`) on port 3500 and mDNS service discovery |

## Other related libraries

| Name | Description |
| --- | --- |
| [WNFS for Android](https://github.com/functionland/wnfs-android) | Android build for WNFS rust version |
| [WNFS for iOS](https://github.com/functionland/wnfs-ios) | iOS build for WNFS rust version |

## PeerID Architecture

Blox devices maintain two separate libp2p identities:

- **Kubo peerID** — Derived via HMAC-SHA256 (domain `"fula-kubo-identity-v1"`) from the main identity key. Used by the embedded IPFS (kubo) node. Always available.
- **ipfs-cluster peerID** — The original identity from `config.yaml`. Used by ipfs-cluster when installed.

Kubo is always running on the device; ipfs-cluster may not be installed. The `wifi.GetKuboPeerID()` utility function provides reliable access to the kubo peerID (via config file or kubo API fallback) without depending on ipfs-cluster.

### Getting ipfs-cluster and kubo PeerIDs

There are several ways to retrieve peerIDs depending on which part of the system you are working with.

#### 1. WAP HTTP Endpoints (port 3500)

These endpoints are served by the WAP server on the device. Mobile apps connect to them over the local network.

**GET `/properties`**

Returns device properties including the kubo peerID.

```bash
curl http://<device-ip>:3500/properties
```

Response (relevant fields):
```json
{
  "kubo_peer_id": "12D3KooWAbCdEf...",
  "hardwareID": "a1b2c3...",
  "bloxFreeSpace": { "device_count": 1, "size": 500000000000, "used": 120000000000, "avail": 380000000000, "used_percentage": 24 },
  "ota_version": "1.2.3",
  "...": "..."
}
```

- `kubo_peer_id` — The kubo (IPFS) peerID. Always present when kubo is running.

**GET `/readiness`**

Returns readiness status and properties, also including the kubo peerID.

```bash
curl http://<device-ip>:3500/readiness
```

Response (relevant fields):
```json
{
  "name": "fula_go",
  "kubo_peer_id": "12D3KooWAbCdEf...",
  "...": "..."
}
```

#### 2. Mobile SDK (`GetClusterInfo`)

From the mobile app, call `GetClusterInfo()` on the Fula client. This uses libp2p to call the blox node directly (no HTTP needed).

```go
result, err := client.GetClusterInfo()
```

Response JSON:
```json
{
  "cluster_peer_id": "12D3KooWXyZaBc...",
  "cluster_peer_name": "12D3KooWAbCdEf..."
}
```

| Field | Description | When empty |
|-------|-------------|------------|
| `cluster_peer_id` | ipfs-cluster peerID (from `/uniondrive/ipfs-cluster/identity.json`) | ipfs-cluster is not installed |
| `cluster_peer_name` | kubo peerID (from kubo config or API) | Only if kubo is also unreachable |

When ipfs-cluster is not installed, the response will have an empty `cluster_peer_id` but `cluster_peer_name` (kubo peerID) will still be populated:
```json
{
  "cluster_peer_id": "",
  "cluster_peer_name": "12D3KooWAbCdEf..."
}
```

#### 3. Kubo API Directly (port 5001)

If you have direct access to the device, you can query kubo's identity endpoint:

```bash
curl -X POST http://127.0.0.1:5001/api/v0/id
```

Response:
```json
{
  "ID": "12D3KooWAbCdEf...",
  "PublicKey": "base64encodedkey...",
  "Addresses": [],
  "AgentVersion": "0.0.1",
  "ProtocolVersion": "fx_exchange/0.0.1",
  "Protocols": ["fx_exchange"]
}
```

- `ID` — The kubo peerID.

#### 4. mDNS Service Discovery

Blox devices advertise both peerIDs via mDNS TXT records on the local network (service type `_fulatower._tcp`).

| TXT Key | Value | Description |
|---------|-------|-------------|
| `bloxPeerIdString` | `12D3KooWAbCdEf...` | Kubo peerID |
| `ipfsClusterID` | `12D3KooWXyZaBc...` | ipfs-cluster peerID |
| `poolName` | `my-pool` | Pool name from config |
| `authorizer` | `...` | Authorizer address |
| `hardwareID` | `a1b2c3...` | Device hardware ID |

If the config file is missing or identity derivation fails, `bloxPeerIdString` falls back to reading from kubo's config file via `GetKuboPeerID()`. Fields default to `"NA"` when unavailable.

#### 5. Go Code (`wifi.GetKuboPeerID()`)

For internal Go code that needs the kubo peerID, use the standalone utility function:

```go
import wifi "github.com/functionland/go-fula/wap/pkg/wifi"

kuboPeerID, err := wifi.GetKuboPeerID()
if err != nil {
    // both kubo config file and kubo API are unavailable
}
```

This function tries two sources in order:
1. Read `/internal/ipfs_data/config` and parse `Identity.PeerID`
2. Call kubo API at `http://127.0.0.1:5001/api/v0/id` and parse the `ID` field

It does not depend on ipfs-cluster being installed.

## Run

```go
git clone https://github.com/functionland/go-fula.git

cd go-fula

go run ./cmd/blox --authorizer [PeerID of client allowed to write to the backend] --logLevel=[info/debug] --ipniPublisherDisabled=[true/false]
```

example on `Windows` is:
```
go run ./cmd/blox --authorizer=12D3KooWMMt4C3FKui14ai4r1VWwznRw6DoP5DcgTfzx2D5VZoWx --config=C:\Users\me\.fula\blox2\config.yaml --storeDir=C:\Users\me\.fula\blox2 --secretsPath=C:\Users\me\.fula\blox2\secret_seed.txt --logLevel=debug
```

The default generated config goes to a YAML file in home directory, under `/.fula/blox/config.yaml`

## Build

```go
goreleaser --rm-dist --snapshot
```

## Build for the iOS platform (using gomobile)
This process includes some iOS specific preparation.
```sh
make fula-xcframework
```


## License

[MIT](LICENSE)

## Related Publications and News

- https://www.crunchbase.com/organization/functionland/signals_and_news
