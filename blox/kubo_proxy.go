package blox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultKuboAPIAddr = "127.0.0.1:5001"

	// Protocol IDs registered on kubo for stream forwarding
	fulaBlockchainProtocol = "/x/fula-blockchain"
	fulaPingProtocol       = "/x/fula-ping"
	fulaClusterProtocol    = "/x/fula-cluster"

	// Ports where go-fula listens for kubo-forwarded streams
	blockchainTargetPort = "4020"
	pingTargetPort       = "4021"
	clusterForwardPort   = "19096"
	clusterSwarmPort     = "9096"

	healthCheckInterval = 30 * time.Second

	poolsAPIEndpoint             = "https://pools.fx.land/pools/"
	serverKuboPeerIDCachePath    = "/internal/.tmp/pool_%s_server_kubo.tmp"

	// Static relay PeerID — matches Peering.Peers / Swarm.RelayClient.StaticRelays
	// in the kubo template config. The watchdog gates fula restarts on actual
	// relay reachability so we don't restart-loop while internet is genuinely down.
	staticRelayPeerID = "12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"

	// Layer-2 fail-safe thresholds (silent when Layer 1's kubo
	// Internal.Libp2pForceReachability=private keeps /p2p-circuit advertised).
	relayCircuitMissingThreshold = 2               // 2 × healthCheckInterval ≈ 60s
	minTimeBetweenFulaRestarts   = 5 * time.Minute // cooldown to prevent restart loops

	// Marker file the host's commands.sh watches; mounted from /home/pi/commands
	// (docker-compose.yml: /home/pi/:/home:rw,rshared on the go-fula service).
	// The host handler runs `docker compose restart` (not just kubo) because
	// ipfs-cluster's init-time peering registrations on kubo are lost on a
	// kubo-only restart and only re-applied when cluster's init re-runs.
	fulaRestartCommandPath = "/home/commands/.command_restart_fula"
)

type p2pProtocol struct {
	name   string
	target string
}

// getProtocols returns the p2p protocols with target addresses using the
// appropriate IP. When kubo runs in Docker bridge networking and go-fula
// runs on the host (or with network_mode: host), 127.0.0.1 inside kubo's
// container refers to kubo itself. We detect the docker0 bridge interface
// IP so kubo can reach the host.
func getProtocols() []p2pProtocol {
	ip := resolveProxyTargetIP()
	return []p2pProtocol{
		{fulaBlockchainProtocol, fmt.Sprintf("/ip4/%s/tcp/%s", ip, blockchainTargetPort)},
		{fulaPingProtocol, fmt.Sprintf("/ip4/%s/tcp/%s", ip, pingTargetPort)},
		// Cluster listener: when the server's ipfs-cluster dials a device's kubo
		// via /x/fula-cluster, the server's kubo needs a listener that forwards
		// the stream to the local ipfs-cluster swarm port (9096).
		// On the device side this listener also exists but the forward registered
		// by registerClusterForward() is what actually tunnels traffic to the server.
		{fulaClusterProtocol, fmt.Sprintf("/ip4/%s/tcp/%s", ip, clusterSwarmPort)},
	}
}

