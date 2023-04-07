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

func NewSimpleKeyStorer(dbPath string) *SimpleKeyStorer {
	if err := os.MkdirAll(dbPath, os.ModePerm); err != nil {
		log.Error("SimpleKeyStorer: can't make the required dirs, fallback to local dir")
		dbPath = "."
	}
	return &SimpleKeyStorer{dbPath: dbPath}
}

func (s *SimpleKeyStorer) SaveKey(ctx context.Context, key []byte) error {
	return os.WriteFile(s.dbPath+"/key.db", key, 0400)
}

func (s *SimpleKeyStorer) LoadKey(ctx context.Context) ([]byte, error) {
	return os.ReadFile(s.dbPath + "/key.db")
}
