package blockchain

import (
	"context"
	"testing"
)

func TestSimpleKeyStore(t *testing.T) {
	keyStore := NewSimpleKeyStorer("./internal")
	err := keyStore.SaveKey(context.Background(), "dummy")
	if err != nil {
		t.Errorf("while save key: %v", err)
	}
	key, err := keyStore.LoadKey(context.Background())
	if err != nil {
		t.Errorf("while load key: %v", err)
	}
	if string(key) != "dummy" {
		t.Errorf("error loading the stored key: %v != dummy", string(key))
	}
}
