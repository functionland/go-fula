package fulaMobileClient

import (
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"
)

// This is the interface specification for fula mobile client
// The client is the component that is going to be compiled into Java and Obj-C
// for use in the mobile env. Also a WASM build is under consideration for the browser
// and other environments.

// The client is responsible for handling the connection between mobile (the client side)
// and the Blox (the server side). Any means of communication between clients and the Blox
// should be handled by this client.

// Note that here we are specifying the interface only. Many of the function arguments can
// be stored in a receiver struct and passed to each of these functions

// Connect to a blox with peerID
// This will make sure that the connection can be made between two hosts (one on client and
// the other on the blox)
// It returns an error if there cannot be a connection with the given peerID otherwise nil
func ConnectWithPeerID(pid peer.ID) error {

}

// Find a peerID for when the user does not own a blox.
// It will return a blox's peerID which is willing to respond to client's requests
func ConnectWithPoolName(pn string) (peer.ID, error) {

}

// Sync the local blockstore with the WNFS drive on the blox.
// Each user has a WNFS drive bound to his/her DID. The drive is basically a DAG which is created
// and edited on the client side (outside of the scope of this client) and then is uploaded
// to the user's blox to be stored on the blox's blockstore.
// A user can have multiple client devices and also the possibility of data loss on the client side
// may make the two drives's (on the client and the blox) states diverge.
// After the sync is done, bs (the local blockstore on the client) should have all the cids that are present
// on the blox's blockstore
func SyncDrive(bloxPID peer.ID, bs BlockStore) error {

}

// Since all of the DAG processing is getting done by WNFS on the client side, we can see the blox as a
// remote blockstore which we use for persisting our WNFS blocs. Seeing the problem from this point of
// view, we will need standard blockstore interface except for a remote store.
// Following functions are meant to be used for manual drive syncing or fallback scenarios where a full
// sync is not applicable.

// Get an object by its cid from a remote blockstore on a blox.
// `bloxPid` is the peerId for the blox, cid is the cid of object we want to get, and bs is the local blockstore
// which will be used for storing object after it is received
func GetObject(bloxPid peer.ID, cid cid.Cid, bs BlockStore) error {

}

// Put an object that is locally stored on the blockstore to the remote blockstore which is on the blox
func PutObject(bloxPid peer.ID, cid cid.Cid, bs BlockStore) error {

}

// Check if an object exists on the remote blockstore
func Exists(bloxPid peer.ID, cid cid.Cid) (bool, error) {

}