// resolveProxyTargetIP determines the IP address that kubo's container can
// use to reach go-fula's proxy server.
//
// When kubo runs in Docker bridge networking (not host mode), 127.0.0.1
// inside its container is kubo itself. The docker0 bridge interface IP
// is the host's address visible from bridge containers.
//
// Detection order:
//  1. docker0 interface IPv4 address (Docker default bridge gateway)
//  2. Any Docker Compose bridge (br-<hash>) IPv4 address
//  3. Container's own bridge IP (when go-fula runs inside a bridge-networked container)
//  4. Fallback to 127.0.0.1 (works when kubo is also on host network)
func resolveProxyTargetIP() string {
	// 1. Try docker0 (default Docker bridge)
	if ip := getIPv4FromInterface("docker0"); ip != "" {
		log.Infow("Detected Docker bridge IP for proxy target", "iface", "docker0", "ip", ip)
		return ip
	}

	// 2. Try any Docker Compose bridge (br-<hash>)
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if strings.HasPrefix(iface.Name, "br-") {
				if ip := getIPv4FromInterface(iface.Name); ip != "" {
					log.Infow("Detected Docker Compose bridge IP for proxy target", "iface", iface.Name, "ip", ip)
					return ip
				}
			}
		}
	}

	// 3. Container's own bridge IP (Docker bridge networking mode).
	// When go-fula runs inside a container on a bridge network,
	// docker0/br-* don't exist inside the container. Use the container's
	// own IPv4 on its bridge interface (typically eth0), which is reachable
	// from other containers on the same Docker Compose network.
	if ifaces == nil {
		ifaces, _ = net.Interfaces()
	}
	if ifaces != nil {
		for _, iface := range ifaces {
			if iface.Name == "lo" || strings.HasPrefix(iface.Name, "docker") ||
				strings.HasPrefix(iface.Name, "br-") || strings.HasPrefix(iface.Name, "veth") {
				continue
			}
			if ip := getIPv4FromInterface(iface.Name); ip != "" {
				log.Infow("Detected container bridge IP for proxy target (bridge networking mode)",
					"iface", iface.Name, "ip", ip)
				return ip
			}
		}
	}

	// 4. Fallback
	log.Debug("No Docker bridge interface found, using 127.0.0.1 for proxy target")
	return "127.0.0.1"
}

// getIPv4FromInterface returns the first IPv4 address of the named interface,
// or "" if the interface doesn't exist or has no IPv4 address.
func getIPv4FromInterface(name string) string {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return ""
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}

// registerKuboProtocols registers p2p protocol listeners on kubo via its HTTP API.
// Each protocol maps a libp2p stream protocol to a local TCP address.
// It first closes ALL existing p2p listeners to clear any stale state from
// previous go-fula processes, then registers fresh listeners.
func registerKuboProtocols(kuboAPI string) error {
	// Close all existing p2p listeners first to clear stale state.
	// This is safe because go-fula owns all p2p protocol registrations on kubo.
	closeAllURL := fmt.Sprintf("http://%s/api/v0/p2p/close?all=true", kuboAPI)
	closeResp, err := http.Post(closeAllURL, "", nil)
	if err != nil {
		log.Warnw("Could not close existing p2p listeners", "err", err)
	} else {
		body, _ := io.ReadAll(closeResp.Body)
		closeResp.Body.Close()
		log.Debugw("Closed existing p2p listeners", "status", closeResp.StatusCode, "response", string(body))
	}

	protocols := getProtocols()
	for _, p := range protocols {
		if err := registerSingleProtocol(kuboAPI, p.name, p.target); err != nil {
			return fmt.Errorf("failed to register protocol %s: %w", p.name, err)
		}
		log.Infow("Registered kubo p2p protocol", "protocol", p.name, "target", p.target)
	}
	return nil
}

