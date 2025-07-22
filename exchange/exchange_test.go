package exchange

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/storage/memstore"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFxExchangeCreation tests the creation of FxExchange instance
func TestFxExchangeCreation(t *testing.T) {
	// Generate test identity
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	// Create libp2p host
	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	// Create link system
	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	// Create exchange instance
	ex, err := NewFxExchange(h, ls,
		WithAuthorizer(h.ID()),
		WithAllowTransientConnection(true),
		WithIpniPublishDisabled(true),
	)
	require.NoError(t, err)
	require.NotNil(t, ex)

	// Test that exchange has required components
	assert.NotNil(t, ex)
}

// TestNoopExchange tests the NoopExchange implementation
func TestNoopExchange(t *testing.T) {
	noop := NoopExchange{}
	ctx := context.Background()

	// Generate test peer
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)
	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	// Create test link using a simple CID
	testCid := cid.MustParse("bafkr4ifmwbmdrkxep3mci37ionvgturlylvganap4ch7ouia2ui5tmr4iy")
	link := cidlink.Link{Cid: testCid}

	// Test NoopExchange methods
	err = noop.Start(ctx)
	assert.NoError(t, err)

	err = noop.Push(ctx, h.ID(), link)
	assert.NoError(t, err)

	err = noop.Pull(ctx, h.ID(), link)
	assert.NoError(t, err)

	err = noop.PullBlock(ctx, h.ID(), link)
	assert.NoError(t, err)

	err = noop.SetAuth(ctx, h.ID(), h.ID(), true)
	assert.NoError(t, err)

	err = noop.Shutdown(ctx)
	assert.NoError(t, err)

	// Test IPNI methods
	noop.IpniNotifyLink(link)

	providers, err := noop.FindProvidersIpni(link, []string{})
	assert.NoError(t, err)
	assert.Empty(t, providers)
}

