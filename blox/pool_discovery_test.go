package blox

import (
	"context"
	"flag"
	"strconv"
	"testing"
	"time"

	"github.com/functionland/go-fula/blockchain"
)

// Command line flags for test parameters
var (
	testPeerID  = flag.String("peerid", "", "PeerID to test membership for")
	testPoolID  = flag.Int("poolid", 0, "Specific pool ID to test (0 = test all pools)")
	testChain   = flag.String("chain", "", "Specific chain to test (empty = test all chains)")
	testTimeout = flag.Int("timeout", 60, "Timeout in seconds for the test")
)

// TestPoolDiscoveryFlow replicates the exact pool discovery logic from Blox.Start()
// Run with: go test -v -run TestPoolDiscoveryFlow -peerid=<your_peer_id> [-poolid=<pool_id>] [-chain=<chain_name>]
func TestPoolDiscoveryFlow(t *testing.T) {
	flag.Parse()

	peerID := *testPeerID
	if peerID == "" {
		// Default test peer ID
		peerID = "12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1"
		t.Logf("No peerID provided, using default: %s", peerID)
	}

	poolID := uint32(*testPoolID)
	chainFilter := *testChain
	timeout := time.Duration(*testTimeout) * time.Second

	t.Logf("=== Pool Discovery Test ===")
	t.Logf("PeerID: %s", peerID)
	t.Logf("Pool ID filter: %d (0 = all pools)", poolID)
	t.Logf("Chain filter: %s (empty = all chains)", chainFilter)
	t.Logf("Timeout: %v", timeout)

	// Create blockchain instance (same as in Blox)
	bl := createTestBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Replicate the exact logic from Blox.Start()
	found := false
	foundChain := ""
	foundPoolID := uint32(0)

	// List of chains to check (skale first as default) - same as Start()
	chainList := []string{"skale", "base"}
	if chainFilter != "" {
		chainList = []string{chainFilter}
	}

	t.Logf("\n=== Starting Pool Discovery (same as Blox.Start) ===")

	for _, chainName := range chainList {
		if found {
			break
		}

		t.Logf("\n--- Checking chain: %s ---", chainName)

		// Get pool list for this chain (same as Start())
		poolList, err := bl.HandleEVMPoolList(ctx, chainName)
		if err != nil {
			t.Logf("ERROR getting pool list from chain %s: %v", chainName, err)
			continue // Try next chain
		}

		t.Logf("Found %d pools on chain %s", len(poolList.Pools), chainName)

		// If specific pool ID requested, filter to just that pool
		poolsToCheck := poolList.Pools
		if poolID != 0 {
			poolsToCheck = nil
			for _, p := range poolList.Pools {
				if p.ID == poolID {
					poolsToCheck = append(poolsToCheck, p)
					break
				}
			}
			if len(poolsToCheck) == 0 {
				t.Logf("Pool ID %d not found in pool list for chain %s", poolID, chainName)
				continue
			}
		}

		// Check each pool to see if our peerID is a member (same as Start())
		for _, pool := range poolsToCheck {
			if found {
				break
			}

			t.Logf("\nChecking pool ID %d (%s) on chain %s...", pool.ID, pool.Name, chainName)
			t.Logf("  Creator: %s", pool.Creator)
			t.Logf("  Members: %d/%d", pool.MemberCount, pool.MaxMembers)
			t.Logf("  Region: %s", pool.Region)

			// peerID format for membership check (same as Start())
			membershipReq := blockchain.IsMemberOfPoolRequest{
				PeerID:    peerID, // Use the provided peer ID
				PoolID:    pool.ID,
				ChainName: chainName,
			}

			t.Logf("  Calling HandleIsMemberOfPool with PeerID=%s, PoolID=%d, Chain=%s",
				membershipReq.PeerID, membershipReq.PoolID, membershipReq.ChainName)

			membershipResp, err := bl.HandleIsMemberOfPool(ctx, membershipReq)
			if err != nil {
				t.Logf("  ERROR checking pool membership: %v", err)
				continue // Try next pool
			}

			t.Logf("  Result: IsMember=%t, MemberAddress=%s",
				membershipResp.IsMember, membershipResp.MemberAddress)

			if membershipResp.IsMember {
				topicStr := strconv.Itoa(int(pool.ID))
				t.Logf("\n*** FOUND POOL MEMBERSHIP ***")
				t.Logf("  Chain: %s", chainName)
				t.Logf("  Pool ID: %d", pool.ID)
				t.Logf("  Pool Name: %s", pool.Name)
				t.Logf("  Topic String: %s", topicStr)
				t.Logf("  Member Address: %s", membershipResp.MemberAddress)

				found = true
				foundChain = chainName
				foundPoolID = pool.ID
				break
			}
		}
	}

	// Final result (same logging as Start())
	t.Logf("\n=== Pool Discovery Result ===")
	if found {
		t.Logf("SUCCESS: Pool discovery completed successfully")
		t.Logf("  Pool Name (topic): %s", strconv.Itoa(int(foundPoolID)))
		t.Logf("  Chain Name: %s", foundChain)
	} else {
		t.Logf("FAILED: Could not find pool membership on any chain")
		t.Logf("  PeerID: %s", peerID)
		t.Logf("  Chains checked: %v", chainList)
	}
}

