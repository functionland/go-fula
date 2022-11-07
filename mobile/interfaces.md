# Fula interfaces and architecture

This issue is trying to create an overview of the final architecture of Fula as well as detailed information about each module's interfaces.

## Use case
An end-to-end experience of using (developing with) the Fula ecosystem of tools and protocols provides a user (developer) with the necessary infrastructure and services for a decentralized application platform.

The Fula ecosystem consists of different tools enabling an application to use the Fula network of Bloxes as a reliable, highly available, and consistent back-end service. For the sake of clarity and to avoid confusion, we define an end-to-end sample for using the Fula platform. This sample use case also will help us to identify necessary modules and interfaces along the way.

__Sample Scenario__: A full end-to-end use of FxFotos application leading to having a photo gallery with the ability of cloud (Blox) backup and sharing. The sample scenario consists of these steps:

1. User (A) connects her FxFotos app installed on her phone to her Blox.
2.  User (A) views her photos on the FxFotos App and uploads some to the Blox (encrypted).
3. User (A) connects another device with FxFotos installed on it to her Blox.
4. User (A) downloads the previously uploaded photo (decrypted) to the new device and views it.
5. User (A) shares the uploaded photo with another User (B).
6. User (B) downloads (decrypted) the shared photo on his FxFotos app on his device and views it.

Notes:
- Each user (A, B) is identified by a personal wallet stored by Metamask on his/her device. User (A) that has two devices, should have the same wallet on each of them.
- Sharing in this context means sharing the cryptographic key needed to decrypt the photo. We will talk about the format for this process in this issue, but sending this key (encapsulated in a specific structure) to the other user is out of context for now.
- We are assuming that each user has his/her own Blox. Also, another assumption is that Blox has an open read endpoint but the write endpoint is protected and only available for the owner.
- The replication process (among different Bloxes) is going to happen by the Pool protocol and will be discussed in a separate issue.

Having the above scenario in mind, we can now identify different modules in the process and the way they are connected to each other.

## Modules
In order for the scenario to happen, different components are needed. Each of these components has a specific role in the design. We will discuss their tasks and also the means of communication between each two of them.

We can divide the components based on the device they are running on. Some of them run on the client (user's mobile device) and others run on the Blox side.

### Client Side 
Considering the sample scenario, we can identify different requirements on the client side (from now on when we talk about the client we mean an application, like FxFotos, installed on the user's mobile device):
- Client wants to connect to a Blox 
  - Client needs to find the Blox on the network with some address.
  - The client needs to check if the Blox is accessible and accepting requests.
- The client needs to prove that it is connecting from a device that is owned by the owner of the Blox.
- Client wants to send its content (in any format structured as a directory tree) to the owner's Blox.
- Client wants to make sure that the sent data is stored on the Blox.
- Client wants to get specific content from Blox.
  - Client wants to see what content is already stored on the Blox.
  - Client wants to select specific content (in a human-understandable way) and get it from Blox.
- Client wants to share specific content with another client/user
  - The client needs to identify the other user with a unique ID.
  - The client wants the other user to be able to view his/her encrypted content.
- The user wants to share a unique ID among all of his/her devices.
- The user wants his/her data to be stored encrypted and only decryptable by a user-defined list of people.
- Client wants to make sure that the content it is storing is in-sync with the content stored on the Blox.

#### React Native Fula
This is the main module that is going to be installed on the client device. Considering the client is a react native application (FxFotos), React Native Fule will be the only package that the developer needs to use in order to satisfy the sample scenario. React Native Fule is responsible for all of requirements listed above. 

First, we list the interfaces that React Native Fula exposes for development of something like FxFotos. To be compatible with the design system of react applications, React Native Fula will introduce a custom hook that hides all of the complexity from the developer and providing a Drive interface for satisfying the requirements.

- `useDrive(bloxPid: string, userKP: KeyPair): FulaDrive`

`FulaDrive` is the struct encapsulating all of necessary functionalities for using a Blox as a back-end service. It has two different members `public` and `private` which point to public and private directories in the drive respectively. Each of these members provides a POSIX-like interface for working with its corresponding directory tree.

- `async drive.public.mkdir(path: []string, directoryName: string): StoredDir`: it will create a new directory in the specified path, returns the stored directory (more on `StoredDir` later on this issue)

- `async drive.public.ls(path: []string): []DirEntry`: lists the entries inside a directory. `DirEntry` is a type representing a file or a directory.

- `async drive.public.write(path: []string, data: UInt8Array): StoredFile`: writes the data represented in bytes to a specific path.

- `async drive.public.read(path: []string): File`: reads the data stored in a specific path and returns it as `File` (more on this type later in this issue)

- `async drive.public.rm(path: []string, isDir: boolean): StoredDir`: removes a file/directory at a specific path, returns the the imediate parent directory.

- `async drive.public.info(path: []string): Meta?`: returns a file/directory's metadata, if not exists, returns null.

The private directory inside a FulaDrive provides the same interface. 

- `async drive.private.mkdir(path: []string, directoryName: string): StoredDir`

- `async drive.private.ls(path: []string): []DirEntry`

- `async drive.private.write(path: []string, data: UInt8Array): StoredFile`

- `async drive.private.read(path: []string): File` 

- `async drive.private.rm(path: []string, isDir: boolean): StoredDir`

- `async drive.private.info(path: []string): Meta?`

Both private and public directories provide a `share` method. This method is used for exporting/sharing a file or directory to the outside world.

- `async drive.public.share(path: []string): StoredFile`: returns the necessary info for accessing a file/directory by other users. For a public file/directory this will be the `StoredFile` consisting the address (CID) for the file/directory

- `async drive.private.share(path: []string, with: PublicKey): Sharable`: returns a sharable object. A `Sharable` is an object that consists of everything a paricular user needs to view a private file. The returning sharable object is encypted by the receivers public key and consists of the address and the cryptographic key needed for viewing the file.

Besides the POSIX interface that a FulaDrive provides for maipulating the driectory structure, it has another method for syncing all of changes with a Blox. Since the creation of the directory structure is completely done on the client side (by WNFS) we need a method for persisting the changes on the Blox.

- `async drive.sync(opts: SyncOptions): Conflicts?`: syncronises the local drive with the remote one stored on the Blox. The synchronization process can follow different policies. For now we are considring three different modes:
  - `push`: The local drive will be pushed to the Blox and replaces the one currently on the Blox.
  - `pull`: The remote drive will be pulled from the Blox and replace the local one.
  - `merge`: Try to pull all of contents that are just on the remote drive and push all the content that are on the local drive. If facing a conflict, cancel the operation and inform about the conflicts.

#### Inside React Native Fule
React Native Fula is the only library that the application developer is going to use. To provide this level of simplicity, we wrap all of other packages inside React Native Fula. Each of these packages has different responsibilites which are listed below.

- __Fula Client__: This package is reponsible for any kind of connection with the Blox. Right now, it is written in go and gets compiled into java using gomobile. This package wraps a libp2p host which connects to the Blox. It also provides interfaces for sending and receiving data to and from the Blox.
- __react-native-wnfs__: This is the package that runs the WNFS inside react-native. all of the interfaces inside `public` and `private` of `FulaDrive` are going to be translated corresponding ones from wnfs which is provided by this package. 

The `FulaDrive` interface is wrapping above packages to provide a unified experience and hide the complexities of WNFS's `PublicDirectory` and `PrivateDirectory`. Also it will connect the Fula Client as a remote blockstore to the WNFS core.

### Box Side
#### Pool Protocol
#### go-fula Blox node