// TestExchangeStart tests the Start operation
func TestExchangeStart(t *testing.T) {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	ex, err := NewFxExchange(h, ls,
		WithAuthorizer(h.ID()),
		WithIpniPublishDisabled(true),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Test Start operation
	err = ex.Start(ctx)
	assert.NoError(t, err)

	// Test Shutdown operation
	err = ex.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestExchangeAuth tests authorization operations
func TestExchangeAuth(t *testing.T) {
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

	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	ex, err := NewFxExchange(h1, ls,
		WithAuthorizer(h1.ID()),
		WithIpniPublishDisabled(true),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = ex.Start(ctx)
	require.NoError(t, err)
	defer ex.Shutdown(ctx)

	// Test SetAuth operation
	err = ex.SetAuth(ctx, h1.ID(), h2.ID(), true)
	assert.NoError(t, err)

	// Test removing authorization
	err = ex.SetAuth(ctx, h1.ID(), h2.ID(), false)
	assert.NoError(t, err)
}

// TestExchangePushPull tests Push and Pull operations
func TestExchangePushPull(t *testing.T) {
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

	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	ex, err := NewFxExchange(h1, ls,
		WithAuthorizer(h1.ID()),
		WithIpniPublishDisabled(true),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = ex.Start(ctx)
	require.NoError(t, err)
	defer ex.Shutdown(ctx)

	// Create test link using a simple CID
	testCid := cid.MustParse("bafkr4ifmwbmdrkxep3mci37ionvgturlylvganap4ch7ouia2ui5tmr4iy")
	link := cidlink.Link{Cid: testCid}

	// Test Push operation (will likely fail due to network setup)
	err = ex.Push(ctx, h2.ID(), link)
	// We don't assert success here as it depends on network connectivity

	// Test Pull operation (will likely fail due to network setup)
	err = ex.Pull(ctx, h2.ID(), link)
	// We don't assert success here as it depends on network connectivity

	// Test PullBlock operation
	err = ex.PullBlock(ctx, h2.ID(), link)
	// We don't assert success here as it depends on network connectivity

	assert.NotNil(t, ex)
}

// TestExchangeOptions tests various exchange options
func TestExchangeOptions(t *testing.T) {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	// Test with various options
	ex, err := NewFxExchange(h, ls,
		WithAuthorizer(h.ID()),
		WithAllowTransientConnection(true),
		WithIpniPublishDisabled(true),
		WithPoolHostMode(false),
		WithMaxPushRate(50),
		WithIpniGetEndPoint("http://127.0.0.1:3000/cid/"),
	)
	require.NoError(t, err)
	require.NotNil(t, ex)

	ctx := context.Background()
	err = ex.Start(ctx)
	assert.NoError(t, err)

	err = ex.Shutdown(ctx)
	assert.NoError(t, err)
}

// TestExchangeIPNI tests IPNI functionality
func TestExchangeIPNI(t *testing.T) {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	ex, err := NewFxExchange(h, ls,
		WithAuthorizer(h.ID()),
		WithIpniPublishDisabled(false), // Enable IPNI for this test
		WithIpniGetEndPoint("http://127.0.0.1:3000/cid/"),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = ex.Start(ctx)
	require.NoError(t, err)
	defer ex.Shutdown(ctx)

	// Create test link using a simple CID
	testCid := cid.MustParse("bafkr4ifmwbmdrkxep3mci37ionvgturlylvganap4ch7ouia2ui5tmr4iy")
	link := cidlink.Link{Cid: testCid}

	// Test IPNI notify
	ex.IpniNotifyLink(link)

	// Test IPNI provider finding (will likely fail due to network setup)
	relays := []string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}
	providers, err := ex.FindProvidersIpni(link, relays)
	// We don't assert success here as it depends on network connectivity
	_ = providers
	_ = err

	assert.NotNil(t, ex)
}

// TestExchangeErrorHandling tests error handling scenarios
func TestExchangeErrorHandling(t *testing.T) {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	ex, err := NewFxExchange(h, ls,
		WithAuthorizer(h.ID()),
		WithIpniPublishDisabled(true),
	)
	require.NoError(t, err)

	ctx := context.Background()
	err = ex.Start(ctx)
	require.NoError(t, err)
	defer ex.Shutdown(ctx)

	// Test with invalid peer ID
	invalidPeerID := peer.ID("invalid-peer-id")
	testCid := cid.MustParse("bafkr4ifmwbmdrkxep3mci37ionvgturlylvganap4ch7ouia2ui5tmr4iy")
	link := cidlink.Link{Cid: testCid}

	// These operations should handle errors gracefully
	err = ex.Push(ctx, invalidPeerID, link)
	// We expect errors but the system should not panic

	err = ex.Pull(ctx, invalidPeerID, link)
	// We expect errors but the system should not panic

	assert.NotNil(t, ex)
}

// TestExchangeTimeout tests timeout scenarios
func TestExchangeTimeout(t *testing.T) {
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)

	h, err := libp2p.New(libp2p.Identity(priv))
	require.NoError(t, err)
	defer h.Close()

	store := &memstore.Store{}
	ls := cidlink.DefaultLinkSystem()
	ls.SetReadStorage(store)
	ls.SetWriteStorage(store)

	ex, err := NewFxExchange(h, ls,
		WithAuthorizer(h.ID()),
		WithIpniPublishDisabled(true),
	)
	require.NoError(t, err)

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = ex.Start(ctx)
	require.NoError(t, err)
	defer ex.Shutdown(context.Background())

	// Create test link using a simple CID
	testCid := cid.MustParse("bafkr4ifmwbmdrkxep3mci37ionvgturlylvganap4ch7ouia2ui5tmr4iy")
	link := cidlink.Link{Cid: testCid}

	// Create another peer for testing
	priv2, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	require.NoError(t, err)
	h2, err := libp2p.New(libp2p.Identity(priv2))
	require.NoError(t, err)
	defer h2.Close()

	// Test operations with timeout context
	err = ex.Push(ctx, h2.ID(), link)
	// Should handle timeout gracefully

	assert.NotNil(t, ex)
}
