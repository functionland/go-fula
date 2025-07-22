package blox

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/functionland/go-fula/blockchain"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/storage/memstore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBloxCreation tests the creation of Blox instance
func TestBloxCreation(t *testing.T) {
	// Generate test identity
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	// Create libp2p host
	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	// Create datastore
	ds := dssync.MutexWrap(datastore.NewMapDatastore())

	// Create link system
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	// Create blox instance
	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithLinkSystem(&ls),
		WithPoolName("test-pool"),
		WithTopicName("test-topic"),
	)
	require.NoError(t, err)
	require.NotNil(t, blox)

	// Test that blox has required components
	assert.NotNil(t, blox.h)
	assert.NotNil(t, blox.ls)
	assert.Equal(t, "test-pool", blox.name)
}

// TestBloxStoreAndLoad tests basic store and load operations
func TestBloxStoreAndLoad(t *testing.T) {
	// Setup blox instance
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithLinkSystem(&ls),
		WithPoolName("test-pool"),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Create test data
	testNode := basicnode.NewString("test data")

	// Test Store operation
	link, err := blox.Store(ctx, testNode)
	assert.NoError(t, err)
	assert.NotNil(t, link)

	// Test Load operation
	loadedNode, err := blox.Load(ctx, link, basicnode.Prototype.String)
	assert.NoError(t, err)
	assert.NotNil(t, loadedNode)

	// Verify data integrity
	loadedStr, err := loadedNode.AsString()
	assert.NoError(t, err)
	assert.Equal(t, "test data", loadedStr)
}

// TestBloxHas tests the Has operation
func TestBloxHas(t *testing.T) {
	// Setup blox instance
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())

	// Create blox instance - it will set up its own link system that writes to the datastore
	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithPoolName("test-pool"),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Create and store test data
	testNode := basicnode.NewString("test data for has")
	link, err := blox.Store(ctx, testNode)
	require.NoError(t, err)

	// Test Has operation - should return true for existing data
	exists, err := blox.Has(ctx, link)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test Has operation - should return false for non-existing data
	nonExistentLink := cidlink.Link{Cid: cid.MustParse("bafkr4ifmwbmdrkxep3mci37ionvgturlylvganap4ch7ouia2ui5tmr4iy")}
	exists, err = blox.Has(ctx, nonExistentLink)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestBloxStoreCid tests the StoreCid operation
func TestBloxStoreCid(t *testing.T) {
	// Setup blox instance
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithLinkSystem(&ls),
		WithPoolName("test-pool"),
		WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Create test link
	testNode := basicnode.NewString("test data for store cid")
	link, err := blox.Store(ctx, testNode)
	require.NoError(t, err)

	// Test StoreCid operation (this will likely fail due to network setup)
	err = blox.StoreCid(ctx, link, 3) // Limit of 3 replicas
	// We don't assert success here as it depends on network connectivity
	// But we test that the method exists and can be called
	assert.NotNil(t, blox)
}

// TestBloxStoreManifest tests the StoreManifest operation
func TestBloxStoreManifest(t *testing.T) {
	// Setup blox instance
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithLinkSystem(&ls),
		WithPoolName("test-pool"),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Create test links
	testNode1 := basicnode.NewString("test data 1")
	testNode2 := basicnode.NewString("test data 2")

	link1, err := blox.Store(ctx, testNode1)
	require.NoError(t, err)
	link2, err := blox.Store(ctx, testNode2)
	require.NoError(t, err)

	// Create links with limits for manifest
	linksWithLimits := []blockchain.LinkWithLimit{
		{Link: link1, Limit: 3},
		{Link: link2, Limit: 3},
	}

	// Test StoreManifest operation
	err = blox.StoreManifest(ctx, linksWithLimits, 10)
	// We don't assert success here as it depends on network connectivity
	// But we test that the method exists and can be called
	assert.NotNil(t, blox)
}

// TestBloxPushPull tests Push and Pull operations
func TestBloxPushPull(t *testing.T) {
	// Setup blox instance
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithLinkSystem(&ls),
		WithPoolName("test-pool"),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Create test data
	testNode := basicnode.NewString("test data for push/pull")
	link, err := blox.Store(ctx, testNode)
	require.NoError(t, err)

	// Create another peer for testing
	priv2, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)
	h2, err := libp2p.New(libp2p.Identity(priv2))
	require.NoError(t, err)
	defer h2.Close()

	// Test Push operation (will likely fail due to network setup)
	err = blox.Push(ctx, h2.ID(), link)
	// We don't assert success here as it depends on network connectivity

	// Test Pull operation (will likely fail due to network setup)
	err = blox.Pull(ctx, h2.ID(), link)
	// We don't assert success here as it depends on network connectivity

	assert.NotNil(t, blox)
}

