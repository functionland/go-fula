package blockchain

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	testNasBloxPeerID = "12D3KooWTestBloxPeerIDForNasCredentials"
	testNasPassword   = "abcde-fghjk-mnpqr-stuvw"
	testNasCreds      = `{"version":1,"username":"fxnas","password":"` + testNasPassword +
		`","share":"SharedFolder","created_at":"2026-10-03T00:00:00Z","rotated_at":null}`
)

func newNasTestPeer(t *testing.T) (crypto.PrivKey, peer.ID) {
	t.Helper()
	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return priv, pid
}

// stubNasEnv points the handler at a temp credentials file (nil = absent) and
// a fixed kubo peer ID, restoring the real ones afterwards.
func stubNasEnv(t *testing.T, content *string, kuboErr error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "credentials.json")
	if content != nil {
		if err := os.WriteFile(path, []byte(*content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	origPath, origKubo := nasCredentialsPath, kuboPeerIDFunc
	nasCredentialsPath = func() string { return path }
	kuboPeerIDFunc = func() (string, error) {
		if kuboErr != nil {
			return "", kuboErr
		}
		return testNasBloxPeerID, nil
	}
	t.Cleanup(func() { nasCredentialsPath, kuboPeerIDFunc = origPath, origKubo })
}

func newNasRequest(t *testing.T, bloxPeerID string) *http.Request {
	t.Helper()
	body, err := json.Marshal(NasCredentialsRequest{BloxPeerID: bloxPeerID})
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewRequest(http.MethodPost, "http://blox.invalid/"+actionNasCredentials, bytes.NewReader(body))
}

func decodeNasResponse(t *testing.T, rec *httptest.ResponseRecorder) NasCredentialsResponse {
	t.Helper()
	var resp NasCredentialsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	return resp
}

func TestAuthorizedNasCredentialsIsOwnerOnly(t *testing.T) {
	_, owner := newNasTestPeer(t)
	_, delegated := newNasTestPeer(t)
	_, stranger := newNasTestPeer(t)

	bl := &FxBlockchain{
		options:         &options{authorizer: owner},
		authorizedPeers: map[peer.ID]struct{}{delegated: {}, owner: {}},
	}
	if !bl.authorized(owner, actionNasCredentials) {
		t.Error("the owner must be authorized for nas-credentials")
	}
	if bl.authorized(delegated, actionNasCredentials) {
		t.Error("a delegated peer must NOT be authorized for nas-credentials")
	}
	if bl.authorized(stranger, actionNasCredentials) {
		t.Error("an unknown peer must NOT be authorized for nas-credentials")
	}
	// Existing behaviour is unchanged: delegated peers keep their access.
	if !bl.authorized(delegated, actionAutoPinPair) {
		t.Error("delegated peers must still be authorized for auto-pin-pair")
	}

	unowned := &FxBlockchain{
		options:         &options{},
		authorizedPeers: map[peer.ID]struct{}{owner: {}},
	}
	if unowned.authorized(owner, actionNasCredentials) {
		t.Error("with no authorizer set, nobody may read nas-credentials")
	}
}

func TestHandleNasCredentials(t *testing.T) {
	valid := testNasCreds
	malformed := `{"username": "fxnas", "password": `
	noPassword := `{"version":1,"username":"fxnas","share":"SharedFolder"}`

	tests := []struct {
		name       string
		content    *string
		kuboErr    error
		bloxPeerID string
		wantCode   int
		wantStatus string
	}{
		{"ok", &valid, nil, testNasBloxPeerID, http.StatusOK, "ok"},
		{"other blox", &valid, nil, "12D3KooWSomeOtherBlox", http.StatusBadRequest, "blox_peer_mismatch"},
		{"missing blox peer id", &valid, nil, "", http.StatusBadRequest, "bad_request"},
		{"not provisioned", nil, nil, testNasBloxPeerID, http.StatusNotFound, "not_provisioned"},
		{"malformed file", &malformed, nil, testNasBloxPeerID, http.StatusInternalServerError, "malformed"},
		{"file without password", &noPassword, nil, testNasBloxPeerID, http.StatusInternalServerError, "malformed"},
		{"kubo peer id unavailable", &valid, errors.New("kubo down"), testNasBloxPeerID, http.StatusServiceUnavailable, "unavailable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubNasEnv(t, tc.content, tc.kuboErr)
			_, owner := newNasTestPeer(t)
			bl := &FxBlockchain{options: &options{authorizer: owner}}

			rec := httptest.NewRecorder()
			bl.handleNasCredentials(owner, rec, newNasRequest(t, tc.bloxPeerID))

			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tc.wantCode)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
			body := rec.Body.String()
			resp := decodeNasResponse(t, rec)
			if resp.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", resp.Status, tc.wantStatus)
			}
			if tc.wantCode == http.StatusOK {
				if resp.Username != "fxnas" || resp.Password != testNasPassword || resp.Share != "SharedFolder" ||
					resp.CreatedAt != "2026-10-03T00:00:00Z" {
					t.Errorf("unexpected credentials in response: %+v", resp)
				}
			} else if strings.Contains(body, testNasPassword) {
				t.Errorf("error response leaked the password: %s", body)
			}
		})
	}
}

// TestServeProxyNasCredentials drives the real proxy entry point: signature
// verification, authorization, then dispatch.
func TestServeProxyNasCredentials(t *testing.T) {
	valid := testNasCreds
	stubNasEnv(t, &valid, nil)

	ownerKey, owner := newNasTestPeer(t)
	delegatedKey, delegated := newNasTestPeer(t)
	strangerKey, stranger := newNasTestPeer(t)
	bl := &FxBlockchain{
		options:         &options{authorizer: owner},
		authorizedPeers: map[peer.ID]struct{}{delegated: {}},
	}

	tests := []struct {
		name     string
		key      crypto.PrivKey
		pid      peer.ID
		wantCode int
	}{
		{"owner, signed", ownerKey, owner, http.StatusOK},
		{"unsigned", nil, "", http.StatusUnauthorized},
		{"delegated peer, signed", delegatedKey, delegated, http.StatusUnauthorized},
		{"stranger, signed", strangerKey, stranger, http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := newNasRequest(t, testNasBloxPeerID)
			if tc.key != nil {
				if err := signRequest(req, tc.key, tc.pid); err != nil {
					t.Fatal(err)
				}
			}
			rec := httptest.NewRecorder()
			bl.serveProxy(rec, req)
			if rec.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d (body %q)", rec.Code, tc.wantCode, rec.Body.String())
			}
			if tc.wantCode == http.StatusOK {
				if resp := decodeNasResponse(t, rec); resp.Password != testNasPassword {
					t.Errorf("owner did not receive the password: %+v", resp)
				}
			} else if strings.Contains(rec.Body.String(), testNasPassword) {
				t.Errorf("rejected request leaked the password")
			}
		})
	}
}
