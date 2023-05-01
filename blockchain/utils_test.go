package blockchain

import (
	"os"
	"testing"
)

func TestBloxFreeSpaceSanity(t *testing.T) {
	os.Mkdir("./tmp", 0400)
	os.Setenv("FULA_BLOX_STORE_DIR", "./tmp")

	out, err := getBloxFreeSpace()
	if err != nil {
		t.Error(err)
	}

	// Sanity check
	if out.Avail+out.Used-out.Size > 0.1 {
		t.Error("insane result from the bloxFreeSpace api")
	}
	t.Log(out)
	os.Remove("./tmp")
}