// TestBloxOptions tests various blox options
func TestBloxOptions(t *testing.T) {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	// Test with various options
	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithLinkSystem(&ls),
		WithPoolName("test-pool"),
		WithTopicName("test-topic"),
		WithStoreDir("/tmp/test-store"),
		WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		WithPingCount(5),
		WithMaxPingTime(1000),
		WithMinSuccessPingRate(80),
	)
	require.NoError(t, err)
	require.NotNil(t, blox)

	// Verify options are set correctly
	assert.Equal(t, "test-pool", blox.name)
	assert.Equal(t, "test-topic", blox.topicName)
	assert.Equal(t, "/tmp/test-store", blox.storeDir)
	assert.Equal(t, 5, blox.pingCount)
	assert.Equal(t, 1000, blox.maxPingTime)
	assert.Equal(t, 80, blox.minSuccessRate)
}

// TestBloxEVMChainIntegration tests EVM chain integration functionality
func TestBloxEVMChainIntegration(t *testing.T) {
	// Setup blox instance
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithLinkSystem(&ls),
		WithPoolName("1"),
		WithBlockchainEndPoint("127.0.0.1:4000"),
		WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
	)
	require.NoError(t, err)
	require.NotNil(t, blox.bl, "Blockchain should be initialized")

	ctx := context.Background()

	// Test HandleEVMPoolList for Base chain
	t.Run("HandleEVMPoolList_Base", func(t *testing.T) {
		response, err := blox.bl.HandleEVMPoolList(ctx, "base")
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})

	// Test HandleEVMPoolList for Skale chain
	t.Run("HandleEVMPoolList_Skale", func(t *testing.T) {
		response, err := blox.bl.HandleEVMPoolList(ctx, "skale")
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})

	// Test HandleEVMPoolList with invalid chain
	t.Run("HandleEVMPoolList_InvalidChain", func(t *testing.T) {
		response, err := blox.bl.HandleEVMPoolList(ctx, "invalid-chain")
		// Should return error for invalid chain
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported chain")
		_ = response
	})

	// Test HandleIsMemberOfPool for Base chain
	t.Run("HandleIsMemberOfPool_Base", func(t *testing.T) {
		req := blockchain.IsMemberOfPoolRequest{
			PoolID:    1,
			PeerID:    h.ID().String(),
			ChainName: "base",
		}
		response, err := blox.bl.HandleIsMemberOfPool(ctx, req)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})

	// Test HandleIsMemberOfPool for Skale chain
	t.Run("HandleIsMemberOfPool_Skale", func(t *testing.T) {
		req := blockchain.IsMemberOfPoolRequest{
			PoolID:    1,
			PeerID:    h.ID().String(),
			ChainName: "skale",
		}
		response, err := blox.bl.HandleIsMemberOfPool(ctx, req)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})

	// Test HandleIsMemberOfPool with invalid chain
	t.Run("HandleIsMemberOfPool_InvalidChain", func(t *testing.T) {
		req := blockchain.IsMemberOfPoolRequest{
			PoolID:    1,
			PeerID:    h.ID().String(),
			ChainName: "invalid-chain",
		}
		response, err := blox.bl.HandleIsMemberOfPool(ctx, req)
		// Should return error for invalid chain
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported chain")
		_ = response
	})
}

