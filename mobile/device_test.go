package fulamobile

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"testing"
	"time"

	gostream "github.com/libp2p/go-libp2p-gostream"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	libp2pping "github.com/libp2p/go-libp2p/p2p/protocol/ping"
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

	// Additional flags for comprehensive method testing
	flagAccount       = flag.String("account", "", "Blockchain account address for account-related tests")
	flagPoolID        = flag.Int("pool-id", 1, "Pool ID for pool-related tests (default: 1)")
	flagChain         = flag.String("chain", "", "Chain name for chain-specific pool operations")
	flagEnableWrite   = flag.Bool("enable-write", false, "Enable write/mutation operations (account create, pool join, etc.)")
	flagDestructive   = flag.Bool("destructive", false, "Enable destructive operations (reboot, partition, erase, etc.)")
	flagWifiName      = flag.String("wifi-name", "", "WiFi connection name for wifi tests")
	flagPluginName    = flag.String("plugin-name", "", "Plugin name for plugin tests")
	flagContainerName = flag.String("container-name", "fula_go", "Container name for log tests")
	flagFolderPath    = flag.String("folder-path", "/uniondrive", "Folder path for GetFolderSize test")
	flagAIModel       = flag.String("ai-model", "", "AI model name for ChatWithAI test")
	flagAIMessage     = flag.String("ai-message", "Hello, how are you?", "User message for ChatWithAI test")
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

	// --- Verify libp2p connectivity (skipped in DHT mode — uses bogus IP) ---
	t.Run("ConnectToBlox", func(t *testing.T) {
		if *flagMode == "dht" {
			t.Skip("Skipped in DHT mode: direct connect uses bogus IP; DHT fallback is tested via BloxFreeSpace")
		}
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

	// --- Raw stream diagnostic (skipped in DHT mode) ---
	t.Run("RawStream", func(t *testing.T) {
		if *flagMode == "dht" {
			t.Skip("Skipped in DHT mode: raw stream uses bogus IP; DHT fallback is tested via BloxFreeSpace")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ctx = network.WithUseTransient(ctx, "test")
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

	// --- Gostream dial diagnostic (skipped in DHT mode) ---
	t.Run("GostreamDial", func(t *testing.T) {
		if *flagMode == "dht" {
			t.Skip("Skipped in DHT mode: gostream dial uses bogus IP; DHT fallback is tested via BloxFreeSpace")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ctx = network.WithUseTransient(ctx, "test")
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

	// --- libp2p ping (works in all modes) ---
	// Runs after BloxFreeSpace so that in DHT mode the connection is already
	// established via the DHT fallback.
	t.Run("Ping", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		ctx = network.WithUseTransient(ctx, "test")

		const pingCount = 3
		var successes int
		for i := 0; i < pingCount; i++ {
			result := <-libp2pping.Ping(ctx, client.h, client.bloxPid)
			if result.Error != nil {
				t.Logf("ping %d failed (non-fatal): %v", i+1, result.Error)
			} else {
				successes++
				t.Logf("ping %d: %s", i+1, result.RTT)
			}
		}
		require.Positive(t, successes, "all %d pings failed", pingCount)
		t.Logf("Ping: %d/%d succeeded", successes, pingCount)
	})

	// ================================================================
	// Client method tests — exercises every exported mobile method
	// against a real device.
	// ================================================================

	// ---- Core Client Methods ----

	t.Run("ClientConnectToBlox", func(t *testing.T) {
		err := client.ConnectToBlox()
		if err != nil {
			t.Errorf("ConnectToBlox failed: %v", err)
		} else {
			t.Logf("ConnectToBlox: OK")
		}
	})

	t.Run("ClientPing", func(t *testing.T) {
		result, err := client.Ping()
		if err != nil {
			t.Errorf("Ping failed: %v", err)
		} else {
			t.Logf("Ping: %s", string(result))
			if !json.Valid(result) {
				t.Errorf("Ping returned invalid JSON")
			}
		}
	})

	t.Run("ClientID", func(t *testing.T) {
		id := client.ID()
		if id == "" {
			t.Errorf("ID returned empty string")
		} else {
			t.Logf("ID: %s", id)
		}
	})

	t.Run("Flush", func(t *testing.T) {
		err := client.Flush()
		if err != nil {
			t.Errorf("Flush failed: %v", err)
		} else {
			t.Logf("Flush: OK")
		}
	})

	// ---- Hardware / System Info (read-only) ----

	t.Run("GetDatastoreSize", func(t *testing.T) {
		result, err := client.GetDatastoreSize()
		if err != nil {
			t.Errorf("GetDatastoreSize failed: %v", err)
		} else {
			t.Logf("GetDatastoreSize: %s", string(result))
		}
	})

	t.Run("GetDockerImageBuildDates", func(t *testing.T) {
		result, err := client.GetDockerImageBuildDates()
		if err != nil {
			t.Errorf("GetDockerImageBuildDates failed: %v", err)
		} else {
			t.Logf("GetDockerImageBuildDates: %s", string(result))
		}
	})

	t.Run("GetClusterInfo", func(t *testing.T) {
		result, err := client.GetClusterInfo()
		if err != nil {
			t.Errorf("GetClusterInfo failed: %v", err)
		} else {
			t.Logf("GetClusterInfo: %s", string(result))
		}
	})

	// ---- Account (read-only) ----

	t.Run("GetAccount", func(t *testing.T) {
		result, err := client.GetAccount()
		if err != nil {
			t.Errorf("GetAccount failed: %v", err)
		} else {
			t.Logf("GetAccount: %s", string(result))
		}
	})

	t.Run("AccountExists", func(t *testing.T) {
		if *flagAccount == "" {
			t.Skip("Skipped: pass -account to run AccountExists test")
		}
		result, err := client.AccountExists(*flagAccount)
		if err != nil {
			t.Errorf("AccountExists failed: %v", err)
		} else {
			t.Logf("AccountExists(%s): %s", *flagAccount, string(result))
		}
	})

	t.Run("AccountBalance", func(t *testing.T) {
		if *flagAccount == "" {
			t.Skip("Skipped: pass -account to run AccountBalance test")
		}
		result, err := client.AccountBalance(*flagAccount)
		if err != nil {
			t.Errorf("AccountBalance failed: %v", err)
		} else {
			t.Logf("AccountBalance(%s): %s", *flagAccount, string(result))
		}
	})

	t.Run("AssetsBalance", func(t *testing.T) {
		if *flagAccount == "" {
			t.Skip("Skipped: pass -account to run AssetsBalance test")
		}
		result, err := client.AssetsBalance(*flagAccount, 0, 0)
		if err != nil {
			t.Errorf("AssetsBalance failed: %v", err)
		} else {
			t.Logf("AssetsBalance(%s, 0, 0): %s", *flagAccount, string(result))
		}
	})

	// ---- Pool (read-only) ----

	t.Run("PoolList", func(t *testing.T) {
		result, err := client.PoolList()
		if err != nil {
			t.Errorf("PoolList failed: %v", err)
		} else {
			t.Logf("PoolList: %s", string(result))
		}
	})

	t.Run("PoolUserList", func(t *testing.T) {
		result, err := client.PoolUserList(*flagPoolID)
		if err != nil {
			t.Errorf("PoolUserList failed: %v", err)
		} else {
			t.Logf("PoolUserList(%d): %s", *flagPoolID, string(result))
		}
	})

	t.Run("PoolRequests", func(t *testing.T) {
		result, err := client.PoolRequests(*flagPoolID)
		if err != nil {
			t.Errorf("PoolRequests failed: %v", err)
		} else {
			t.Logf("PoolRequests(%d): %s", *flagPoolID, string(result))
		}
	})

	t.Run("ManifestAvailable", func(t *testing.T) {
		result, err := client.ManifestAvailable(*flagPoolID)
		if err != nil {
			t.Errorf("ManifestAvailable failed: %v", err)
		} else {
			t.Logf("ManifestAvailable(%d): %s", *flagPoolID, string(result))
		}
	})

	// ---- Container / Logs (read-only) ----

	t.Run("FetchContainerLogs", func(t *testing.T) {
		result, err := client.FetchContainerLogs(*flagContainerName, "50")
		if err != nil {
			t.Errorf("FetchContainerLogs failed: %v", err)
		} else {
			t.Logf("FetchContainerLogs(%s, 50): %s", *flagContainerName, string(result))
		}
	})

	t.Run("FindBestAndTargetInLogs", func(t *testing.T) {
		result, err := client.FindBestAndTargetInLogs(*flagContainerName, "100")
		if err != nil {
			t.Errorf("FindBestAndTargetInLogs failed: %v", err)
		} else {
			t.Logf("FindBestAndTargetInLogs(%s, 100): %s", *flagContainerName, string(result))
		}
	})

	t.Run("GetFolderSize", func(t *testing.T) {
		result, err := client.GetFolderSize(*flagFolderPath)
		if err != nil {
			t.Errorf("GetFolderSize failed: %v", err)
		} else {
			t.Logf("GetFolderSize(%s): %s", *flagFolderPath, string(result))
		}
	})

	// ---- Plugin (read-only) ----

	t.Run("ListPlugins", func(t *testing.T) {
		result, err := client.ListPlugins()
		if err != nil {
			t.Errorf("ListPlugins failed: %v", err)
		} else {
			t.Logf("ListPlugins: %s", string(result))
		}
	})

	t.Run("ListActivePlugins", func(t *testing.T) {
		result, err := client.ListActivePlugins()
		if err != nil {
			t.Errorf("ListActivePlugins failed: %v", err)
		} else {
			t.Logf("ListActivePlugins: %s", string(result))
		}
	})

	t.Run("ShowPluginStatus", func(t *testing.T) {
		if *flagPluginName == "" {
			t.Skip("Skipped: pass -plugin-name to run ShowPluginStatus test")
		}
		result, err := client.ShowPluginStatus(*flagPluginName, 50)
		if err != nil {
			t.Errorf("ShowPluginStatus failed: %v", err)
		} else {
			t.Logf("ShowPluginStatus(%s, 50): %s", *flagPluginName, string(result))
		}
	})

	t.Run("GetInstallStatus", func(t *testing.T) {
		if *flagPluginName == "" {
			t.Skip("Skipped: pass -plugin-name to run GetInstallStatus test")
		}
		result, err := client.GetInstallStatus(*flagPluginName)
		if err != nil {
			t.Errorf("GetInstallStatus failed: %v", err)
		} else {
			t.Logf("GetInstallStatus(%s): %s", *flagPluginName, string(result))
		}
	})

	// ================================================================
	// Write / mutation operations — gated behind -enable-write
	// ================================================================

	// ---- Account Write ----

	t.Run("AccountCreate", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		result, err := client.AccountCreate()
		if err != nil {
			t.Errorf("AccountCreate failed: %v", err)
		} else {
			t.Logf("AccountCreate: %s", string(result))
		}
	})

	t.Run("AccountFund", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		if *flagAccount == "" {
			t.Skip("Skipped: pass -account to run AccountFund test")
		}
		result, err := client.AccountFund(*flagAccount)
		if err != nil {
			t.Errorf("AccountFund failed: %v", err)
		} else {
			t.Logf("AccountFund(%s): %s", *flagAccount, string(result))
		}
	})

	t.Run("TransferToFula", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		if *flagAccount == "" {
			t.Skip("Skipped: pass -account to run TransferToFula test")
		}
		chain := *flagChain
		if chain == "" {
			chain = "mumbai"
		}
		result, err := client.TransferToFula("1000000000000000000", *flagAccount, chain)
		if err != nil {
			t.Errorf("TransferToFula failed: %v", err)
		} else {
			t.Logf("TransferToFula(1000000000000000000, %s, %s): %s", *flagAccount, chain, string(result))
		}
	})

	// ---- Pool Write ----

	t.Run("PoolJoin", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		result, err := client.PoolJoin(*flagPoolID)
		if err != nil {
			t.Errorf("PoolJoin failed: %v", err)
		} else {
			t.Logf("PoolJoin(%d): %s", *flagPoolID, string(result))
		}
	})

	t.Run("PoolJoinWithChain", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		if *flagChain == "" {
			t.Skip("Skipped: pass -chain to run PoolJoinWithChain test")
		}
		result, err := client.PoolJoinWithChain(*flagPoolID, *flagChain)
		if err != nil {
			t.Errorf("PoolJoinWithChain failed: %v", err)
		} else {
			t.Logf("PoolJoinWithChain(%d, %s): %s", *flagPoolID, *flagChain, string(result))
		}
	})

	t.Run("PoolCancelJoin", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		result, err := client.PoolCancelJoin(*flagPoolID)
		if err != nil {
			t.Errorf("PoolCancelJoin failed: %v", err)
		} else {
			t.Logf("PoolCancelJoin(%d): %s", *flagPoolID, string(result))
		}
	})

	t.Run("PoolLeave", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		result, err := client.PoolLeave(*flagPoolID)
		if err != nil {
			t.Errorf("PoolLeave failed: %v", err)
		} else {
			t.Logf("PoolLeave(%d): %s", *flagPoolID, string(result))
		}
	})

	t.Run("PoolLeaveWithChain", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		if *flagChain == "" {
			t.Skip("Skipped: pass -chain to run PoolLeaveWithChain test")
		}
		result, err := client.PoolLeaveWithChain(*flagPoolID, *flagChain)
		if err != nil {
			t.Errorf("PoolLeaveWithChain failed: %v", err)
		} else {
			t.Logf("PoolLeaveWithChain(%d, %s): %s", *flagPoolID, *flagChain, string(result))
		}
	})

	t.Run("BatchUploadManifest", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		// Use a dummy CID for testing — the blox will reject unknown CIDs gracefully
		cids := []byte("bafybeihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenera6dum")
		result, err := client.BatchUploadManifest(cids, *flagPoolID, 1)
		if err != nil {
			t.Errorf("BatchUploadManifest failed: %v", err)
		} else {
			t.Logf("BatchUploadManifest: %s", string(result))
		}
	})

	t.Run("ReplicateInPool", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		if *flagAccount == "" {
			t.Skip("Skipped: pass -account to run ReplicateInPool test")
		}
		cids := []byte("bafybeihdwdcefgh4dqkjv67uzcmw7ojee6xedzdetojuzjevtenera6dum")
		result := client.ReplicateInPool(cids, *flagAccount, *flagPoolID)
		t.Logf("ReplicateInPool: %s", string(result))
	})

	// ---- Plugin Write ----

	t.Run("InstallPlugin", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		if *flagPluginName == "" {
			t.Skip("Skipped: pass -plugin-name to run InstallPlugin test")
		}
		result, err := client.InstallPlugin(*flagPluginName, "")
		if err != nil {
			t.Errorf("InstallPlugin failed: %v", err)
		} else {
			t.Logf("InstallPlugin(%s): %s", *flagPluginName, string(result))
		}
	})

	t.Run("GetInstallOutput", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		if *flagPluginName == "" {
			t.Skip("Skipped: pass -plugin-name to run GetInstallOutput test")
		}
		result, err := client.GetInstallOutput(*flagPluginName, "")
		if err != nil {
			t.Errorf("GetInstallOutput failed: %v", err)
		} else {
			t.Logf("GetInstallOutput(%s): %s", *flagPluginName, string(result))
		}
	})

	t.Run("UpdatePlugin", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		if *flagPluginName == "" {
			t.Skip("Skipped: pass -plugin-name to run UpdatePlugin test")
		}
		result, err := client.UpdatePlugin(*flagPluginName)
		if err != nil {
			t.Errorf("UpdatePlugin failed: %v", err)
		} else {
			t.Logf("UpdatePlugin(%s): %s", *flagPluginName, string(result))
		}
	})

	t.Run("UninstallPlugin", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		if *flagPluginName == "" {
			t.Skip("Skipped: pass -plugin-name to run UninstallPlugin test")
		}
		result, err := client.UninstallPlugin(*flagPluginName)
		if err != nil {
			t.Errorf("UninstallPlugin failed: %v", err)
		} else {
			t.Logf("UninstallPlugin(%s): %s", *flagPluginName, string(result))
		}
	})

	// ---- AI Chat ----

	t.Run("ChatWithAI", func(t *testing.T) {
		if *flagAIModel == "" {
			t.Skip("Skipped: pass -ai-model to run ChatWithAI test")
		}
		streamIDBytes, err := client.ChatWithAI(*flagAIModel, *flagAIMessage)
		if err != nil {
			t.Errorf("ChatWithAI failed: %v", err)
			return
		}
		streamID := string(streamIDBytes)
		t.Logf("ChatWithAI started, streamID: %s", streamID)

		// Read a few chunks via GetChatChunk
		for i := 0; i < 10; i++ {
			chunk, err := client.GetChatChunk(streamID)
			if err != nil {
				t.Logf("GetChatChunk ended at chunk %d: %v", i, err)
				break
			}
			if chunk == "" {
				t.Logf("GetChatChunk chunk %d: (empty — stream closed)", i)
				break
			}
			t.Logf("GetChatChunk chunk %d: %s", i, chunk)
		}
	})

	t.Run("ChatWithAI_StreamIterator", func(t *testing.T) {
		if *flagAIModel == "" {
			t.Skip("Skipped: pass -ai-model to run ChatWithAI_StreamIterator test")
		}
		streamIDBytes, err := client.ChatWithAI(*flagAIModel, *flagAIMessage)
		if err != nil {
			t.Errorf("ChatWithAI failed: %v", err)
			return
		}
		streamID := string(streamIDBytes)
		t.Logf("ChatWithAI started (iterator), streamID: %s", streamID)

		iter, err := client.GetStreamIterator(streamID)
		if err != nil {
			t.Errorf("GetStreamIterator failed: %v", err)
			return
		}
		defer iter.Close()

		count := 0
		for !iter.IsComplete() && count < 10 {
			chunk, err := iter.Next()
			if err == io.EOF {
				t.Logf("StreamIterator: EOF after %d chunks", count)
				break
			}
			if err != nil {
				t.Errorf("StreamIterator.Next failed: %v", err)
				break
			}
			if chunk != "" {
				t.Logf("StreamIterator chunk %d: %s", count, chunk)
				count++
			}
		}
		t.Logf("StreamIterator: HasNext=%v, IsComplete=%v", iter.HasNext(), iter.IsComplete())
	})

	// ---- SetAuth (write) ----

	t.Run("SetAuth", func(t *testing.T) {
		if !*flagEnableWrite {
			t.Skip("Skipped: pass -enable-write to run write operations")
		}
		// Authorize the blox to access the client's data
		err := client.SetAuth(client.ID(), client.bloxPid.String(), true)
		if err != nil {
			t.Errorf("SetAuth failed: %v", err)
		} else {
			t.Logf("SetAuth(%s, %s, true): OK", client.ID(), client.bloxPid.String())
		}
	})

	// ================================================================
	// Destructive operations — gated behind -destructive
	// WARNING: These can erase data, remove WiFi configs, or reboot
	// the device. Only run if you know what you are doing.
	// ================================================================

	t.Run("EraseBlData", func(t *testing.T) {
		if !*flagDestructive {
			t.Skip("Skipped: pass -destructive to run destructive operations (erases blockchain data)")
		}
		result, err := client.EraseBlData()
		if err != nil {
			t.Errorf("EraseBlData failed: %v", err)
		} else {
			t.Logf("EraseBlData: %s", string(result))
		}
	})

	t.Run("DeleteWifi", func(t *testing.T) {
		if !*flagDestructive {
			t.Skip("Skipped: pass -destructive to run destructive operations")
		}
		if *flagWifiName == "" {
			t.Skip("Skipped: pass -wifi-name to run DeleteWifi test")
		}
		result, err := client.DeleteWifi(*flagWifiName)
		if err != nil {
			t.Errorf("DeleteWifi failed: %v", err)
		} else {
			t.Logf("DeleteWifi(%s): %s", *flagWifiName, string(result))
		}
	})

	t.Run("DisconnectWifi", func(t *testing.T) {
		if !*flagDestructive {
			t.Skip("Skipped: pass -destructive to run destructive operations")
		}
		if *flagWifiName == "" {
			t.Skip("Skipped: pass -wifi-name to run DisconnectWifi test")
		}
		result, err := client.DisconnectWifi(*flagWifiName)
		if err != nil {
			t.Errorf("DisconnectWifi failed: %v", err)
		} else {
			t.Logf("DisconnectWifi(%s): %s", *flagWifiName, string(result))
		}
	})

	t.Run("WifiRemoveall", func(t *testing.T) {
		if !*flagDestructive {
			t.Skip("Skipped: pass -destructive to run destructive operations (removes ALL saved WiFi)")
		}
		result, err := client.WifiRemoveall()
		if err != nil {
			t.Errorf("WifiRemoveall failed: %v", err)
		} else {
			t.Logf("WifiRemoveall: %s", string(result))
		}
	})

	t.Run("DeleteFulaConfig", func(t *testing.T) {
		if !*flagDestructive {
			t.Skip("Skipped: pass -destructive to run destructive operations (deletes config.yaml)")
		}
		result, err := client.DeleteFulaConfig()
		if err != nil {
			t.Errorf("DeleteFulaConfig failed: %v", err)
		} else {
			t.Logf("DeleteFulaConfig: %s", string(result))
		}
	})

	t.Run("Partition", func(t *testing.T) {
		if !*flagDestructive {
			t.Skip("Skipped: pass -destructive to run destructive operations (partitions SSD/NVMe)")
		}
		result, err := client.Partition()
		if err != nil {
			t.Errorf("Partition failed: %v", err)
		} else {
			t.Logf("Partition: %s", string(result))
		}
	})

	// Reboot is last — it will make the device unreachable
	t.Run("Reboot", func(t *testing.T) {
		if !*flagDestructive {
			t.Skip("Skipped: pass -destructive to run destructive operations (REBOOTS the device)")
		}
		result, err := client.Reboot()
		if err != nil {
			t.Errorf("Reboot failed: %v", err)
		} else {
			t.Logf("Reboot: %s", string(result))
		}
	})
}
