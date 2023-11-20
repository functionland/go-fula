package blockchain

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
)

func TestBloxFreeSpaceSanity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rng := rand.New(rand.NewSource(42))

	// Instantiate the first node in the pool
	pid1, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h1, err := libp2p.New(libp2p.Identity(pid1))
	if err != nil {
		panic(err)
	}
	bl, err := NewFxBlockchain(h1, nil, nil,
		NewSimpleKeyStorer(""),
		WithAuthorizer(h1.ID()),
		WithAllowTransientConnection(true),
		WithBlockchainEndPoint("127.0.0.1:4000"),
		WithTimeout(30),
	)
	if err != nil {
		t.Errorf("creating blockchain instance: %v", err)
	}
	bl.Start(ctx)

}
