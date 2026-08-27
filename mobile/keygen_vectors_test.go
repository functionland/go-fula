package fulamobile

import (
	"bytes"
	"crypto/sha256"
	"strconv"
	"strings"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TestGenerateEd25519KeyFromStringParity guards the identity contract relied on by browser clients
// (functionland/fxblox-web packages/fula-web-client): the peer ID derived from the identity string the app passes
// to newClient must equal Ed25519-from-seed(sha256(secret)). blockchain/auth_signed_vectors_test.go emits golden
// vectors with the same inline derivation; this test proves GenerateEd25519KeyFromString matches it.
func TestGenerateEd25519KeyFromStringParity(t *testing.T) {
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
	for _, secret := range []string{join(asc), join(desc), join(lcg)} {
		got, err := GenerateEd25519KeyFromString(secret)
		if err != nil {
			t.Fatalf("GenerateEd25519KeyFromString: %v", err)
		}
		gotPk, err := crypto.UnmarshalPrivateKey(got)
		if err != nil {
			t.Fatalf("UnmarshalPrivateKey: %v", err)
		}
		gotID, err := peer.IDFromPrivateKey(gotPk)
		if err != nil {
			t.Fatal(err)
		}

		seed := sha256.Sum256([]byte(secret))
		wantPk, _, err := crypto.GenerateEd25519Key(bytes.NewReader(seed[:]))
		if err != nil {
			t.Fatal(err)
		}
		wantID, err := peer.IDFromPrivateKey(wantPk)
		if err != nil {
			t.Fatal(err)
		}
		if gotID != wantID {
			t.Fatalf("peer ID mismatch for secret %.24s…: got %s want %s", secret, gotID, wantID)
		}
		if !gotPk.Equals(wantPk) {
			t.Fatalf("private key mismatch for secret %.24s…", secret)
		}
	}
}
