package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/functionland/go-fula/wap/pkg/config"
)

// writeConfig points FULA_CONFIG_PATH at a temp file with the given authorizer ("" writes no authorizer key),
// restoring the original path when the test ends.
func writeConfig(t *testing.T, authorizer string, write bool) {
	t.Helper()
	orig := config.FULA_CONFIG_PATH
	t.Cleanup(func() { config.FULA_CONFIG_PATH = orig })
	path := filepath.Join(t.TempDir(), "config.yaml")
	if write {
		body := "storeDir: /uniondrive\n"
		if authorizer != "" {
			body += "authorizer: " + authorizer + "\n"
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	config.FULA_CONFIG_PATH = path
}

func TestBloxHasOwner(t *testing.T) {
	t.Run("no config file at all → unowned (a fresh box)", func(t *testing.T) {
		writeConfig(t, "", false)
		if bloxHasOwner() {
			t.Fatal("a missing config must read as unowned, otherwise first-time LAN setup is impossible")
		}
	})

	t.Run("config with no authorizer → unowned", func(t *testing.T) {
		writeConfig(t, "", true)
		if bloxHasOwner() {
			t.Fatal("empty authorizer must read as unowned")
		}
	})

	t.Run("config with an authorizer → owned", func(t *testing.T) {
		writeConfig(t, "12D3KooWPnaMDrD7QLZKiT2iktjm9Kucx7XEPrSCUS6TTBbYuiRj", true)
		if !bloxHasOwner() {
			t.Fatal("a set authorizer must read as owned")
		}
	})
}

// The security property that makes the LAN listener acceptable: /peer/exchange can be called repeatedly, so
// the listener must stop serving the moment an owner exists — not merely at the moment it was created.
func TestLANSetupGuardClosesAfterOwnershipIsSet(t *testing.T) {
	served := 0
	h := lanSetupGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served++
		w.WriteHeader(http.StatusOK)
	}))

	writeConfig(t, "", true)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/peer/exchange", nil))
	if rec.Code != http.StatusOK || served != 1 {
		t.Fatalf("unowned box must accept setup: code=%d served=%d", rec.Code, served)
	}

	// Same running listener, box now claimed.
	writeConfig(t, "12D3KooWPnaMDrD7QLZKiT2iktjm9Kucx7XEPrSCUS6TTBbYuiRj", true)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/peer/exchange", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("an owned box must refuse LAN setup, got %d", rec.Code)
	}
	if served != 1 {
		t.Fatalf("request reached the handler after ownership was set (served=%d)", served)
	}
}

func TestStartLANSetupListenersSkippedWhenOwned(t *testing.T) {
	writeConfig(t, "12D3KooWPnaMDrD7QLZKiT2iktjm9Kucx7XEPrSCUS6TTBbYuiRj", true)
	if ls := startLANSetupListeners(http.NewServeMux(), "0", "10.42.0.1"); ls != nil {
		for _, l := range ls {
			_ = l.Close()
		}
		t.Fatal("an owned box must not open LAN setup listeners at all")
	}
}

// A settled box must look exactly as it did before this field existed: anything reading /properties or
// /readiness today sees no new keys unless kubo is genuinely mid-identity-change.
func TestAddKuboIdentityStateIsSilentWhenSettled(t *testing.T) {
	t.Run("no live id → nothing added", func(t *testing.T) {
		out := map[string]interface{}{}
		addKuboIdentityState(out, "")
		if len(out) != 0 {
			t.Fatalf("expected no fields, got %v", out)
		}
	})

	// With no readable kubo config (the usual case in a unit test) the helper must stay quiet rather than
	// guess that an identity change is under way.
	t.Run("unreadable kubo config → nothing added", func(t *testing.T) {
		out := map[string]interface{}{}
		addKuboIdentityState(out, "12D3KooWLive")
		if _, pending := out["kubo_identity_pending"]; pending {
			t.Fatalf("must not report a pending identity when the config cannot be read: %v", out)
		}
	})
}

