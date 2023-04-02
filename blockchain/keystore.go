package blockchain

import (
	"context"
	"os"
)

// Implementations for this interface should be responsible for saving/loading a single key.
type KeyStorer interface {
	SaveKey(ctx context.Context, key []byte) error
	LoadKey(ctx context.Context) ([]byte, error)
}

type SimpleKeyStorer struct {
}

func NewSimpleKeyStorer() *SimpleKeyStorer {
	return &SimpleKeyStorer{}
}

func (s *SimpleKeyStorer) SaveKey(ctx context.Context, key []byte) error {
	return os.WriteFile("key.db", key, 0400)
}

func (s *SimpleKeyStorer) LoadKey(ctx context.Context) ([]byte, error) {
	return os.ReadFile("key.db")
}
