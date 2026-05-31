package fulamobile

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newNoopClient builds an in-process client with the no-op exchange (no network,
// no BloxAddr required) for lifecycle tests.
func newNoopClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(&Config{
		Exchange:  "noop",
		StorePath: t.TempDir(),
		PoolName:  "1",
	})
	require.NoError(t, err)
	require.NotNil(t, c)
	return c
}

// TestConnectToBloxAfterShutdownReturnsErrClosed is the regression test for the
// use-during-shutdown crash (Android SIGABRT): after Shutdown, host-using ops
// must be rejected with ErrClientClosed rather than proceeding to touch the
// freed libp2p host/datastore.
//
// Pre-fix this FAILS: ConnectToBlox on a noop client returns nil after Shutdown
// (it never checks a closed flag), so the operation would proceed against a
// torn-down client on the real (fx) exchange.
func TestConnectToBloxAfterShutdownReturnsErrClosed(t *testing.T) {
	c := newNoopClient(t)

	require.NoError(t, c.Shutdown())

	err := c.ConnectToBlox()
	require.ErrorIs(t, err, ErrClientClosed,
		"ConnectToBlox after Shutdown must return ErrClientClosed, not proceed to use the closed client")
}

// TestPingAfterShutdownReturnsErrClosed is the Ping counterpart — Ping uses the
// same host (ensureConnected + libp2p ping) and must also be rejected once the
// client is closed.
func TestPingAfterShutdownReturnsErrClosed(t *testing.T) {
	c := newNoopClient(t)

	require.NoError(t, c.Shutdown())

	_, err := c.Ping()
	require.ErrorIs(t, err, ErrClientClosed)
}

// TestShutdownCancelsAndDrainsInflightOp proves the anti-crash property: a
// Shutdown concurrent with an in-flight host-using operation must (1) cancel the
// operation's context promptly (not wait out its full 60s timeout) and (2) wait
// for it to finish before closing the host/datastore. Uses beginOp directly so
// it does not depend on real network timing.
func TestShutdownCancelsAndDrainsInflightOp(t *testing.T) {
	c := newNoopClient(t)

	started := make(chan struct{})
	returned := make(chan error, 1)
	go func() {
		ctx, done, err := c.beginOp()
		if err != nil {
			returned <- err
			return
		}
		defer done()
		close(started)
		<-ctx.Done() // block until Shutdown cancels us
		returned <- ctx.Err()
	}()

	<-started // ensure the op is admitted (inflight.Add done) before Shutdown

	start := time.Now()
	shutDone := make(chan error, 1)
	go func() { shutDone <- c.Shutdown() }()

	select {
	case err := <-returned:
		require.ErrorIs(t, err, context.Canceled,
			"in-flight op must be cancelled by Shutdown")
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight op was not cancelled by Shutdown within 2s")
	}

	select {
	case err := <-shutDone:
		require.NoError(t, err)
		require.Less(t, time.Since(start), 5*time.Second,
			"Shutdown should return promptly after draining, not wait out a 60s op timeout")
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not drain the in-flight op within 5s")
	}

	// After a drained Shutdown, new ops are rejected.
	require.ErrorIs(t, c.ConnectToBlox(), ErrClientClosed)
}

// TestShutdownIsIdempotent ensures repeated Shutdown calls are safe (the second
// is a no-op), matching how the mobile bridge may call shutdown more than once.
func TestShutdownIsIdempotent(t *testing.T) {
	c := newNoopClient(t)
	require.NoError(t, c.Shutdown())
	require.NoError(t, c.Shutdown())
}
