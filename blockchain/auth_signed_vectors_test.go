package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// These tests pin the identity-derivation and request-signing byte formats so browser/JS clients
// (packages/fula-web-client in functionland/fxblox-web) can prove parity with go-fula.
//
// Emit the golden vectors with:
//
//	FULA_EMIT_VECTORS=1 FULA_VECTORS_OUT=/path/to/dir go test ./blockchain -run 'TestVectors' -v
//
// Without FULA_EMIT_VECTORS the tests only assert internal consistency (signing helper == verifySignedRequest).

// keyFromIdentityString mirrors fulamobile.GenerateEd25519KeyFromString (mobile/keygen.go) without importing the
// mobile package (which imports blockchain). mobile/keygen_vectors_test.go asserts the two derivations agree.
func keyFromIdentityString(t *testing.T, secret string) (crypto.PrivKey, peer.ID) {
	t.Helper()
	seed := sha256.Sum256([]byte(secret))
	pk, _, err := crypto.GenerateEd25519Key(bytes.NewReader(seed[:]))
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	pid, err := peer.IDFromPrivateKey(pk)
	if err != nil {
		t.Fatalf("IDFromPrivateKey: %v", err)
	}
	return pk, pid
}

// vectorSecrets returns the identity strings the mobile app would pass to newClient:
// Uint8Array(64).toString() == comma-joined decimals.
func vectorSecrets() []string {
	join := func(b []byte) string {
		parts := make([]string, len(b))
		for i, v := range b {
			parts[i] = strconv.Itoa(int(v))
		}
		return strings.Join(parts, ",")
	}
	asc := make([]byte, 64)
	desc := make([]byte, 64)
	lcg := make([]byte, 64)
	for i := range asc {
		asc[i] = byte(i)
		desc[i] = byte(255 - i)
		lcg[i] = byte((i*73 + 41) % 256)
	}
	return []string{join(asc), join(desc), join(lcg)}
}

// signWithTimestamp reproduces signRequest's construction with an explicit timestamp (signRequest uses time.Now()).
// TestVectorsSigningRoundTrip proves this construction is accepted by verifySignedRequest.
func signWithTimestamp(t *testing.T, pk crypto.PrivKey, action, timestamp string, body []byte) (message string, digest [32]byte, sig []byte) {
	t.Helper()
	bodyHash := sha256.Sum256(body)
	message = action + ":" + timestamp + ":" + base64.StdEncoding.EncodeToString(bodyHash[:])
	digest = sha256.Sum256([]byte(message))
	var err error
	sig, err = pk.Sign(digest[:])
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return message, digest, sig
}

type signingCase struct {
	action, timestamp, body string
}

func signingCases() []signingCase {
	return []signingCase{
		{"blox-free-space", "1756166400", "{}"},
		{"fetch-container-logs", "1756166400", "{\"ContainerName\":\"fula_go\",\"TailCount\":\"50\"}\n"},
		{"account-fund", "1756166400", "{\"amount\":1000000000000000000,\"to\":\"5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty\"}"},
	}
}

// TestVectorsSigningRoundTrip: headers built by signWithTimestamp (current time) and by the real signRequest must
// both pass verifySignedRequest and yield the same peer ID + body.
func TestVectorsSigningRoundTrip(t *testing.T) {
	for _, secret := range vectorSecrets() {
		pk, pid := keyFromIdentityString(t, secret)
		for _, c := range signingCases() {
			// (1) explicit-timestamp construction
			now := strconv.FormatInt(time.Now().Unix(), 10)
			_, _, sig := signWithTimestamp(t, pk, c.action, now, []byte(c.body))
			req, _ := http.NewRequest(http.MethodPost, "http://"+pid.String()+".invalid/"+c.action, strings.NewReader(c.body))
			req.Header.Set(headerPeerID, pid.String())
			req.Header.Set(headerTimestamp, now)
			req.Header.Set(headerSignature, base64.StdEncoding.EncodeToString(sig))
			from, body, err := verifySignedRequest(req)
			if err != nil {
				t.Fatalf("verifySignedRequest(explicit ts) %s: %v", c.action, err)
			}
			if from != pid || string(body) != c.body {
				t.Fatalf("round-trip mismatch for %s", c.action)
			}
			// (2) the production signRequest path
			req2, _ := http.NewRequest(http.MethodPost, "http://"+pid.String()+".invalid/"+c.action, strings.NewReader(c.body))
			if err := signRequest(req2, pk, pid); err != nil {
				t.Fatalf("signRequest: %v", err)
			}
			if from2, _, err := verifySignedRequest(req2); err != nil || from2 != pid {
				t.Fatalf("verifySignedRequest(signRequest) %s: %v", c.action, err)
			}
		}
	}
}

// TestVectorsEmit writes identity.json and signing.json for the JS golden tests (opt-in via FULA_EMIT_VECTORS=1).
func TestVectorsEmit(t *testing.T) {
	if os.Getenv("FULA_EMIT_VECTORS") == "" {
		t.Skip("set FULA_EMIT_VECTORS=1 to emit golden vectors")
	}
	type identityVec struct {
		Secret             string `json:"secret"`
		SeedHex            string `json:"seedHex"`
		PrivKeyProtobufB64 string `json:"privKeyProtobufB64"`
		PubKeyRawHex       string `json:"pubKeyRawHex"`
		PeerID             string `json:"peerId"`
	}
	type signingVec struct {
		Secret       string `json:"secret"`
		PeerID       string `json:"peerId"`
		Action       string `json:"action"`
		Timestamp    string `json:"timestamp"`
		Body         string `json:"body"`
		Message      string `json:"message"`
		DigestHex    string `json:"digestHex"`
		SignatureB64 string `json:"signatureB64"`
	}
	var ids []identityVec
	var sigs []signingVec
	for _, secret := range vectorSecrets() {
		pk, pid := keyFromIdentityString(t, secret)
		seed := sha256.Sum256([]byte(secret))
		pb, err := crypto.MarshalPrivateKey(pk)
		if err != nil {
			t.Fatal(err)
		}
		pubRaw, err := pk.GetPublic().Raw()
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, identityVec{
			Secret:             secret,
			SeedHex:            hex.EncodeToString(seed[:]),
			PrivKeyProtobufB64: base64.StdEncoding.EncodeToString(pb),
			PubKeyRawHex:       hex.EncodeToString(pubRaw),
			PeerID:             pid.String(),
		})
		for _, c := range signingCases() {
			msg, digest, sig := signWithTimestamp(t, pk, c.action, c.timestamp, []byte(c.body))
			sigs = append(sigs, signingVec{
				Secret: secret, PeerID: pid.String(), Action: c.action, Timestamp: c.timestamp, Body: c.body,
				Message: msg, DigestHex: hex.EncodeToString(digest[:]), SignatureB64: base64.StdEncoding.EncodeToString(sig),
			})
		}
	}
	out := os.Getenv("FULA_VECTORS_OUT")
	if out == "" {
		out = t.TempDir()
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, v any) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, name), append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", filepath.Join(out, name))
	}
	write("identity.json", ids)
	write("signing.json", sigs)
}