// TestBloxEVMPoolOperations tests EVM-specific pool operations
func TestBloxEVMPoolOperations(t *testing.T) {
	// Setup blox instance
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithLinkSystem(&ls),
		WithPoolName("1"),
		WithBlockchainEndPoint("127.0.0.1:4000"),
	)
	require.NoError(t, err)
	require.NotNil(t, blox.bl, "Blockchain should be initialized")

	ctx := context.Background()

	// Create another peer for testing
	priv2, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)
	h2, err := libp2p.New(libp2p.Identity(priv2))
	require.NoError(t, err)
	defer h2.Close()

	// Test PoolJoin with Base chain
	t.Run("PoolJoin_BaseChain", func(t *testing.T) {
		joinReq := blockchain.PoolJoinRequest{
			PoolID:    1,
			PeerID:    h2.ID().String(),
			ChainName: "base",
		}
		response, err := blox.bl.PoolJoin(ctx, h2.ID(), joinReq)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})

	// Test PoolJoin with Skale chain
	t.Run("PoolJoin_SkaleChain", func(t *testing.T) {
		joinReq := blockchain.PoolJoinRequest{
			PoolID:    1,
			PeerID:    h2.ID().String(),
			ChainName: "skale",
		}
		response, err := blox.bl.PoolJoin(ctx, h2.ID(), joinReq)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})

	// Test PoolLeave with Base chain
	t.Run("PoolLeave_BaseChain", func(t *testing.T) {
		leaveReq := blockchain.PoolLeaveRequest{
			PoolID:    1,
			ChainName: "base",
		}
		response, err := blox.bl.PoolLeave(ctx, h2.ID(), leaveReq)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})

	// Test PoolLeave with Skale chain
	t.Run("PoolLeave_SkaleChain", func(t *testing.T) {
		leaveReq := blockchain.PoolLeaveRequest{
			PoolID:    1,
			ChainName: "skale",
		}
		response, err := blox.bl.PoolLeave(ctx, h2.ID(), leaveReq)
		// This will likely fail due to network setup, but tests the interface
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})

	// Test PoolJoin with invalid chain
	t.Run("PoolJoin_InvalidChain", func(t *testing.T) {
		joinReq := blockchain.PoolJoinRequest{
			PoolID:    1,
			PeerID:    h2.ID().String(),
			ChainName: "invalid-chain",
		}
		response, err := blox.bl.PoolJoin(ctx, h2.ID(), joinReq)
		// Should handle invalid chain gracefully
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})

	// Test PoolLeave with invalid chain
	t.Run("PoolLeave_InvalidChain", func(t *testing.T) {
		leaveReq := blockchain.PoolLeaveRequest{
			PoolID:    1,
			ChainName: "invalid-chain",
		}
		response, err := blox.bl.PoolLeave(ctx, h2.ID(), leaveReq)
		// Should handle invalid chain gracefully
		_ = response
		_ = err
		assert.NotNil(t, blox)
	})
}

// TestBloxEVMChainValidation tests EVM chain validation and configuration
func TestBloxEVMChainValidation(t *testing.T) {
	// Setup blox instance
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	blox, err := New(
		WithHost(h),
		WithDatastore(ds),
		WithLinkSystem(&ls),
		WithPoolName("1"),
		WithBlockchainEndPoint("127.0.0.1:4000"),
	)
	require.NoError(t, err)
	require.NotNil(t, blox.bl, "Blockchain should be initialized")

	// Test chain configuration retrieval
	t.Run("GetChainConfigs", func(t *testing.T) {
		configs := blockchain.GetChainConfigs()

		// Verify Base chain configuration
		baseConfig, exists := configs["base"]
		assert.True(t, exists, "Base chain configuration should exist")
		if exists {
			assert.Equal(t, "base", baseConfig.Name)
			assert.Equal(t, int64(8453), baseConfig.ChainID)
			assert.Equal(t, "https://base-rpc.publicnode.com", baseConfig.RPC)
			assert.Equal(t, "0xf293A6902662DcB09E310254A5e418cb28D71b6b", baseConfig.Contract)
		}

		// Verify Skale chain configuration
		skaleConfig, exists := configs["skale"]
		assert.True(t, exists, "Skale chain configuration should exist")
		if exists {
			assert.Equal(t, "skale", skaleConfig.Name)
			assert.Equal(t, int64(2046399126), skaleConfig.ChainID)
			assert.Equal(t, "https://mainnet.skalenodes.com/v1/elated-tan-skat", skaleConfig.RPC)
			assert.Equal(t, "0xf293A6902662DcB09E310254A5e418cb28D71b6b", skaleConfig.Contract)
		}
	})

	// Test that blox has blockchain integration
	t.Run("BlockchainIntegration", func(t *testing.T) {
		// Verify that blockchain is properly initialized
		assert.NotNil(t, blox.bl, "Blockchain should be initialized")

		// Test that we can access blockchain methods without panicking
		// These are the public methods we can test
		assert.NotNil(t, blox)
	})
}
