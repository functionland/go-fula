package fulamobile

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
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

	// Validate peer ID early
	_, err := peer.Decode(*flagBloxPeerID)
	require.NoError(t, err, "Invalid -blox-peer-id")

	// Generate deterministic identity
	identity, err := GenerateEd25519KeyFromString(testIdentitySeed)
	require.NoError(t, err)

	pk, err := crypto.UnmarshalPrivateKey(identity)
	require.NoError(t, err)
	clientPeerID, err := peer.IDFromPrivateKey(pk)
	require.NoError(t, err)

	t.Logf("========================================")
	t.Logf("  CLIENT PEER ID: %s", clientPeerID)
	t.Logf("========================================")
	t.Logf("Whitelist this peer ID on your blox before running.")
	t.Logf("")

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
		cfg.IpfsDHTLookupDisabled = true // Only test direct path
		t.Logf("Mode: DIRECT")
		t.Logf("BloxAddr: %s", cfg.BloxAddr)

	case "relay":
		cfg.BloxAddr = fmt.Sprintf(
			"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835/p2p-circuit/p2p/%s",
			*flagBloxPeerID,
		)
		cfg.IpfsDHTLookupDisabled = true // Only test relay path
		t.Logf("Mode: RELAY (via fx.land)")
		t.Logf("BloxAddr: %s", cfg.BloxAddr)

	case "dht":
		// Bogus IP so direct dial fails; empty relays so relay fails; DHT must find the peer
		cfg.BloxAddr = fmt.Sprintf("/ip4/192.168.99.99/tcp/4001/p2p/%s", *flagBloxPeerID)
		cfg.StaticRelays = []string{}
		cfg.IpfsDHTLookupDisabled = false
		t.Logf("Mode: DHT (bogus IP, no relays — IPFS DHT fallback only)")
		t.Logf("BloxAddr: %s", cfg.BloxAddr)

	default:
		t.Fatalf("Unknown -mode %q. Use: direct, relay, or dht", *flagMode)
	}

	t.Logf("")
	t.Logf("Creating client...")
	client, err := NewClient(cfg)
	require.NoError(t, err, "NewClient failed")
	defer func() {
		t.Logf("Shutting down client...")
		if err := client.Shutdown(); err != nil {
			t.Logf("Shutdown error: %v", err)
		}
	}()

	t.Logf("Client ready, peer ID confirmed: %s", client.ID())

	if *flagMode == "dht" {
		t.Logf("Waiting 20s for IPFS DHT bootstrap to warm up...")
		time.Sleep(20 * time.Second)
	}

	// --- Test 1: PoolList (read-only, safe to call) ---
	t.Run("PoolList", func(t *testing.T) {
		t.Logf("Calling PoolList...")
		result, err := client.PoolList()
		if err != nil {
			t.Errorf("PoolList failed: %v", err)
		} else {
			t.Logf("PoolList response: %s", string(result))
		}
	})

	// --- Test 2: BloxFreeSpace (read-only, safe to call) ---
	t.Run("BloxFreeSpace", func(t *testing.T) {
		t.Logf("Calling BloxFreeSpace...")
		result, err := client.BloxFreeSpace()
		if err != nil {
			t.Errorf("BloxFreeSpace failed: %v", err)
		} else {
			t.Logf("BloxFreeSpace response: %s", string(result))
		}
	})

	// --- Test 3: AccountExists (read-only, safe to call) ---
	t.Run("AccountExists", func(t *testing.T) {
		testAccount := "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"
		t.Logf("Calling AccountExists(%s)...", testAccount)
		result, err := client.AccountExists(testAccount)
		if err != nil {
			t.Errorf("AccountExists failed: %v", err)
		} else {
			t.Logf("AccountExists response: %s", string(result))
		}
	})
}
