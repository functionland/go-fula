package blockchain

import (
	"context"
	"testing"
	"time"
)

// TestPoolDiscoveryAndMembership tests the complete pool discovery and membership flow
func TestPoolDiscoveryAndMembership(t *testing.T) {
	// Create blockchain instance with proper options
	bl, err := NewFxBlockchain(
		NewSimpleKeyStorer(""),
		WithTimeout(30),
	)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	testPeerID := "12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1"

	// Test both chains
	chains := []string{"skale", "base"}
	
	for _, chainName := range chains {
		t.Run("Chain_"+chainName, func(t *testing.T) {
			t.Logf("Testing pool discovery and membership for chain: %s", chainName)

			// Step 1: Test pool discovery
			poolList, err := bl.HandleEVMPoolList(ctx, chainName)
			if err != nil {
				t.Fatalf("Pool discovery failed for chain %s: %v", chainName, err)
			}

			t.Logf("Found %d pools on chain %s", len(poolList.Pools), chainName)

			// Verify we found at least one pool
			if len(poolList.Pools) == 0 {
				t.Fatalf("No pools discovered on chain %s", chainName)
			}

			// Verify pool IDs are not 0
			for i, pool := range poolList.Pools {
				if pool.ID == 0 {
					t.Errorf("Pool at index %d has ID 0, which should not exist", i)
				}
				t.Logf("Discovered pool ID: %d on chain %s", pool.ID, chainName)
			}

			// Step 2: Test membership check for each discovered pool
			for _, pool := range poolList.Pools {
				t.Run("Pool_"+string(rune(pool.ID)), func(t *testing.T) {
					req := IsMemberOfPoolRequest{
						PeerID:    testPeerID,
						PoolID:    pool.ID,
						ChainName: chainName,
					}

					t.Logf("Checking membership for pool %d on chain %s", pool.ID, chainName)

					resp, err := bl.HandleIsMemberOfPool(ctx, req)
					if err != nil {
						t.Errorf("Membership check failed for pool %d on chain %s: %v", pool.ID, chainName, err)
						return
					}

					t.Logf("Membership result for pool %d on chain %s: isMember=%t, address=%s", 
						pool.ID, chainName, resp.IsMember, resp.MemberAddress)

					// Verify response structure
					if resp.ChainName != chainName {
						t.Errorf("Expected chain name %s, got %s", chainName, resp.ChainName)
					}
					if resp.PoolID != pool.ID {
						t.Errorf("Expected pool ID %d, got %d", pool.ID, resp.PoolID)
					}

					// For skale chain and pool 1, we expect membership to be true
					if chainName == "skale" && pool.ID == 1 {
						if !resp.IsMember {
							t.Errorf("Expected membership to be true for pool 1 on skale chain")
						}
						if resp.MemberAddress == "0x0000000000000000000000000000000000000000" {
							t.Errorf("Expected non-zero member address for pool 1 on skale chain")
						}
					}
				})
			}
		})
	}
}

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

// TestPoolDiscoveryReturnsCorrectPoolIDs tests that pool discovery returns the correct pool IDs
func TestPoolDiscoveryReturnsCorrectPoolIDs(t *testing.T) {
	bl, err := NewFxBlockchain(
		NewSimpleKeyStorer(""),
		WithTimeout(30),
	)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	chains := []string{"skale", "base"}
	
	for _, chainName := range chains {
		t.Run("PoolIDs_"+chainName, func(t *testing.T) {
			poolList, err := bl.HandleEVMPoolList(ctx, chainName)
			if err != nil {
				t.Fatalf("Pool discovery failed: %v", err)
			}

			t.Logf("Chain %s discovered pools:", chainName)
			for i, pool := range poolList.Pools {
				t.Logf("  Pool[%d]: ID=%d, Name=%s, Creator=%s", i, pool.ID, pool.Name, pool.Creator)
				
				// Verify pool ID is valid (not 0)
				if pool.ID == 0 {
					t.Errorf("Pool at index %d has invalid ID 0", i)
				}
			}

			// We expect to find pool 1 on both chains
			foundPool1 := false
			for _, pool := range poolList.Pools {
				if pool.ID == 1 {
					foundPool1 = true
					break
				}
			}

			if !foundPool1 {
				t.Errorf("Expected to find pool ID 1 on chain %s", chainName)
			}
		})
	}
}
