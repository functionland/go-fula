package blockchain

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multibase"
)

// peerIdToBytes32 converts a libp2p PeerID to a 32-byte representation for blockchain contracts
// This follows the same logic as the TypeScript implementation
func peerIdToBytes32(peerID string) (string, error) {
	// Normalize to multibase format (starts with z)
	if !strings.HasPrefix(peerID, "z") {
		peerID = "z" + peerID
	}

	// Decode the multibase string
	_, decoded, err := multibase.Decode(peerID)
	if err != nil {
		return "", fmt.Errorf("failed to decode multibase: %w", err)
	}

	var bytes32 []byte

	// CIDv1 (Ed25519 public key) format
	cidHeader := []byte{0x00, 0x24, 0x08, 0x01, 0x12}
	isCIDv1 := len(decoded) >= len(cidHeader)
	if isCIDv1 {
		for i, v := range cidHeader {
			if i >= len(decoded) || decoded[i] != v {
				isCIDv1 = false
				break
			}
		}
	}

	if isCIDv1 && len(decoded) >= 37 {
		// Extract the last 32 bytes as the public key
		bytes32 = decoded[len(decoded)-32:]
	} else if len(decoded) == 34 && decoded[0] == 0x12 && decoded[1] == 0x20 {
		// Legacy multihash format
		bytes32 = decoded[2:] // Skip the multihash header (0x12, 0x20)
	} else {
		return "", fmt.Errorf("unsupported PeerID format or unexpected length: %d", len(decoded))
	}

	if len(bytes32) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(bytes32))
	}

	// Convert to hex string with 0x prefix
	result := "0x" + hex.EncodeToString(bytes32)

	// Reversible check
	reconstructed, err := bytes32ToPeerId(result)
	if err != nil {
		return "", fmt.Errorf("failed reversible check: %w", err)
	}

	// Remove the 'z' prefix for comparison
	originalWithoutZ := peerID
	if strings.HasPrefix(originalWithoutZ, "z") {
		originalWithoutZ = originalWithoutZ[1:]
	}

	if reconstructed != originalWithoutZ {
		return "", fmt.Errorf("could not revert the encoded bytes32 back to original PeerID. Got: %s, Expected: %s", reconstructed, originalWithoutZ)
	}

	return result, nil
}

// bytes32ToPeerId reconstructs the full Base58 PeerID from a bytes32 digest retrieved from the contract
// Always returns a multibase-style PeerID (without the 'z' prefix by default)
func bytes32ToPeerId(digestBytes32 string) (string, error) {
	// Remove 0x prefix if present
	if strings.HasPrefix(digestBytes32, "0x") {
		digestBytes32 = digestBytes32[2:]
	}

	// Decode hex string to bytes
	pubkeyBytes, err := hex.DecodeString(digestBytes32)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex string: %w", err)
	}

	if len(pubkeyBytes) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(pubkeyBytes))
	}

	// Reconstruct the full CIDv1 format
	full := make([]byte, 0, 37)
	full = append(full, 0x00, 0x24)     // CIDv1 prefix
	full = append(full, 0x08, 0x01)     // ed25519-pub key
	full = append(full, 0x12, 0x20)     // multihash: sha2-256, 32 bytes
	full = append(full, pubkeyBytes...) // the actual public key bytes

	// Encode to multibase (base58btc)
	encoded, err := multibase.Encode(multibase.Base58BTC, full)
	if err != nil {
		return "", fmt.Errorf("failed to encode to multibase: %w", err)
	}

	// Return without the multibase 'z' prefix (match legacy PeerID style)
	if strings.HasPrefix(encoded, "z") {
		return encoded[1:], nil
	}
	return encoded, nil
}

// PeerIDToBytes32 is the public wrapper for peerIdToBytes32
func PeerIDToBytes32(peerID peer.ID) (string, error) {
	return peerIdToBytes32(peerID.String())
}

// Bytes32ToPeerID is the public wrapper for bytes32ToPeerId that returns a peer.ID
func Bytes32ToPeerID(digestBytes32 string) (peer.ID, error) {
	peerIDStr, err := bytes32ToPeerId(digestBytes32)
	if err != nil {
		return "", err
	}

	return peer.Decode(peerIDStr)
}

// ValidatePeerIDConversion validates that a PeerID can be converted to bytes32 and back
func ValidatePeerIDConversion(peerID peer.ID) error {
	bytes32, err := PeerIDToBytes32(peerID)
	if err != nil {
		return fmt.Errorf("failed to convert to bytes32: %w", err)
	}

	reconstructed, err := Bytes32ToPeerID(bytes32)
	if err != nil {
		return fmt.Errorf("failed to convert back to PeerID: %w", err)
	}

	if peerID != reconstructed {
		return fmt.Errorf("conversion not reversible: original=%s, reconstructed=%s", peerID, reconstructed)
	}

	return nil
}
