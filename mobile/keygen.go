package fulamobile

import "github.com/libp2p/go-libp2p/core/crypto"

// GenerateEd25519Key generates a random Ed25519 libp2p private key.
func GenerateEd25519Key() ([]byte, error) {
	pk, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		return nil, err
	}
	return crypto.MarshalPrivateKey(pk)
}