// TestPoolDiscoveryWithSpecificPool tests membership for a specific pool ID
// Run with: go test -v -run TestPoolDiscoveryWithSpecificPool -peerid=<peer_id> -poolid=<pool_id> -chain=<chain>
func TestPoolDiscoveryWithSpecificPool(t *testing.T) {
	flag.Parse()

	peerID := *testPeerID
	if peerID == "" {
		peerID = "12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1"
	}

	poolID := uint32(*testPoolID)
	if poolID == 0 {
		poolID = 1 // Default to pool 1
	}

	chainName := *testChain
	if chainName == "" {
		chainName = "skale" // Default to skale
	}

	timeout := time.Duration(*testTimeout) * time.Second

	t.Logf("=== Direct Pool Membership Test ===")
	t.Logf("PeerID: %s", peerID)
	t.Logf("Pool ID: %d", poolID)
	t.Logf("Chain: %s", chainName)

	bl := createTestBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Direct membership check (bypassing pool discovery)
	membershipReq := blockchain.IsMemberOfPoolRequest{
		PeerID:    peerID,
		PoolID:    poolID,
		ChainName: chainName,
	}

	t.Logf("\nCalling HandleIsMemberOfPool...")
	membershipResp, err := bl.HandleIsMemberOfPool(ctx, membershipReq)
	if err != nil {
		t.Fatalf("HandleIsMemberOfPool failed: %v", err)
	}

	t.Logf("\n=== Result ===")
	t.Logf("IsMember: %t", membershipResp.IsMember)
	t.Logf("MemberAddress: %s", membershipResp.MemberAddress)
	t.Logf("PoolID: %d", membershipResp.PoolID)
	t.Logf("ChainName: %s", membershipResp.ChainName)

	if membershipResp.IsMember {
		t.Logf("\nSUCCESS: PeerID is a member of pool %d on %s", poolID, chainName)
	} else {
		t.Logf("\nNOT FOUND: PeerID is NOT a member of pool %d on %s", poolID, chainName)
	}
}

// TestListAllPools lists all pools on all chains without checking membership
// Run with: go test -v -run TestListAllPools [-chain=<chain>]
func TestListAllPools(t *testing.T) {
	flag.Parse()

	chainFilter := *testChain
	timeout := time.Duration(*testTimeout) * time.Second

	bl := createTestBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	chainList := []string{"skale", "base"}
	if chainFilter != "" {
		chainList = []string{chainFilter}
	}

	t.Logf("=== Listing All Pools ===")

	for _, chainName := range chainList {
		t.Logf("\n--- Chain: %s ---", chainName)

		poolList, err := bl.HandleEVMPoolList(ctx, chainName)
		if err != nil {
			t.Logf("ERROR getting pool list: %v", err)
			continue
		}

		t.Logf("Found %d pools:", len(poolList.Pools))
		for i, pool := range poolList.Pools {
			t.Logf("  [%d] Pool ID: %d", i, pool.ID)
			t.Logf("      Name: %s", pool.Name)
			t.Logf("      Creator: %s", pool.Creator)
			t.Logf("      Members: %d/%d", pool.MemberCount, pool.MaxMembers)
			t.Logf("      Region: %s", pool.Region)
		}
	}
}

