package mdns

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/grandcat/zeroconf"
)

func TestInstanceNameUniqueness(t *testing.T) {
	// Save and restore globalConfig
	origConfig := globalConfig
	defer func() { globalConfig = origConfig }()

	t.Run("nil globalConfig returns fulatower_NEW", func(t *testing.T) {
		globalConfig = nil
		got := instanceName()
		if got != "fulatower_NEW" {
			t.Errorf("expected fulatower_NEW, got %s", got)
		}
	})

	t.Run("empty BloxPeerIdString returns fulatower_NEW", func(t *testing.T) {
		globalConfig = &Meta{BloxPeerIdString: ""}
		got := instanceName()
		if got != "fulatower_NEW" {
			t.Errorf("expected fulatower_NEW, got %s", got)
		}
	})

	t.Run("NA BloxPeerIdString returns fulatower_NEW", func(t *testing.T) {
		globalConfig = &Meta{BloxPeerIdString: "NA"}
		got := instanceName()
		if got != "fulatower_NEW" {
			t.Errorf("expected fulatower_NEW, got %s", got)
		}
	})

	t.Run("short peerID uses full string", func(t *testing.T) {
		globalConfig = &Meta{BloxPeerIdString: "abc"}
		got := instanceName()
		if got != "fulatower_abc" {
			t.Errorf("expected fulatower_abc, got %s", got)
		}
	})

	t.Run("exactly 5 chars uses full string", func(t *testing.T) {
		globalConfig = &Meta{BloxPeerIdString: "ABCDE"}
		got := instanceName()
		if got != "fulatower_ABCDE" {
			t.Errorf("expected fulatower_ABCDE, got %s", got)
		}
	})

	t.Run("long peerID uses last 5 chars", func(t *testing.T) {
		globalConfig = &Meta{BloxPeerIdString: "12D3KooWAbCdEfGhIjKlMnOpQrStUvWxYz12345"}
		got := instanceName()
		if got != "fulatower_12345" {
			t.Errorf("expected fulatower_12345, got %s", got)
		}
	})

	t.Run("different peerIDs produce different names", func(t *testing.T) {
		ids := []string{
			"12D3KooWAAAAA",
			"12D3KooWBBBBB",
			"12D3KooWCCCCC",
		}
		seen := make(map[string]bool)
		for _, id := range ids {
			globalConfig = &Meta{BloxPeerIdString: id}
			name := instanceName()
			if seen[name] {
				t.Errorf("duplicate instance name %s for peerID %s", name, id)
			}
			seen[name] = true
		}
	})
}

func TestMDNSServiceDiscovery(t *testing.T) {
	type testService struct {
		instanceName string
		port         int
		txt          []string
	}

	services := []testService{
		{
			instanceName: "fulatower_AAAAA",
			port:         18080,
			txt: []string{
				"bloxPeerIdString=12D3KooWAAAAA",
				"ipfsClusterID=clusterA",
				"poolName=poolA",
				"hardwareID=hwA",
			},
		},
		{
			instanceName: "fulatower_BBBBB",
			port:         18081,
			txt: []string{
				"bloxPeerIdString=12D3KooWBBBBB",
				"ipfsClusterID=clusterB",
				"poolName=poolB",
				"hardwareID=hwB",
			},
		},
		{
			instanceName: "fulatower_CCCCC",
			port:         18082,
			txt: []string{
				"bloxPeerIdString=12D3KooWCCCCC",
				"ipfsClusterID=clusterC",
				"poolName=poolC",
				"hardwareID=hwC",
			},
		},
	}

	// Use a test-only service type to avoid interference
	const serviceType = "_fxtest._tcp"

	// Register all 3 services
	var servers []*zeroconf.Server
	for _, svc := range services {
		server, err := zeroconf.Register(
			svc.instanceName,
			serviceType,
			"local.",
			svc.port,
			svc.txt,
			nil, // nil interfaces works on CI (uses all multicast-capable including loopback)
		)
		if err != nil {
			// Clean up already-registered servers
			for _, s := range servers {
				s.Shutdown()
			}
			t.Fatalf("failed to register service %s: %v", svc.instanceName, err)
		}
		servers = append(servers, server)
	}
	defer func() {
		for _, s := range servers {
			s.Shutdown()
		}
	}()

	// Browse for services
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		t.Fatalf("failed to create resolver: %v", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var mu sync.Mutex
	found := make(map[string]*zeroconf.ServiceEntry)

	go func() {
		for entry := range entries {
			mu.Lock()
			found[entry.Instance] = entry
			if len(found) == len(services) {
				cancel() // all found, stop early
			}
			mu.Unlock()
		}
	}()

	err = resolver.Browse(ctx, serviceType, "local.", entries)
	if err != nil {
		t.Fatalf("browse failed: %v", err)
	}

	// Wait for context to finish (either all found or timeout)
	<-ctx.Done()
	// Give a moment for channel processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Assert all 3 services were discovered
	if len(found) != len(services) {
		t.Fatalf("expected %d services, found %d: %v", len(services), len(found), keys(found))
	}

	// Verify each service has correct data
	for _, svc := range services {
		entry, ok := found[svc.instanceName]
		if !ok {
			t.Errorf("service %s not discovered", svc.instanceName)
			continue
		}

		// Check port
		if entry.Port != svc.port {
			t.Errorf("service %s: expected port %d, got %d", svc.instanceName, svc.port, entry.Port)
		}

		// Check TXT records
		txtMap := make(map[string]string)
		for _, record := range entry.Text {
			for _, expected := range svc.txt {
				if record == expected {
					txtMap[expected] = record
				}
			}
		}
		for _, expected := range svc.txt {
			if _, ok := txtMap[expected]; !ok {
				t.Errorf("service %s: missing TXT record %q, got %v", svc.instanceName, expected, entry.Text)
			}
		}
	}

	// Verify all instance names are unique
	names := make(map[string]bool)
	for name := range found {
		if names[name] {
			t.Errorf("duplicate instance name: %s", name)
		}
		names[name] = true
	}
}

func keys(m map[string]*zeroconf.ServiceEntry) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}

func init() {
	// Suppress log output during tests
	_ = fmt.Sprintf("test init")
}