func registerSingleProtocol(kuboAPI, protocol, target string) error {
	listenURL := fmt.Sprintf("http://%s/api/v0/p2p/listen?arg=%s&arg=%s&allow-custom-protocol=true",
		kuboAPI, protocol, target)
	resp, err := http.Post(listenURL, "", nil)
	if err != nil {
		return fmt.Errorf("kubo API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := make([]byte, 512)
		n, _ := resp.Body.Read(body)
		bodyStr := string(body[:n])
		if strings.Contains(bodyStr, "listener already registered") {
			log.Debugw("Protocol already registered, treating as success", "protocol", protocol)
			return nil
		}
		return fmt.Errorf("kubo API returned status %d: %s", resp.StatusCode, bodyStr)
	}
	return nil
}

// checkKuboP2PListeners verifies that all required p2p protocol listeners are registered on kubo.
// Returns true if all protocol listeners are present.
// It distinguishes listeners from forwards: a listener has ListenAddress starting with /p2p/
// (the local peer ID), while a forward has ListenAddress starting with /ip4/ (a local bind address).
func checkKuboP2PListeners(kuboAPI string) bool {
	url := fmt.Sprintf("http://%s/api/v0/p2p/ls", kuboAPI)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result struct {
		Listeners []struct {
			Protocol      string `json:"Protocol"`
			ListenAddress string `json:"ListenAddress"`
		} `json:"Listeners"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	// Only count entries where ListenAddress starts with /p2p/ (true listeners),
	// not /ip4/ (forwards). This avoids counting the cluster forward as a listener.
	registered := make(map[string]bool)
	for _, l := range result.Listeners {
		if strings.HasPrefix(l.ListenAddress, "/p2p/") {
			registered[l.Protocol] = true
		}
	}

	for _, p := range getProtocols() {
		if !registered[p.name] {
			return false
		}
	}
	return true
}

// checkKuboAlive verifies that kubo is responding on its API.
func checkKuboAlive(kuboAPI string) bool {
	url := fmt.Sprintf("http://%s/api/v0/id", kuboAPI)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// waitForKuboAndRegister waits for kubo to become available and registers protocols.
// Returns nil when registration is successful, or error if context is cancelled.
func waitForKuboAndRegister(ctx context.Context, kuboAPI string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if !checkKuboAlive(kuboAPI) {
				log.Debug("Waiting for kubo to become available...")
				continue
			}
			if err := registerKuboProtocols(kuboAPI); err != nil {
				log.Errorw("Failed to register kubo protocols", "err", err)
				continue
			}
			return nil
		}
	}
}

// watchKuboP2P periodically checks that kubo p2p listeners are active.
// If kubo restarts, re-registers all protocols and the cluster forward.
//
// Also runs a Layer-2 fail-safe: if kubo's Identify Addresses lacks a
// /p2p-circuit address for relayCircuitMissingThreshold consecutive cycles
// while the static relay is still reachable, signal the host to restart
// the fula stack (compose restart, not just kubo, so ipfs-cluster's init
// re-runs and re-registers its peering/forward setup against the fresh
// kubo). When Layer 1 (Internal.Libp2pForceReachability=private in the
// kubo config) is in effect this path is silent because /p2p-circuit
// stays continuously advertised.
func (p *Blox) watchKuboP2P(ctx context.Context) {
	kuboAPI := p.kuboAPIAddr
	if kuboAPI == "" {
		kuboAPI = defaultKuboAPIAddr
	}

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	var consecutiveCircuitMissing int
	var lastRestartAt time.Time

	for {
		select {
		case <-ticker.C:
			if !checkKuboAlive(kuboAPI) {
				log.Warn("Kubo not responding, will re-register when available")
				consecutiveCircuitMissing = 0
				continue
			}
			if !checkKuboP2PListeners(kuboAPI) {
				log.Info("Kubo p2p listeners missing, re-registering...")
				if err := registerKuboProtocols(kuboAPI); err != nil {
					log.Errorw("Failed to re-register kubo protocols", "err", err)
				}
				// registerKuboProtocols calls p2p/close?all=true which clears
				// ALL p2p entries (listeners AND forwards). Re-register the
				// cluster forward if configured.
				if serverPeerID := p.getServerKuboPeerID(); serverPeerID != "" {
					if err := registerClusterForward(kuboAPI, serverPeerID); err != nil {
						log.Warnw("Failed to re-register cluster forward", "err", err)
					}
				}
			} else if serverPeerID := p.getServerKuboPeerID(); serverPeerID != "" {
				// Listeners OK — check forward separately
				if !checkClusterForward(kuboAPI) {
					log.Info("Cluster forward missing, re-registering...")
					if err := registerClusterForward(kuboAPI, serverPeerID); err != nil {
						log.Warnw("Failed to register cluster forward", "err", err)
					}
				}
			}

			if checkKuboHasCircuitAddress(kuboAPI) {
				consecutiveCircuitMissing = 0
				continue
			}
			consecutiveCircuitMissing++
			log.Warnw("Kubo Identify is missing /p2p-circuit address",
				"consecutive", consecutiveCircuitMissing,
				"threshold", relayCircuitMissingThreshold)
			if consecutiveCircuitMissing < relayCircuitMissingThreshold {
				continue
			}
			if !checkRelayPeerConnected(kuboAPI, staticRelayPeerID) {
				log.Debug("Relay peer not connected; deferring fula restart until connectivity returns")
				consecutiveCircuitMissing = 0
				continue
			}
			if time.Since(lastRestartAt) < minTimeBetweenFulaRestarts {
				log.Infow("Skipping fula restart — cooldown not elapsed",
					"elapsed", time.Since(lastRestartAt))
				continue
			}
			log.Warn("Kubo has no circuit reservation; signaling host to restart fula stack")
			if err := signalFulaRestart(); err != nil {
				log.Errorw("Failed to signal fula restart", "err", err)
				continue
			}
			lastRestartAt = time.Now()
			consecutiveCircuitMissing = 0
		case <-ctx.Done():
			return
		}
	}
}

// GetKuboAPIAddr returns the kubo API address, with appropriate defaults
func getKuboAPIAddr(addr string) string {
	if addr == "" {
		return defaultKuboAPIAddr
	}
	return strings.TrimPrefix(addr, "http://")
}

// fetchServerKuboPeerID fetches the server's kubo peer ID for the given pool.
// It checks a filesystem cache first, then falls back to the pools API.
func fetchServerKuboPeerID(poolID string) (string, error) {
	cachePath := fmt.Sprintf(serverKuboPeerIDCachePath, poolID)

	// Check filesystem cache
	if data, err := os.ReadFile(cachePath); err == nil {
		peerID := strings.TrimSpace(string(data))
		if peerID != "" {
			log.Debugw("Using cached server kubo peer ID", "poolID", poolID, "peerID", peerID)
			return peerID, nil
		}
	}

	// Fetch from pools API
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(poolsAPIEndpoint + poolID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch pool info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pools API returned status %d", resp.StatusCode)
	}

	var result struct {
		KuboPeerID string `json:"kubo-peerid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode pool info: %w", err)
	}

	if result.KuboPeerID == "" {
		return "", fmt.Errorf("kubo-peerid is empty in pool info for pool %s", poolID)
	}

	// Write to cache
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		log.Warnw("Failed to create cache directory", "err", err)
	} else if err := os.WriteFile(cachePath, []byte(result.KuboPeerID), 0644); err != nil {
		log.Warnw("Failed to write server kubo peer ID cache", "err", err)
	}

	log.Infow("Fetched server kubo peer ID from API", "poolID", poolID, "peerID", result.KuboPeerID)
	return result.KuboPeerID, nil
}

