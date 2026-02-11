package fulamobile

import (
	"context"
	"flag"
	"fmt"
	"io"
	"testing"
	"time"

	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/require"
)

// Test flags — pass via:
//
//	go test ./mobile/ -run TestRealDeviceConnection -v -timeout 120s \
//	  -blox-peer-id 12D3KooW... -mode direct -blox-ip 192.168.x.x
//
// Modes:
//
//	direct — connect to blox IP directly (requires -blox-ip)
//	relay  — connect through fx.land static relay
//	dht    — bogus IP + no relays, only IPFS DHT fallback works
var (
	flagBloxPeerID = flag.String("blox-peer-id", "", "Peer ID of the blox/kubo node (required)")
	flagBloxIP     = flag.String("blox-ip", "", "IP address of the blox (required for direct mode)")
	flagMode       = flag.String("mode", "direct", "Connection mode: direct, relay, or dht")
)

// Fixed seed so the client peer ID is deterministic across runs.
const testIdentitySeed = "fula-mobile-device-test-identity-seed"

func TestRealDeviceConnection(t *testing.T) {
	if *flagBloxPeerID == "" {
		t.Skip("Skipping: -blox-peer-id not provided. Run with: go test ./mobile/ -run TestRealDeviceConnection -v -timeout 120s -blox-peer-id <PEER_ID> -mode <direct|relay|dht> [-blox-ip <IP>]")
	}

	_, err := peer.Decode(*flagBloxPeerID)
	require.NoError(t, err, "Invalid -blox-peer-id")

	identity, err := GenerateEd25519KeyFromString(testIdentitySeed)
	require.NoError(t, err)

	pk, err := crypto.UnmarshalPrivateKey(identity)
	require.NoError(t, err)
	clientPeerID, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	t.Logf("========================================")
	t.Logf("  CLIENT PEER ID: %s", clientPeerID)
	t.Logf("========================================")

	cfg := NewConfig()
	cfg.Identity = identity
	cfg.StorePath = t.TempDir()
	cfg.AllowTransientConnection = true

	switch *flagMode {
	case "direct":
		if *flagBloxIP == "" {
			t.Fatal("-blox-ip is required for direct mode")
		}
		cfg.BloxAddr = fmt.Sprintf("/ip4/%s/tcp/4001/p2p/%s", *flagBloxIP, *flagBloxPeerID)
		cfg.IpfsDHTLookupDisabled = true
		t.Logf("Mode: DIRECT — %s", cfg.BloxAddr)

	case "relay":
		cfg.BloxAddr = fmt.Sprintf(
			"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835/p2p-circuit/p2p/%s",
			*flagBloxPeerID,
		)
		cfg.IpfsDHTLookupDisabled = true
		t.Logf("Mode: RELAY — %s", cfg.BloxAddr)

	case "dht":
		cfg.BloxAddr = fmt.Sprintf("/ip4/192.168.99.99/tcp/4001/p2p/%s", *flagBloxPeerID)
		cfg.StaticRelays = []string{}
		cfg.IpfsDHTLookupDisabled = false
		t.Logf("Mode: DHT — %s", cfg.BloxAddr)

	default:
		t.Fatalf("Unknown -mode %q. Use: direct, relay, or dht", *flagMode)
	}

	client, err := NewClient(cfg)
	require.NoError(t, err, "NewClient failed")
	defer func() {
		if err := client.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()
	t.Logf("Client created: %s", client.ID())

	if *flagMode == "dht" {
		t.Logf("Waiting 20s for IPFS DHT bootstrap...")
		time.Sleep(20 * time.Second)
	}

	// --- Verify libp2p connectivity ---
	t.Run("ConnectToBlox", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		peerInfo := client.h.Peerstore().PeerInfo(client.bloxPid)
		t.Logf("Peerstore addresses: %v", peerInfo.Addrs)
		err := client.h.Connect(ctx, peerInfo)
		require.NoError(t, err, "libp2p connect failed")

		conns := client.h.Network().ConnsToPeer(client.bloxPid)
		for i, c := range conns {
			t.Logf("  conn %d: %s -> %s", i, c.LocalMultiaddr(), c.RemoteMultiaddr())
		}

		protos, err := client.h.Peerstore().GetProtocols(client.bloxPid)
		if err == nil {
			t.Logf("  Remote protocols (%d):", len(protos))
			for _, p := range protos {
				t.Logf("    %s", p)
			}
		}
	})

	// --- Raw stream diagnostic ---
	t.Run("RawStream", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		s, err := client.h.NewStream(ctx, client.bloxPid, protocol.ID("/x/fula-blockchain"))
		if err != nil {
			t.Fatalf("NewStream failed: %v", err)
		}
		defer s.Close()
		t.Logf("Stream opened, protocol: %s", s.Protocol())

		_, err = s.Write([]byte("GET /blox-free-space HTTP/1.1\r\nHost: test\r\n\r\n"))
		require.NoError(t, err, "Write failed")

		buf := make([]byte, 4096)
		s.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := s.Read(buf)
		if err != nil && err != io.EOF {
			t.Fatalf("Read failed: %v", err)
		}
		t.Logf("Response (%d bytes): %s", n, string(buf[:n]))
	})

	// --- Gostream dial diagnostic ---
	t.Run("GostreamDial", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		conn, err := gostream.Dial(ctx, client.h, client.bloxPid, "/x/fula-blockchain")
		if err != nil {
			t.Fatalf("gostream.Dial failed: %v", err)
		}
		defer conn.Close()
		t.Logf("Gostream connected")

		_, err = conn.Write([]byte("GET /blox-free-space HTTP/1.1\r\nHost: test\r\n\r\n"))
		require.NoError(t, err, "Write failed")

		buf := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			t.Fatalf("Read failed: %v", err)
		}
		t.Logf("Response (%d bytes): %s", n, string(buf[:n]))
	})

	// --- Full blockchain client call ---
	t.Run("BloxFreeSpace", func(t *testing.T) {
		result, err := client.BloxFreeSpace()
		if err != nil {
			t.Errorf("BloxFreeSpace failed: %v", err)
		} else {
			t.Logf("BloxFreeSpace: %s", string(result))
		}
	})
}
