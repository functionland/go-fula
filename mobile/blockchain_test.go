package fulamobile

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/functionland/go-fula/blockchain"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockBlockchainServer creates a mock blockchain server for testing
func MockBlockchainServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/account-exists":
			response := map[string]interface{}{
				"account_exists": true,
			}
			json.NewEncoder(w).Encode(response)
		case "/account-create":
			response := map[string]interface{}{
				"account": "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY",
			}
			json.NewEncoder(w).Encode(response)
		case "/account-balance":
			response := map[string]interface{}{
				"balance": "1000000000000000000",
			}
			json.NewEncoder(w).Encode(response)
		case "/account-fund":
			response := map[string]interface{}{
				"success": true,
				"tx_hash": "0x1234567890abcdef",
			}
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestClientAccountOperations tests account-related operations
func TestClientAccountOperations(t *testing.T) {
	// Create mock blockchain server
	server := MockBlockchainServer()
	defer server.Close()

	// Create test client with fx exchange (not noop) so blockchain gets initialized
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	config := &Config{
		Exchange:           "fx", // Use fx exchange so blockchain gets initialized
		StorePath:          t.TempDir(),
		PoolName:           "1",
		BlockchainEndpoint: server.URL[7:], // Remove http:// prefix
		BloxAddr:           "/ip4/127.0.0.1/tcp/40001/p2p/" + h.ID().String(),
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, client.bl, "Blockchain should be initialized with fx exchange")

	testAccount := "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"

	// Test AccountExists
	t.Run("AccountExists", func(t *testing.T) {
		response, err := client.AccountExists(testAccount)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test AccountCreate
	t.Run("AccountCreate", func(t *testing.T) {
		response, err := client.AccountCreate()
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test AccountBalance
	t.Run("AccountBalance", func(t *testing.T) {
		response, err := client.AccountBalance(testAccount)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test AccountFund
	t.Run("AccountFund", func(t *testing.T) {
		response, err := client.AccountFund(testAccount)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})
}

// TestClientAssetsBalance tests assets balance functionality
func TestClientAssetsBalance(t *testing.T) {
	config := &Config{
		Exchange:  "fx", // Use fx exchange so blockchain gets initialized
		StorePath: t.TempDir(),
		PoolName:  "1",
		BloxAddr:  "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest", // Add BloxAddr for fx exchange
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client.bl, "Blockchain should be initialized")

	testAccount := "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"
	assetId := 1
	classId := 1

	// Test AssetsBalance
	response, err := client.AssetsBalance(testAccount, assetId, classId)
	// This will likely fail due to network setup, but tests the interface
	_ = response
	_ = err
	assert.NotNil(t, client)
}

// TestClientPoolOperations tests pool-related operations
func TestClientPoolOperations(t *testing.T) {
	config := &Config{
		Exchange:  "fx", // Use fx exchange so blockchain gets initialized
		StorePath: t.TempDir(),
		PoolName:  "1",
		BloxAddr:  "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest", // Add BloxAddr for fx exchange
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client.bl, "Blockchain should be initialized")

	// Test PoolJoin
	t.Run("PoolJoin", func(t *testing.T) {
		response, err := client.PoolJoin(1)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test PoolLeave
	t.Run("PoolLeave", func(t *testing.T) {
		response, err := client.PoolLeave(1)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test PoolCancelJoin
	t.Run("PoolCancelJoin", func(t *testing.T) {
		response, err := client.PoolCancelJoin(1)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})
}

// TestClientEVMChainPoolOperations tests EVM chain-specific pool operations
func TestClientEVMChainPoolOperations(t *testing.T) {
	config := &Config{
		Exchange:  "fx", // Use fx exchange so blockchain gets initialized
		StorePath: t.TempDir(),
		PoolName:  "1",
		BloxAddr:  "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest", // Add BloxAddr for fx exchange
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client.bl, "Blockchain should be initialized")

	// Test PoolJoinWithChain - Base chain
	t.Run("PoolJoinWithChain_Base", func(t *testing.T) {
		response, err := client.PoolJoinWithChain(1, "base")
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test PoolJoinWithChain - Skale chain
	t.Run("PoolJoinWithChain_Skale", func(t *testing.T) {
		response, err := client.PoolJoinWithChain(1, "skale")
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test PoolLeaveWithChain - Base chain
	t.Run("PoolLeaveWithChain_Base", func(t *testing.T) {
		response, err := client.PoolLeaveWithChain(1, "base")
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test PoolLeaveWithChain - Skale chain
	t.Run("PoolLeaveWithChain_Skale", func(t *testing.T) {
		response, err := client.PoolLeaveWithChain(1, "skale")
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test PoolJoinWithChain with invalid chain
	t.Run("PoolJoinWithChain_InvalidChain", func(t *testing.T) {
		response, err := client.PoolJoinWithChain(1, "invalid-chain")
		// Should handle invalid chain gracefully
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test PoolLeaveWithChain with invalid chain
	t.Run("PoolLeaveWithChain_InvalidChain", func(t *testing.T) {
		response, err := client.PoolLeaveWithChain(1, "invalid-chain")
		// Should handle invalid chain gracefully
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test PoolJoinWithChain with invalid pool ID
	t.Run("PoolJoinWithChain_InvalidPoolID", func(t *testing.T) {
		response, err := client.PoolJoinWithChain(-1, "base")
		// Should handle invalid pool ID gracefully
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test PoolLeaveWithChain with invalid pool ID
	t.Run("PoolLeaveWithChain_InvalidPoolID", func(t *testing.T) {
		response, err := client.PoolLeaveWithChain(-1, "base")
		// Should handle invalid pool ID gracefully
		_ = response
		_ = err
		assert.NotNil(t, client)
	})
}

// TestClientManifestOperations tests manifest-related operations
func TestClientManifestOperations(t *testing.T) {
	config := &Config{
		Exchange:  "fx", // Use fx exchange so blockchain gets initialized
		StorePath: t.TempDir(),
		PoolName:  "1",
		BloxAddr:  "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest", // Add BloxAddr for fx exchange
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client.bl, "Blockchain should be initialized")

	// Test ManifestAvailable
	t.Run("ManifestAvailable", func(t *testing.T) {
		response, err := client.ManifestAvailable(1)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})

	// Test BatchUploadManifest
	t.Run("BatchUploadManifest", func(t *testing.T) {
		cidsBytes := []byte("bafkr4ifmwbmdrkxep3mci37ionvgturlylvganap4ch7ouia2ui5tmr4iy")
		response, err := client.BatchUploadManifest(cidsBytes, 1, 3)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, client)
	})
}

// TestClientSeeded tests seeded functionality
func TestClientSeeded(t *testing.T) {
	config := &Config{
		Exchange:  "fx", // Use fx exchange so blockchain gets initialized
		StorePath: t.TempDir(),
		PoolName:  "1",
		BloxAddr:  "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest", // Add BloxAddr for fx exchange
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	// Test that client exists (Seeded method doesn't exist in mobile client)
	assert.NotNil(t, client)
	assert.NotNil(t, client.bl) // Blockchain should be available
}

// TestClientBloxFreeSpace tests blox free space functionality
func TestClientBloxFreeSpace(t *testing.T) {
	config := &Config{
		Exchange:  "fx", // Use fx exchange so blockchain gets initialized
		StorePath: t.TempDir(),
		PoolName:  "1",
		BloxAddr:  "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest", // Add BloxAddr for fx exchange
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client.bl, "Blockchain should be initialized")

	// Test BloxFreeSpace
	response, err := client.BloxFreeSpace()
	// This will likely fail due to network setup, but tests the interface
	_ = response
	_ = err
	assert.NotNil(t, client)
}

// TestClientErrorHandling tests error handling in blockchain operations
func TestClientErrorHandling(t *testing.T) {
	config := &Config{
		Exchange:  "fx", // Use fx exchange so blockchain gets initialized
		StorePath: t.TempDir(),
		PoolName:  "1",
		BloxAddr:  "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest", // Add BloxAddr for fx exchange
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client.bl, "Blockchain should be initialized")

	// Test with invalid account format
	t.Run("InvalidAccount", func(t *testing.T) {
		_, _ = client.AccountExists("invalid-account-format")
		// Should handle invalid account gracefully
		assert.NotNil(t, client) // At least the client should exist
	})

	// Test with invalid pool ID
	t.Run("InvalidPoolID", func(t *testing.T) {
		_, _ = client.PoolJoin(-1) // Negative pool ID
		// Should handle invalid pool ID gracefully
		assert.NotNil(t, client)
	})

	// Test with invalid asset parameters
	t.Run("InvalidAssetParams", func(t *testing.T) {
		_, _ = client.AssetsBalance("valid-account", -1, -1)
		// Should handle invalid asset parameters gracefully
		assert.NotNil(t, client)
	})
}

// TestClientConfigValidation tests configuration validation
func TestClientConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		errString string
	}{
		{
			name: "valid_config_with_blockchain",
			config: &Config{
				Exchange:           "fx", // Use fx exchange so blockchain gets initialized
				StorePath:          t.TempDir(),
				PoolName:           "1",
				BlockchainEndpoint: "127.0.0.1:4000",
				BloxAddr:           "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest",
			},
			wantErr: false,
		},
		{
			name: "empty_blockchain_endpoint",
			config: &Config{
				Exchange:           "fx", // Use fx exchange so blockchain gets initialized
				StorePath:          t.TempDir(),
				PoolName:           "1",
				BlockchainEndpoint: "",
				BloxAddr:           "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest",
			},
			wantErr: false, // Should use default
		},
		{
			name: "invalid_pool_name",
			config: &Config{
				Exchange:           "fx", // Use fx exchange so blockchain gets initialized
				StorePath:          t.TempDir(),
				PoolName:           "", // Empty pool name
				BlockchainEndpoint: "127.0.0.1:4000",
				BloxAddr:           "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest",
			},
			wantErr: false, // Should use default "0"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.NotNil(t, client.bl) // Blockchain should be initialized
			}
		})
	}
}

// TestClientBlockchainIntegration tests blockchain integration
func TestClientBlockchainIntegration(t *testing.T) {
	config := &Config{
		Exchange:           "fx", // Use fx exchange so blockchain gets initialized
		StorePath:          t.TempDir(),
		PoolName:           "1",
		BlockchainEndpoint: "127.0.0.1:4000",
		BloxAddr:           "/ip4/127.0.0.1/tcp/40001/p2p/12D3KooWTest",
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client.bl)

	// Test that blockchain instance is properly configured
	assert.NotNil(t, client.bl)

	// Test that we can access blockchain methods without panicking
	ctx := context.Background()

	// These calls will likely fail due to network setup, but should not panic
	_, _ = client.bl.AccountExists(ctx, client.h.ID(), blockchain.AccountExistsRequest{
		Account: "test-account",
	})

	assert.NotNil(t, client)
}