// registerClusterForward registers a p2p/forward on kubo for cluster traffic tunneling.
// The forward maps /x/fula-cluster on the local kubo to the server's kubo peer ID,
// with a local listen address on 127.0.0.1:19096.
func registerClusterForward(kuboAPI, serverKuboPeerID string) error {
	forwardURL := fmt.Sprintf(
		"http://%s/api/v0/p2p/forward?arg=%s&arg=/ip4/127.0.0.1/tcp/%s&arg=/p2p/%s&allow-custom-protocol=true",
		kuboAPI, fulaClusterProtocol, clusterForwardPort, serverKuboPeerID)

	resp, err := http.Post(forwardURL, "", nil)
	if err != nil {
		return fmt.Errorf("kubo API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := make([]byte, 512)
		n, _ := resp.Body.Read(body)
		bodyStr := string(body[:n])
		if strings.Contains(bodyStr, "already forwarding") || strings.Contains(bodyStr, "listener already registered") {
			log.Debugw("Cluster forward already registered, treating as success")
			return nil
		}
		return fmt.Errorf("kubo API returned status %d: %s", resp.StatusCode, bodyStr)
	}

	log.Infow("Registered cluster forward", "protocol", fulaClusterProtocol, "port", clusterForwardPort, "target", serverKuboPeerID)
	return nil
}

// checkClusterForward checks if the /x/fula-cluster forward is registered on kubo.
// It distinguishes a forward from a listener: a forward has TargetAddress starting with /p2p/
// (pointing to a remote peer), while a listener has TargetAddress starting with /ip4/
// (pointing to a local address).
func checkClusterForward(kuboAPI string) bool {
	url := fmt.Sprintf("http://%s/api/v0/p2p/ls", kuboAPI)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var result struct {
		Listeners []struct {
			Protocol      string `json:"Protocol"`
			TargetAddress string `json:"TargetAddress"`
		} `json:"Listeners"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	for _, l := range result.Listeners {
		// A forward has TargetAddress pointing to a remote peer (/p2p/...),
		// while a listener has TargetAddress pointing to a local address (/ip4/...).
		if l.Protocol == fulaClusterProtocol && strings.HasPrefix(l.TargetAddress, "/p2p/") {
			return true
		}
	}
	return false
}

// checkKuboHasCircuitAddress returns true if kubo's /api/v0/id Addresses list
// contains a /p2p-circuit address. That's the only address reachable from peers
// behind NAT (which is everyone, including the mobile app), so its absence
// means the device is unreachable over the internet — the bug this watchdog
// catches if Layer 1 (Internal.Libp2pForceReachability=private) doesn't apply.
func checkKuboHasCircuitAddress(kuboAPI string) bool {
	url := fmt.Sprintf("http://%s/api/v0/id", kuboAPI)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Addresses []string `json:"Addresses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	for _, a := range result.Addresses {
		if strings.Contains(a, "/p2p-circuit/") {
			return true
		}
	}
	return false
}

// checkRelayPeerConnected returns true if the static relay's PeerID appears in
// kubo's swarm peers list. Gates kubo bounces on actual relay reachability: if
// internet is down, bouncing won't acquire a reservation either, so we wait
// until connectivity returns instead of restart-looping.
func checkRelayPeerConnected(kuboAPI, relayPeerID string) bool {
	url := fmt.Sprintf("http://%s/api/v0/swarm/peers", kuboAPI)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var result struct {
		Peers []struct {
			Peer string `json:"Peer"`
		} `json:"Peers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	for _, p := range result.Peers {
		if p.Peer == relayPeerID {
			return true
		}
	}
	return false
}

// signalFulaRestart drops a marker file the host's commands.sh watches via
// inotify. The host handler runs `docker compose restart` for the whole fula
// stack, not just kubo, because ipfs-cluster's init-time setup on kubo
// (peering and p2p forward registrations) is lost on a kubo-only restart.
// Removes any stale marker first so the create event fires reliably even if
// commands.sh hasn't yet processed a previous file.
func signalFulaRestart() error {
	if err := os.MkdirAll(filepath.Dir(fulaRestartCommandPath), 0755); err != nil {
		return fmt.Errorf("mkdir commands dir: %w", err)
	}
	_ = os.Remove(fulaRestartCommandPath)
	f, err := os.Create(fulaRestartCommandPath)
	if err != nil {
		return fmt.Errorf("create command file: %w", err)
	}
	return f.Close()
}

// cleanupStaleRestartMarker removes any leftover marker file from a previous
// process. Runs once at startup so a stale file never triggers an unwanted
// restart on first commands.sh wake-up.
func cleanupStaleRestartMarker() {
	if err := os.Remove(fulaRestartCommandPath); err != nil && !os.IsNotExist(err) {
		log.Warnw("Failed to clean stale fula-restart marker", "err", err)
	}
}