// The startup wait is bounded, so the watcher is what keeps a box reachable when its AP comes up later —
// which is the normal case (FxBlox has autoconnect=no, and restarting fula_go drops it). Without this, a user
// who joins the hotspot on a healthy box gets nothing on 10.42.0.1:3500 until yet another restart.
func TestWatchForHotspotBindsWhenTheAddressAppears(t *testing.T) {
	// Occupy the address so the first attempts fail exactly as they do while the AP is down.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := blocker.Addr().String()

	mc := &multiCloser{}
	stop := make(chan struct{})
	defer close(stop)

	restore := hotspotWatchIntervalForTests(20 * time.Millisecond)
	defer restore()

	watchForHotspot(stop, mc, http.NewServeMux(), addr)

	// Still taken: the watcher must not have grabbed anything yet.
	time.Sleep(60 * time.Millisecond)
	mc.mu.Lock()
	early := len(mc.listeners)
	mc.mu.Unlock()
	if early != 0 {
		t.Fatalf("watcher bound while the address was still occupied (%d listeners)", early)
	}

	// The "AP appears": the address frees up.
	_ = blocker.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mc.mu.Lock()
		n := len(mc.listeners)
		mc.mu.Unlock()
		if n > 0 {
			_ = mc.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("watcher never bound the address after it became available")
}

// The watcher must outlive the function that starts it.
//
// Serve() runs under `ctx, cancel := context.WithTimeout(...)` + `defer cancel()`, so any context it owns is
// cancelled the instant it returns. A watcher wired to that context would exit on its first tick — the fix
// would look present in the code and do absolutely nothing on the device. startHotspotWatch therefore takes no
// context at all; this test pins that behaviour by starting the watch from a function that cancels its own
// context on the way out, exactly as Serve does.
func TestHotspotWatchOutlivesTheFunctionThatStartedIt(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := blocker.Addr().String()

	mc := &multiCloser{}
	defer func() { _ = mc.Close() }()

	restore := hotspotWatchIntervalForTests(20 * time.Millisecond)
	defer restore()

	// Stand-in for Serve(): owns a context, starts the watch, cancels on return.
	func() {
		_, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		startHotspotWatch(mc, http.NewServeMux(), addr)
	}()

	// The "AP appears" well after that function returned.
	time.Sleep(60 * time.Millisecond)
	_ = blocker.Close()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mc.mu.Lock()
		n := len(mc.listeners)
		mc.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the watcher stopped when its starter returned, so a late hotspot is never served")
}

// Close() must stop the watcher, or a closed server keeps racing to re-bind the port it just released.
func TestCloseStopsTheHotspotWatch(t *testing.T) {
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := blocker.Addr().String()

	mc := &multiCloser{}
	restore := hotspotWatchIntervalForTests(10 * time.Millisecond)
	defer restore()

	startHotspotWatch(mc, http.NewServeMux(), addr)
	if err := mc.Close(); err != nil {
		t.Fatal(err)
	}

	// Free the address; a stopped watcher must not take it.
	_ = blocker.Close()
	time.Sleep(200 * time.Millisecond)

	probe, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("the watcher grabbed the address after Close(): %v", err)
	}
	_ = probe.Close()
}

// A listener opened while the server is shutting down must not be kept, or it would hold the port open.
func TestMultiCloserRefusesListenersAfterClose(t *testing.T) {
	mc := &multiCloser{}
	_ = mc.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if mc.add(ln) {
		t.Fatal("add() accepted a listener after Close()")
	}
	// add() must have closed it: a second close returns an error on an already-closed listener.
	if cerr := ln.Close(); cerr == nil {
		t.Fatal("add() did not close the listener it refused")
	}
}

func TestSkipLANInterface(t *testing.T) {
	for _, name := range []string{"lo", "docker0", "br-afcaa0d61177", "veth24873a8", "dummy0"} {
		if !skipLANInterface(name) {
			t.Errorf("%s should be skipped", name)
		}
	}
	for _, name := range []string{"eth0", "enx00e04c505a48", "wlan0", "end0"} {
		if skipLANInterface(name) {
			t.Errorf("%s is a real LAN interface and should not be skipped", name)
		}
	}
}

// The hotspot address is bound separately; binding it twice here would fail and log noise.
func TestLANListenAddrsExcludesTheHotspot(t *testing.T) {
	for _, addr := range lanListenAddrs("3500", "10.42.0.1") {
		if addr == "10.42.0.1:3500" {
			t.Fatal("the hotspot address must not be included in the LAN set")
		}
	}
}
