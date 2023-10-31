package blockchain

import (
	"context"
	"os"
)

// Implementations for this interface should be responsible for saving/loading a single key.
type KeyStorer interface {
	SaveKey(ctx context.Context, key string) error
	LoadKey(ctx context.Context) (string, error)
}

type SimpleKeyStorer struct {
	dbPath string
}

func NewSimpleKeyStorer() *SimpleKeyStorer {
	// Saving the db in the local dir
	return &SimpleKeyStorer{dbPath: "/internal/.secrets"}
}

func (s *SimpleKeyStorer) SaveKey(ctx context.Context, key string) error {
	return os.WriteFile(s.dbPath+"/secret_seed.txt", []byte(key), 0400)
}

func (s *SimpleKeyStorer) LoadKey(ctx context.Context) (string, error) {
	data, err := os.ReadFile(s.dbPath + "/secret_seed.txt")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
