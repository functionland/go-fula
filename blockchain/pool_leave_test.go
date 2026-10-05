package blockchain

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/functionland/go-fula/blockchain/abi"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeChain answers the pool contract's eth_calls for every chain RPC (the chain URLs are hard-coded, so the
// http.Client transport is swapped instead).
type fakeChain struct {
	mu      sync.Mutex
	member  bool  // isPeerIdMemberOfPool result
	status  uint8 // joinRequests(...).status
	failRPC bool  // answer every call with a JSON-RPC error
	calls   int
}

func (f *fakeChain) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	var body struct {
		ID     interface{}     `json:"id"`
		Params json.RawMessage `json:"params"`
	}
	raw, _ := io.ReadAll(req.Body)
	_ = json.Unmarshal(raw, &body)
	var params []map[string]string
	_ = json.Unmarshal(body.Params, &params)
	data := ""
	if len(params) > 0 {
		data = params[0]["data"]
	}

	resp := map[string]interface{}{"jsonrpc": "2.0", "id": body.ID}
	word := func(v uint64) string { return fmt.Sprintf("%064x", v) }
	switch {
	case f.failRPC:
		resp["error"] = map[string]interface{}{"code": -32000, "message": "upstream unavailable"}
	case strings.HasPrefix(data, abi.MethodSignatures.IsPeerIdMemberOfPool):
		isMember := uint64(0)
		if f.member {
			isMember = 1
		}
		resp["result"] = "0x" + word(isMember) + word(0)
	case strings.HasPrefix(data, abi.MethodSignatures.JoinRequests):
		resp["result"] = "0x" + strings.Repeat(word(0), 6) + word(uint64(f.status)) + word(0)
	default:
		resp["error"] = map[string]interface{}{"code": -32601, "message": "unexpected call " + data}
	}
	out, _ := json.Marshal(resp)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(out)),
		Request:    req,
	}, nil
}

type poolConfig struct {
	mu        sync.Mutex
	pool      string
	chain     string
	cleared   chan struct{}
	hookCalls int
}

func (c *poolConfig) getPool() string  { c.mu.Lock(); defer c.mu.Unlock(); return c.pool }
func (c *poolConfig) getChain() string { c.mu.Lock(); defer c.mu.Unlock(); return c.chain }
func (c *poolConfig) setPool(p string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pool = p
	return nil
}
func (c *poolConfig) setChain(ch string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.chain = ch
	return nil
}

func newLeaveTestBlockchain(t *testing.T, chain *fakeChain, cfg *poolConfig, withClusterPeer bool) *FxBlockchain {
	t.Helper()
	PoolJoinedAtFilePath = filepath.Join(t.TempDir(), "pool_joined_at.tmp")
	poolLeaveCheckInterval = 10 * time.Millisecond
	poolConfigClearedRestartDelay = 0

	opts := []Option{
		WithTimeout(30),
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
	}
	if withClusterPeer {
		priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		id, err := peer.IDFromPrivateKey(priv)
		require.NoError(t, err)
		opts = append(opts, WithClusterPeerID(id))
	}
	bl, err := NewFxBlockchain(NewSimpleKeyStorer(""), opts...)
	require.NoError(t, err)
	bl.ch = &http.Client{Transport: chain}
	return bl
}

