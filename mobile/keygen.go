package fulamobile

import (
	"bytes"
	"crypto/sha256"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// GenerateEd25519Key generates a random Ed25519 libp2p private key.
func GenerateEd25519Key() ([]byte, error) {
	pk, _, err := crypto.GenerateEd25519Key(nil)
	if err != nil {
		return nil, err
	}
	return crypto.MarshalPrivateKey(pk)
}

func GenerateEd25519KeyFromString(secret string) ([]byte, error) {
	// Convert your secret into a seed
	seed := sha256.Sum256([]byte(secret))

	// Generate the Ed25519 key pair
	pk, _, err := crypto.GenerateEd25519Key(bytes.NewReader(seed[:]))
	if err != nil {
		return nil, err
	}

	return crypto.MarshalPrivateKey(pk)
}