// TestDebugPoolDiscoveryStep performs step-by-step debugging of pool discovery
// Run with: go test -v -run TestDebugPoolDiscoveryStep -peerid=<peer_id>
func TestDebugPoolDiscoveryStep(t *testing.T) {
	flag.Parse()

	peerID := *testPeerID
	if peerID == "" {
		peerID = "12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1"
	}

	timeout := time.Duration(*testTimeout) * time.Second

	bl := createTestBlockchain(t)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Logf("=== Debug Pool Discovery ===")
	t.Logf("PeerID: %s", peerID)

	chainList := []string{"skale", "base"}

	for _, chainName := range chainList {
		t.Logf("\n========================================")
		t.Logf("CHAIN: %s", chainName)
		t.Logf("========================================")

		// Step 1: Get pool list
		t.Logf("\n[STEP 1] Getting pool list...")
		poolList, err := bl.HandleEVMPoolList(ctx, chainName)
		if err != nil {
			t.Logf("[STEP 1] FAILED: %v", err)
			continue
		}
		t.Logf("[STEP 1] SUCCESS: Found %d pools", len(poolList.Pools))

		// Step 2: Check each pool
		for _, pool := range poolList.Pools {
			t.Logf("\n[STEP 2] Checking pool %d (%s)...", pool.ID, pool.Name)

			// Step 2a: Prepare request
			req := blockchain.IsMemberOfPoolRequest{
				PeerID:    peerID,
				PoolID:    pool.ID,
				ChainName: chainName,
			}
			t.Logf("[STEP 2a] Request prepared: %+v", req)

			// Step 2b: Call membership check
			t.Logf("[STEP 2b] Calling HandleIsMemberOfPool...")
			resp, err := bl.HandleIsMemberOfPool(ctx, req)
			if err != nil {
				t.Logf("[STEP 2b] FAILED: %v", err)
				continue
			}
			t.Logf("[STEP 2b] SUCCESS: Response=%+v", resp)

			// Step 2c: Evaluate result
			if resp.IsMember {
				t.Logf("[STEP 2c] *** MEMBER FOUND ***")
				t.Logf("  Pool ID: %d", pool.ID)
				t.Logf("  Chain: %s", chainName)
				t.Logf("  Address: %s", resp.MemberAddress)
				t.Logf("  Topic would be: %s", strconv.Itoa(int(pool.ID)))
				return // Found membership, exit
			} else {
				t.Logf("[STEP 2c] Not a member of this pool")
			}
		}
	}

	t.Logf("\n========================================")
	t.Logf("FINAL: No pool membership found for PeerID %s", peerID)
	t.Logf("========================================")
}

// createTestBlockchain creates a blockchain instance for testing
func createTestBlockchain(t *testing.T) *blockchain.FxBlockchain {
	// Create a minimal blockchain instance that can make EVM calls
	// This doesn't require a full libp2p host since we're only testing EVM calls
	bl, err := blockchain.NewTestBlockchain(30)
	if err != nil {
		t.Fatalf("Failed to create blockchain: %v", err)
	}
	return bl
}

// Example usage:
//
// Test full pool discovery flow (same as Blox.Start):
//   go test -v -run TestPoolDiscoveryFlow -peerid=12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1
//
// Test specific pool on specific chain:
//   go test -v -run TestPoolDiscoveryWithSpecificPool -peerid=12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1 -poolid=1 -chain=skale
//
// List all pools:
//   go test -v -run TestListAllPools
//
// Debug step-by-step:
//   go test -v -run TestDebugPoolDiscoveryStep -peerid=12D3KooWGjK8GLeFxYQmthm5rNmbvbiA3S4zbYorzAA63RhKaYc1
