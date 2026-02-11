package fulamobile

import (
	"crypto/rand"
	"testing"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewClient tests client creation with various configurations
func TestNewClient(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantErr   bool
		errString string
	}{
		{
			name: "valid_noop_config",
			config: &Config{
				Exchange:   "noop",
				StorePath:  t.TempDir(),
				PoolName:   "1",
				SyncWrites: false,
			},
			wantErr: false,
		},
		{
			name: "missing_blox_addr",
			config: &Config{
				Exchange:  "fx",
				StorePath: t.TempDir(),
				PoolName:  "1",
				BloxAddr:  "", // Missing BloxAddr for non-noop exchange
			},
			wantErr:   true,
			errString: "BloxAddr must be specified",
		},
		{
			name: "empty_store_path_uses_temp",
			config: &Config{
				Exchange:  "noop",
				StorePath: "",
				PoolName:  "1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			// Always try to cleanup client if it was created, even on error
			defer func() {
				if client != nil {
					if shutdownErr := client.Shutdown(); shutdownErr != nil {
						t.Logf("Warning: failed to shutdown client: %v", shutdownErr)
					}
				}
			}()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				// Test basic client properties
				assert.NotEmpty(t, client.ID())
				assert.NotNil(t, client.h)
				assert.NotNil(t, client.ds)
				assert.NotNil(t, client.ex)
			}
		})
	}
}

// TestClientFlush tests the flush operation
func TestClientFlush(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: false, // Disable sync writes to test flush
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer func() {
		if shutdownErr := client.Shutdown(); shutdownErr != nil {
			t.Logf("Warning: failed to shutdown client: %v", shutdownErr)
		}
	}()

	// Test flush operation
	err = client.Flush()
	assert.NoError(t, err)
}

// TestClientSetAuth tests authorization operations
func TestClientSetAuth(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: true,
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer func() {
		if shutdownErr := client.Shutdown(); shutdownErr != nil {
			t.Logf("Warning: failed to shutdown client: %v", shutdownErr)
		}
	}()

	// Generate test peer IDs
	priv1, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)
	priv2, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h1, err := libp2p.New(libp2p.Identity(priv1))
	require.NoError(t, err)
	defer h1.Close()

	h2, err := libp2p.New(libp2p.Identity(priv2))
	require.NoError(t, err)
	defer h2.Close()

	// Test setting authorization
	err = client.SetAuth(h1.ID().String(), h2.ID().String(), true)
	assert.NoError(t, err)

	// Test removing authorization
	err = client.SetAuth(h1.ID().String(), h2.ID().String(), false)
	assert.NoError(t, err)
}

// TestClientInvalidSetAuth tests error handling for invalid SetAuth inputs
func TestClientInvalidSetAuth(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: true,
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer func() {
		if shutdownErr := client.Shutdown(); shutdownErr != nil {
			t.Logf("Warning: failed to shutdown client: %v", shutdownErr)
		}
	}()

	// Test SetAuth with invalid peer IDs
	err = client.SetAuth("invalid-peer-id", "another-invalid-peer-id", true)
	assert.Error(t, err)
}

// TestClientID tests peer ID functionality
func TestClientID(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: true,
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	defer func() {
		if shutdownErr := client.Shutdown(); shutdownErr != nil {
			t.Logf("Warning: failed to shutdown client: %v", shutdownErr)
		}
	}()

	// Test that ID returns a valid peer ID string
	id := client.ID()
	assert.NotEmpty(t, id)

	// Test that the ID is consistent
	id2 := client.ID()
	assert.Equal(t, id, id2)

	// Test that we can decode the peer ID
	_, err = client.h.ID().MarshalText()
	assert.NoError(t, err)
}
