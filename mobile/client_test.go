package fulamobile

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"

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
			name: "invalid_store_path",
			config: &Config{
				Exchange:  "noop",
				StorePath: "/invalid/path/that/does/not/exist",
				PoolName:  "1",
			},
			wantErr: true,
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

				// Test basic client properties
				assert.NotEmpty(t, client.ID())
				assert.NotNil(t, client.h)
				assert.NotNil(t, client.ds)
				assert.NotNil(t, client.ex)
			}
		})
	}
}

// TestClientDataOperations tests Put/Get operations
func TestClientDataOperations(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: true,
	}

	client, err := NewClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Test data storage and retrieval
	testData := []byte("Hello, World!")
	codec := int64(0x55) // Raw codec

	// Test Put operation
	linkBytes, err := client.Put(testData, codec)
	assert.NoError(t, err)
	assert.NotNil(t, linkBytes)
	assert.NotEmpty(t, linkBytes)

	// Test Get operation
	retrievedData, err := client.Get(linkBytes)
	assert.NoError(t, err)
	assert.Equal(t, testData, retrievedData)
}

// TestClientDataOperationsWithDifferentCodecs tests various codecs
func TestClientDataOperationsWithDifferentCodecs(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: true,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	tests := []struct {
		name  string
		data  []byte
		codec int64
	}{
		{
			name:  "raw_data",
			data:  []byte("raw data test"),
			codec: 0x55,
		},
		{
			name:  "json_data",
			data:  []byte(`{"test": "json data"}`),
			codec: 0x0129, // JSON codec
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store data
			linkBytes, err := client.Put(tt.data, tt.codec)
			assert.NoError(t, err)
			assert.NotNil(t, linkBytes)

			// Retrieve data
			retrievedData, err := client.Get(linkBytes)
			assert.NoError(t, err)
			assert.Equal(t, tt.data, retrievedData)
		})
	}
}

// TestClientPushPull tests Push/Pull operations
func TestClientPushPull(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: true,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	// Store some data first
	testData := []byte("test data for push/pull")
	linkBytes, err := client.Put(testData, 0x55)
	require.NoError(t, err)

	// Test Push operation (with noop exchange, this should succeed)
	err = client.Push(linkBytes)
	assert.NoError(t, err)

	// Test Pull operation (with noop exchange, this should succeed if data exists locally)
	err = client.Pull(linkBytes)
	assert.NoError(t, err)
}

// TestClientLargeData tests operations with large data
func TestClientLargeData(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: true,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	// Create large test data (1MB)
	largeData := make([]byte, 1024*1024)
	_, err = rand.Read(largeData)
	require.NoError(t, err)

	// Test storing large data
	linkBytes, err := client.Put(largeData, 0x55)
	assert.NoError(t, err)
	assert.NotNil(t, linkBytes)

	// Test retrieving large data
	retrievedData, err := client.Get(linkBytes)
	assert.NoError(t, err)
	assert.Equal(t, largeData, retrievedData)
}

// TestClientConcurrentOperations tests concurrent data operations
func TestClientConcurrentOperations(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: true,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	// Number of concurrent operations
	numOps := 10

	// Channel to collect results
	results := make(chan error, numOps)

	// Start concurrent Put operations
	for i := 0; i < numOps; i++ {
		go func(index int) {
			data := []byte("concurrent test data " + string(rune(index)))
			linkBytes, err := client.Put(data, 0x55)
			if err != nil {
				results <- err
				return
			}

			// Verify we can retrieve the data
			retrievedData, err := client.Get(linkBytes)
			if err != nil {
				results <- err
				return
			}

			if !bytes.Equal(data, retrievedData) {
				results <- assert.AnError
				return
			}

			results <- nil
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < numOps; i++ {
		select {
		case err := <-results:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
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

	// Store some data
	testData := []byte("test data for flush")
	_, err = client.Put(testData, 0x55)
	require.NoError(t, err)

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

// TestClientInvalidOperations tests error handling
func TestClientInvalidOperations(t *testing.T) {
	config := &Config{
		Exchange:   "noop",
		StorePath:  t.TempDir(),
		PoolName:   "1",
		SyncWrites: true,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	// Test Get with invalid link
	invalidLink := []byte("invalid link data")
	_, err = client.Get(invalidLink)
	assert.Error(t, err)

	// Test Push with invalid link
	err = client.Push(invalidLink)
	assert.Error(t, err)

	// Test Pull with invalid link
	err = client.Pull(invalidLink)
	assert.Error(t, err)

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
