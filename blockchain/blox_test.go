package blockchain

import (
	"testing"
)

func TestBloxFreeSpaceSanity(t *testing.T) {
	/*bl, err := NewFxBlockchain(nil, NewSimpleKeyStorer())
	if err != nil {
		t.Errorf("creating blockchain instance: %v", err)
	}
	resp, err := bl.BloxFreeSpace(context.Background(), "")
	if err != nil {
		t.Errorf("calling blockchain bloxFreeSpace api: %v", err)
	}
	out := &BloxFreeSpaceResponse{}
	err = json.Unmarshal(resp, out)
	if err != nil {
		t.Errorf("unmarshal bloxFreeSpace api response: %v", err)
	}

	// Sanity check
	if out.Avail+out.Used-out.Size > 0.1 {
		t.Error("insane result from the bloxFreeSpace api")
	}*/
}
