package blockchain

import (
	"context"
	"os"
	"strings"
)

// Implementations for this interface should be responsible for saving/loading a single key.
type KeyStorer interface {
	SaveKey(ctx context.Context, key string) error
	LoadKey(ctx context.Context) (string, error)
}

type SimpleKeyStorer struct {
	dbPath string
}

func NewSimpleKeyStorer(dbPath string) *SimpleKeyStorer {
	// Default to current value if empty
	if dbPath == "" {
		dbPath = "/internal/.secrets"
	}

	// Try to create the directory
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		err := os.MkdirAll(dbPath, 0755)
		if err != nil {
			// Fallback to a local directory
			dbPath = os.Getenv("SECRETS_DIR")
			if dbPath == "" {
				dbPath = "."
			}
		}
	}

	return &SimpleKeyStorer{dbPath: dbPath}
}

func (s *SimpleKeyStorer) SaveKey(ctx context.Context, key string) error {
	return os.WriteFile(s.dbPath+"/secret_seed.txt", []byte(key), 0600)
}

func (s *SimpleKeyStorer) LoadKey(ctx context.Context) (string, error) {
	data, err := os.ReadFile(s.dbPath + "/secret_seed.txt")
	if err != nil {
		return "", err
	}
	// Trim the whitespace characters from the beginning and end of the string
	return strings.TrimSpace(string(data)), nil
}
