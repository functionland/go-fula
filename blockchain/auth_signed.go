package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	headerPeerID    = "X-Fula-Peer-ID"
	headerTimestamp = "X-Fula-Timestamp"
	headerSignature = "X-Fula-Signature"

	// maxTimestampSkew is the maximum allowed difference between the request
	// timestamp and the server's current time. Prevents replay attacks.
	maxTimestampSkew = 5 * time.Minute
)

// verifySignedRequest verifies the signed headers on an incoming HTTP request.
// It returns the authenticated peer ID on success.
//
// Signing protocol:
//   - Header X-Fula-Peer-ID: peer ID string of the caller
//   - Header X-Fula-Timestamp: Unix timestamp (seconds)
//   - Header X-Fula-Signature: base64-encoded Ed25519 signature
//   - Signed message: sha256(<action> + ":" + <timestamp> + ":" + sha256(<request-body>))
func verifySignedRequest(r *http.Request) (peer.ID, []byte, error) {
	peerIDStr := r.Header.Get(headerPeerID)
	if peerIDStr == "" {
		return "", nil, fmt.Errorf("missing %s header", headerPeerID)
	}

	timestampStr := r.Header.Get(headerTimestamp)
	if timestampStr == "" {
		return "", nil, fmt.Errorf("missing %s header", headerTimestamp)
	}

	signatureStr := r.Header.Get(headerSignature)
	if signatureStr == "" {
		return "", nil, fmt.Errorf("missing %s header", headerSignature)
	}

	// Parse peer ID
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		return "", nil, fmt.Errorf("invalid peer ID: %w", err)
	}

	// Verify timestamp is within acceptable window
	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return "", nil, fmt.Errorf("invalid timestamp: %w", err)
	}
	now := time.Now().Unix()
	diff := now - ts
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(math.Ceil(maxTimestampSkew.Seconds())) {
		return "", nil, fmt.Errorf("timestamp too far from current time: %ds skew", diff)
	}

	// Extract public key from peer ID (Ed25519 peer IDs embed the public key)
	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		return "", nil, fmt.Errorf("cannot extract public key from peer ID: %w", err)
	}

	// Read and buffer the request body
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read request body: %w", err)
		}
		r.Body.Close()
	}

	// Compute body hash
	bodyHash := sha256.Sum256(bodyBytes)

	// Reconstruct the signed message: sha256(action + ":" + timestamp + ":" + sha256(body))
	action := path.Base(r.URL.Path)
	message := action + ":" + timestampStr + ":" + base64.StdEncoding.EncodeToString(bodyHash[:])
	messageHash := sha256.Sum256([]byte(message))

	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(signatureStr)
	if err != nil {
		return "", nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	// Verify signature
	ok, err := pubKey.Verify(messageHash[:], sigBytes)
	if err != nil {
		return "", nil, fmt.Errorf("signature verification error: %w", err)
	}
	if !ok {
		return "", nil, fmt.Errorf("invalid signature")
	}

	return pid, bodyBytes, nil
}

// signRequest signs an outgoing HTTP request with the given private key.
// It adds X-Fula-Peer-ID, X-Fula-Timestamp, and X-Fula-Signature headers.
func signRequest(req *http.Request, privKey crypto.PrivKey, selfPeerID peer.ID) error {
	// Read the body to compute hash
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("failed to read request body for signing: %w", err)
		}
		req.Body.Close()
		// Restore the body so it can be sent
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Compute body hash
	bodyHash := sha256.Sum256(bodyBytes)

	// Build signed message: sha256(action + ":" + timestamp + ":" + sha256(body))
	action := path.Base(req.URL.Path)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	message := action + ":" + timestamp + ":" + base64.StdEncoding.EncodeToString(bodyHash[:])
	messageHash := sha256.Sum256([]byte(message))

	// Sign
	sig, err := privKey.Sign(messageHash[:])
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Set headers
	req.Header.Set(headerPeerID, selfPeerID.String())
	req.Header.Set(headerTimestamp, timestamp)
	req.Header.Set(headerSignature, base64.StdEncoding.EncodeToString(sig))

	return nil
}

// signingTransport is an http.RoundTripper that adds signed headers to every request.
type signingTransport struct {
	base    http.RoundTripper
	privKey crypto.PrivKey
	peerID  peer.ID
}

func (t *signingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := signRequest(req, t.privKey, t.peerID); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}
