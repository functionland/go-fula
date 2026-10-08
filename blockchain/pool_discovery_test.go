package blockchain

import (
	"context"
	"testing"
	"time"
)

// TestPoolZeroHandling tests that pool 0 is properly handled
func TestPoolZeroHandling(t *testing.T) {
	bl, err := NewFxBlockchain(
		NewSimpleKeyStorer(""),
		WithTimeout(30),
	)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testPeerID := "12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1"

	// Test membership check for pool 0 (should return false immediately)
	req := IsMemberOfPoolRequest{
		PeerID:    testPeerID,
		PoolID:    0, // Pool 0 doesn't exist
		ChainName: "skale",
	}

	resp, err := bl.HandleIsMemberOfPool(ctx, req)
	if err != nil {
		t.Fatalf("Pool 0 membership check should not return error: %v", err)
	}

	if resp.IsMember {
		t.Error("Pool 0 membership should be false")
	}

	if resp.MemberAddress != "0x0000000000000000000000000000000000000000" {
		t.Errorf("Pool 0 member address should be zero address, got: %s", resp.MemberAddress)
	}

	t.Logf("Pool 0 handling test passed: isMember=%t, address=%s", resp.IsMember, resp.MemberAddress)
}
