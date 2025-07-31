package blockchain

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestPoolDiscoveryBug - Simple test to isolate the pool discovery issue
func TestPoolDiscoveryBug(t *testing.T) {
	fmt.Println("=== TESTING POOL DISCOVERY BUG ===")
	
	// Create blockchain instance with proper initialization
	bl := &FxBlockchain{
		options: &options{
			timeout: 30,
		},
		bufPool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
		reqPool: &sync.Pool{
			New: func() interface{} {
				return new(http.Request)
			},
		},
		ch: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test pool discovery on skale chain
	fmt.Println("Testing pool discovery on skale chain...")
	poolList, err := bl.HandleEVMPoolList(ctx, "skale")
	if err != nil {
		t.Fatalf("Pool discovery failed: %v", err)
	}

	fmt.Printf("Found %d pools\n", len(poolList.Pools))
	
	// Print each discovered pool
	for i, pool := range poolList.Pools {
		fmt.Printf("Pool[%d]: ID=%d, Name='%s', Creator='%s', ChainName='%s'\n", 
			i, pool.ID, pool.Name, pool.Creator, pool.ChainName)
		
		// Verify pool ID is not 0
		if pool.ID == 0 {
			t.Errorf("ERROR: Pool at index %d has ID 0, which should not exist!", i)
		}
	}

	// Now test membership check for each discovered pool
	testPeerID := "12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1"
	
	for _, pool := range poolList.Pools {
		fmt.Printf("\n--- Testing membership for pool ID %d ---\n", pool.ID)
		
		req := IsMemberOfPoolRequest{
			PeerID:    testPeerID,
			PoolID:    pool.ID,
			ChainName: "skale",
		}

		fmt.Printf("Calling HandleIsMemberOfPool with PoolID=%d\n", req.PoolID)
		
		resp, err := bl.HandleIsMemberOfPool(ctx, req)
		if err != nil {
			t.Errorf("Membership check failed for pool %d: %v", pool.ID, err)
			continue
		}

		fmt.Printf("Membership result: IsMember=%t, MemberAddress=%s, PoolID=%d, ChainName=%s\n", 
			resp.IsMember, resp.MemberAddress, resp.PoolID, resp.ChainName)
			
		// Check if the response has the correct pool ID
		if resp.PoolID != pool.ID {
			t.Errorf("ERROR: Response pool ID mismatch! Expected %d, got %d", pool.ID, resp.PoolID)
		}
	}
	
	fmt.Println("\n=== TEST COMPLETE ===")
}

// TestDirectMembershipCheck - Test membership check directly with known pool ID
func TestDirectMembershipCheck(t *testing.T) {
	fmt.Println("=== TESTING DIRECT MEMBERSHIP CHECK ===")
	
	bl := &FxBlockchain{
		options: &options{
			timeout: 30,
		},
		bufPool: &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
		reqPool: &sync.Pool{
			New: func() interface{} {
				return new(http.Request)
			},
		},
		ch: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	testPeerID := "12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1"

	// Test pool 1 directly (we know it exists)
	fmt.Println("Testing membership for pool ID 1 directly...")
	
	req := IsMemberOfPoolRequest{
		PeerID:    testPeerID,
		PoolID:    1, // Directly test pool 1
		ChainName: "skale",
	}

	fmt.Printf("Calling HandleIsMemberOfPool with PoolID=%d\n", req.PoolID)
	
	resp, err := bl.HandleIsMemberOfPool(ctx, req)
	if err != nil {
		t.Fatalf("Direct membership check failed for pool 1: %v", err)
	}

	fmt.Printf("Direct membership result: IsMember=%t, MemberAddress=%s, PoolID=%d, ChainName=%s\n", 
		resp.IsMember, resp.MemberAddress, resp.PoolID, resp.ChainName)

	// Test pool 0 directly (should return false immediately)
	fmt.Println("\nTesting membership for pool ID 0 directly...")
	
	req0 := IsMemberOfPoolRequest{
		PeerID:    testPeerID,
		PoolID:    0, // Test pool 0
		ChainName: "skale",
	}

	fmt.Printf("Calling HandleIsMemberOfPool with PoolID=%d\n", req0.PoolID)
	
	resp0, err := bl.HandleIsMemberOfPool(ctx, req0)
	if err != nil {
		t.Fatalf("Pool 0 membership check should not fail: %v", err)
	}

	fmt.Printf("Pool 0 membership result: IsMember=%t, MemberAddress=%s, PoolID=%d, ChainName=%s\n", 
		resp0.IsMember, resp0.MemberAddress, resp0.PoolID, resp0.ChainName)

	if resp0.IsMember {
		t.Error("ERROR: Pool 0 should not have any members!")
	}
	
	fmt.Println("\n=== DIRECT TEST COMPLETE ===")
}
