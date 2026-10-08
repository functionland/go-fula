package abi

import (
	"fmt"
	"strings"
	"testing"
)

func TestEncodeJoinRequestsCall(t *testing.T) {
	peer := "0x" + strings.Repeat("ab", 32)
	got := EncodeJoinRequestsCall(7, peer)
	want := "0xccc9fc03" + fmt.Sprintf("%064x", 7) + strings.Repeat("ab", 32)
	if got != want {
		t.Fatalf("EncodeJoinRequestsCall = %s, want %s", got, want)
	}
}

func TestDecodeJoinRequestStatus(t *testing.T) {
	word := func(v int) string { return fmt.Sprintf("%064x", v) }
	// (account, poolId, timestamp, index, approvals, rejections, status, peerId)
	pending := "0x" + word(0xabc) + word(1) + word(1700000000) + word(0) + word(2) + word(0) + word(1) + strings.Repeat("cd", 32)
	status, err := DecodeJoinRequestStatus(pending)
	if err != nil || status != 1 {
		t.Fatalf("pending request: status=%d err=%v, want 1", status, err)
	}
	none := "0x" + strings.Repeat(word(0), 8)
	status, err = DecodeJoinRequestStatus(none)
	if err != nil || status != 0 {
		t.Fatalf("no request: status=%d err=%v, want 0", status, err)
	}
	if _, err := DecodeJoinRequestStatus("0x" + word(1)); err == nil {
		t.Fatal("short data must be an error")
	}
}

func TestRemoveMemberPeerIdSelector(t *testing.T) {
	// keccak256("removeMemberPeerId(uint32,bytes32)")[:4] — the old placeholder 0x12345678 matched nothing on-chain.
	if MethodSignatures.RemoveMemberPeerId != "0x3d71233a" {
		t.Fatalf("RemoveMemberPeerId selector = %s", MethodSignatures.RemoveMemberPeerId)
	}
}