func leave(bl *FxBlockchain, poolID int, chainName string) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"pool_id":%d,"chain_name":%q}`, poolID, chainName)
	req := httptest.NewRequest(http.MethodPost, "/"+actionPoolLeave, strings.NewReader(body))
	rec := httptest.NewRecorder()
	bl.HandlePoolLeave(http.MethodPost, actionPoolLeave, "", rec, req)
	return rec
}

func waitCleared(t *testing.T, cfg *poolConfig) {
	t.Helper()
	select {
	case <-cfg.cleared:
	case <-time.After(2 * time.Second):
		t.Fatal("pool-config-cleared hook was not called")
	}
}

func TestHandlePoolLeave_NotMemberClearsConfigAndRestarts(t *testing.T) {
	cfg := &poolConfig{pool: "1", chain: "skale", cleared: make(chan struct{}, 1)}
	bl := newLeaveTestBlockchain(t, &fakeChain{}, cfg, true)
	recordPoolJoinTime(time.Now()) // a leave clears even a fresh join

	rec := leave(bl, 1, "skale")

	assert.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	var res PoolLeaveResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	assert.Equal(t, 1, res.PoolID)
	assert.Equal(t, "skale", res.ChainName)
	assert.Equal(t, "0", cfg.getPool())
	assert.Equal(t, "", cfg.getChain())
	assert.Equal(t, "0", bl.topicName)
	_, ok := PoolJoinedAt()
	assert.False(t, ok, "join marker is removed on leave")
	waitCleared(t, cfg)
}

func TestHandlePoolLeave_StillMemberOrPendingIs409(t *testing.T) {
	for name, chain := range map[string]*fakeChain{
		"member":  {member: true},
		"pending": {status: 1},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := &poolConfig{pool: "1", chain: "base", cleared: make(chan struct{}, 1)}
			bl := newLeaveTestBlockchain(t, chain, cfg, true)

			rec := leave(bl, 1, "base")

			assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "remove it on-chain first")
			assert.Equal(t, "1", cfg.getPool())
			assert.Equal(t, "base", cfg.getChain())
			assert.Equal(t, 0, cfg.hookCalls)
		})
	}
}

func TestHandlePoolLeave_RechecksBeforeGivingUp(t *testing.T) {
	cfg := &poolConfig{pool: "1", chain: "base", cleared: make(chan struct{}, 1)}
	chain := &fakeChain{member: true}
	bl := newLeaveTestBlockchain(t, chain, cfg, true)

	rec := leave(bl, 1, "base")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, poolLeaveCheckAttempts, chain.calls, "one isPeerIdMemberOfPool read per attempt")
}

func TestHandlePoolLeave_OtherConfiguredPoolIs409WithoutRPC(t *testing.T) {
	cfg := &poolConfig{pool: "2", chain: "skale", cleared: make(chan struct{}, 1)}
	chain := &fakeChain{}
	bl := newLeaveTestBlockchain(t, chain, cfg, true)

	rec := leave(bl, 1, "skale")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "configured for pool 2")
	assert.Equal(t, 0, chain.calls)
	assert.Equal(t, "2", cfg.getPool())
}

func TestHandlePoolLeave_UnverifiableIs503AndKeepsConfig(t *testing.T) {
	t.Run("rpc error", func(t *testing.T) {
		cfg := &poolConfig{pool: "1", chain: "skale", cleared: make(chan struct{}, 1)}
		bl := newLeaveTestBlockchain(t, &fakeChain{failRPC: true}, cfg, true)
		rec := leave(bl, 1, "skale")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "1", cfg.getPool())
		assert.Equal(t, 0, cfg.hookCalls)
	})
	t.Run("no cluster peer id", func(t *testing.T) {
		cfg := &poolConfig{pool: "1", chain: "skale", cleared: make(chan struct{}, 1)}
		bl := newLeaveTestBlockchain(t, &fakeChain{}, cfg, false)
		rec := leave(bl, 1, "skale")
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "1", cfg.getPool())
	})
}

func TestReconcilePoolConfig(t *testing.T) {
	t.Run("not a member and no join marker: cleared", func(t *testing.T) {
		cfg := &poolConfig{pool: "1", chain: "skale", cleared: make(chan struct{}, 1)}
		bl := newLeaveTestBlockchain(t, &fakeChain{}, cfg, true)
		cleared, err := bl.ReconcilePoolConfig(t.Context())
		require.NoError(t, err)
		assert.True(t, cleared)
		assert.Equal(t, "0", cfg.getPool())
		assert.Equal(t, "", cfg.getChain())
		waitCleared(t, cfg)
	})
	t.Run("recent join: kept without reading the chain", func(t *testing.T) {
		cfg := &poolConfig{pool: "1", chain: "skale", cleared: make(chan struct{}, 1)}
		chain := &fakeChain{}
		bl := newLeaveTestBlockchain(t, chain, cfg, true)
		recordPoolJoinTime(time.Now().Add(-time.Hour))
		cleared, err := bl.ReconcilePoolConfig(t.Context())
		require.NoError(t, err)
		assert.False(t, cleared)
		assert.Equal(t, "1", cfg.getPool())
		assert.Equal(t, 0, chain.calls)
	})
	t.Run("join older than the grace period: cleared", func(t *testing.T) {
		cfg := &poolConfig{pool: "1", chain: "base", cleared: make(chan struct{}, 1)}
		bl := newLeaveTestBlockchain(t, &fakeChain{}, cfg, true)
		recordPoolJoinTime(time.Now().Add(-PoolJoinGracePeriod - time.Minute))
		cleared, err := bl.ReconcilePoolConfig(t.Context())
		require.NoError(t, err)
		assert.True(t, cleared)
		waitCleared(t, cfg)
	})
	t.Run("still a member or pending: kept", func(t *testing.T) {
		for _, chain := range []*fakeChain{{member: true}, {status: 1}} {
			cfg := &poolConfig{pool: "1", chain: "skale", cleared: make(chan struct{}, 1)}
			bl := newLeaveTestBlockchain(t, chain, cfg, true)
			cleared, err := bl.ReconcilePoolConfig(t.Context())
			require.NoError(t, err)
			assert.False(t, cleared)
			assert.Equal(t, "1", cfg.getPool())
		}
	})
	t.Run("rpc error: kept, error returned", func(t *testing.T) {
		cfg := &poolConfig{pool: "1", chain: "skale", cleared: make(chan struct{}, 1)}
		bl := newLeaveTestBlockchain(t, &fakeChain{failRPC: true}, cfg, true)
		cleared, err := bl.ReconcilePoolConfig(t.Context())
		assert.Error(t, err)
		assert.False(t, cleared)
		assert.Equal(t, "1", cfg.getPool())
		assert.Equal(t, 0, cfg.hookCalls)
	})
	t.Run("no pool configured: nothing to do", func(t *testing.T) {
		cfg := &poolConfig{pool: "0", chain: "", cleared: make(chan struct{}, 1)}
		chain := &fakeChain{}
		bl := newLeaveTestBlockchain(t, chain, cfg, true)
		cleared, err := bl.ReconcilePoolConfig(t.Context())
		require.NoError(t, err)
		assert.False(t, cleared)
		assert.Equal(t, 0, chain.calls)
	})
}

func TestHandlePoolJoinRecordsJoinTime(t *testing.T) {
	PoolJoinedAtFilePath = filepath.Join(t.TempDir(), "pool_joined_at.tmp")
	before := time.Now().Add(-time.Second)
	recordPoolJoinTime(time.Now())
	at, ok := PoolJoinedAt()
	require.True(t, ok)
	assert.True(t, at.After(before))
	clearPoolJoinTime()
	_, err := os.Stat(PoolJoinedAtFilePath)
	assert.True(t, os.IsNotExist(err))
}
