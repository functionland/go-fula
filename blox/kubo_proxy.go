package blox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	defaultKuboAPIAddr = "127.0.0.1:5001"

	// Protocol IDs registered on kubo for stream forwarding
	fulaBlockchainProtocol = "/x/fula-blockchain"
	fulaPingProtocol       = "/x/fula-ping"

	// Ports where go-fula listens for kubo-forwarded streams
	blockchainTargetPort = "4020"
	pingTargetPort       = "4021"

	healthCheckInterval = 30 * time.Second
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
//  1. docker0 interface IPv4 address (Docker bridge gateway)
//  2. Fallback to 127.0.0.1 (works when kubo is also on host network)
func resolveProxyTargetIP() string {
	iface, err := net.InterfaceByName("docker0")
	if err != nil {
		log.Debugw("No docker0 interface found, using 127.0.0.1 for proxy target", "err", err)
		return "127.0.0.1"
	}
	addrs, err := iface.Addrs()
	if err != nil {
		log.Warnw("Failed to get docker0 addresses, using 127.0.0.1", "err", err)
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			ip := ipnet.IP.String()
			log.Infow("Detected Docker bridge IP for proxy target", "ip", ip)
			return ip
		}
	}
	log.Debug("No IPv4 on docker0, using 127.0.0.1 for proxy target")
	return "127.0.0.1"
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

// checkKuboP2PListeners verifies that all required p2p protocols are registered on kubo.
// Returns true if all protocols are present.
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
			Protocol string `json:"Protocol"`
		} `json:"Listeners"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}

	registered := make(map[string]bool)
	for _, l := range result.Listeners {
		registered[l.Protocol] = true
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
// If kubo restarts, re-registers all protocols.
func (p *Blox) watchKuboP2P(ctx context.Context) {
	kuboAPI := p.kuboAPIAddr
	if kuboAPI == "" {
		kuboAPI = defaultKuboAPIAddr
	}

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !checkKuboAlive(kuboAPI) {
				log.Warn("Kubo not responding, will re-register when available")
				continue
			}
			if !checkKuboP2PListeners(kuboAPI) {
				log.Info("Kubo p2p listeners missing, re-registering...")
				if err := registerKuboProtocols(kuboAPI); err != nil {
					log.Errorw("Failed to re-register kubo protocols", "err", err)
				}
			}
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
