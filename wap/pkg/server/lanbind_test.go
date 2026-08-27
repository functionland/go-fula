package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

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
