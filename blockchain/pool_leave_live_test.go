//go:build livechain

// Read-only checks of the pool-leave path against the real SKALE and Base pool contracts (needs network, so it is
// not part of the normal test run):
//
//	go test -tags livechain -run TestLive -v ./blockchain/
//
// LIVE_MEMBER_PEER_ID / LIVE_CHAIN pick a cluster peer that is a member of pool 1 on that chain (defaults: a SKALE
// pool-1 member). LIVE_OWN_PEER_ID is a cluster peer expected NOT to be in a pool there (default: a fresh random
// key) — on a test Blox, pass the device's own cluster peer id. Nothing here touches a real config or restarts a
// service: the pool-name callbacks and the restart hook are fakes.
package blockchain

import (
	"crypto/rand"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultLiveMemberPeerID = "12D3KooWMax6UNK5k9Wf1HTRdqJonSmk9khAxC4JUjJMpXFq7w88" // pool 1 on SKALE

func liveEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func livePeer(t *testing.T, s string) peer.ID {
	t.Helper()
	if s == "" {
		priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		id, err := peer.IDFromPrivateKey(priv)
		require.NoError(t, err)
		return id
	}
	id, err := peer.Decode(s)
	require.NoError(t, err)
	return id
}

// newLiveBlockchain uses the real chain RPCs (default http client) with fake config callbacks.
func newLiveBlockchain(t *testing.T, clusterPeer peer.ID, cfg *poolConfig) *FxBlockchain {
	t.Helper()
	dir := t.TempDir()
	PoolMemberConfirmedFilePath = dir + "/pool_member_confirmed.tmp"
	PoolReconcileClearedAtFilePath = dir + "/pool_reconcile_cleared_at.tmp"
	PoolReconcileDisabledFilePath = dir + "/disable_pool_reconcile"
	poolLeaveCheckInterval = 2 * time.Second
	poolConfigClearedRestartDelay = 0
	poolReconcileConfirmDelay = 2 * time.Second
	bl, err := NewFxBlockchain(NewSimpleKeyStorer(""),
		WithTimeout(60),
		WithClusterPeerID(clusterPeer),
		WithGetPoolName(cfg.getPool),
		WithUpdatePoolName(cfg.setPool),
		WithGetChainName(cfg.getChain),
		WithUpdateChainName(cfg.setChain),
		WithOnPoolConfigCleared(func() {
			cfg.mu.Lock()
			cfg.hookCalls++
			cfg.mu.Unlock()
			cfg.cleared <- struct{}{}
		}),
	)
	require.NoError(t, err)
	return bl
}

func TestLiveClusterPeerPoolStatus(t *testing.T) {
	chain := liveEnv("LIVE_CHAIN", "skale")
	member := livePeer(t, liveEnv("LIVE_MEMBER_PEER_ID", defaultLiveMemberPeerID))
	own := livePeer(t, os.Getenv("LIVE_OWN_PEER_ID"))

	cfg := &poolConfig{pool: "1", chain: chain, cleared: make(chan struct{}, 1)}
	isMember, pending, err := newLiveBlockchain(t, member, cfg).ClusterPeerPoolStatus(t.Context(), 1, chain)
	require.NoError(t, err)
	assert.True(t, isMember, "known member peer %s should be in pool 1 on %s", member, chain)
	assert.False(t, pending)

	isMember, pending, err = newLiveBlockchain(t, own, cfg).ClusterPeerPoolStatus(t.Context(), 1, chain)
	require.NoError(t, err)
	assert.False(t, isMember, "%s should not be in pool 1 on %s", own, chain)
	assert.False(t, pending)
}

func TestLiveHandlePoolLeave(t *testing.T) {
	chain := liveEnv("LIVE_CHAIN", "skale")
	member := livePeer(t, liveEnv("LIVE_MEMBER_PEER_ID", defaultLiveMemberPeerID))
	own := livePeer(t, os.Getenv("LIVE_OWN_PEER_ID"))

	t.Run("not in the pool on-chain: 202 and config cleared", func(t *testing.T) {
		cfg := &poolConfig{pool: "1", chain: chain, cleared: make(chan struct{}, 1)}
		bl := newLiveBlockchain(t, own, cfg)
		rec := leave(bl, 1, chain)
		assert.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
		assert.Equal(t, "0", cfg.getPool())
		assert.Equal(t, "", cfg.getChain())
		waitCleared(t, cfg)
	})
	t.Run("still a member on-chain: 409 after re-checks, config kept", func(t *testing.T) {
		cfg := &poolConfig{pool: "1", chain: chain, cleared: make(chan struct{}, 1)}
		bl := newLiveBlockchain(t, member, cfg)
		rec := leave(bl, 1, chain)
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, "1", cfg.getPool())
		assert.Equal(t, 0, cfg.hookCalls)
	})
	t.Run("configured for another pool: 409", func(t *testing.T) {
		cfg := &poolConfig{pool: "2", chain: chain, cleared: make(chan struct{}, 1)}
		rec := leave(newLiveBlockchain(t, own, cfg), 1, chain)
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Equal(t, "2", cfg.getPool())
	})
	t.Run("chain unreachable: 503, config kept", func(t *testing.T) {
		cfg := &poolConfig{pool: "1", chain: chain, cleared: make(chan struct{}, 1)}
		bl := newLiveBlockchain(t, own, cfg)
		bl.ch = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network is unreachable")
		})}
		rec := leave(bl, 1, chain)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
		assert.Equal(t, "1", cfg.getPool())
	})
}

func TestLiveReconcilePoolConfig(t *testing.T) {
	chain := liveEnv("LIVE_CHAIN", "skale")
	member := livePeer(t, liveEnv("LIVE_MEMBER_PEER_ID", defaultLiveMemberPeerID))
	own := livePeer(t, os.Getenv("LIVE_OWN_PEER_ID"))

	cfg := &poolConfig{pool: "1", chain: chain, cleared: make(chan struct{}, 1)}
	cleared, err := newLiveBlockchain(t, member, cfg).ReconcilePoolConfig(t.Context())
	require.NoError(t, err)
	assert.False(t, cleared, "a real member keeps its pool")
	confirmed, ok := readPoolMemberConfirmed()
	assert.True(t, ok)
	assert.Equal(t, chain+":1", confirmed, "and its membership is recorded")

	cfg = &poolConfig{pool: "1", chain: chain, cleared: make(chan struct{}, 1)}
	cleared, err = newLiveBlockchain(t, own, cfg).ReconcilePoolConfig(t.Context())
	require.NoError(t, err)
	assert.False(t, cleared, "a pool never seen as a membership is left alone")

	cfg = &poolConfig{pool: "1", chain: chain, cleared: make(chan struct{}, 1)}
	bl := newLiveBlockchain(t, own, cfg)
	recordPoolMemberConfirmed(chain, "1") // it was a member once, and the chain no longer lists it
	cleared, err = bl.ReconcilePoolConfig(t.Context())
	require.NoError(t, err)
	assert.True(t, cleared, "a former member that left has the pool cleared")
	waitCleared(t, cfg)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
