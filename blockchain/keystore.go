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
	dbPath string
}

func NewSimpleKeyStorer() *SimpleKeyStorer {
	// Saving the db in the local dir
	return &SimpleKeyStorer{dbPath: "."}
}

func (s *SimpleKeyStorer) SaveKey(ctx context.Context, key []byte) error {
	return os.WriteFile(s.dbPath+"/key.db", key, 0400)
}

func (s *SimpleKeyStorer) LoadKey(ctx context.Context) ([]byte, error) {
	return os.ReadFile(s.dbPath + "/key.db")
}
