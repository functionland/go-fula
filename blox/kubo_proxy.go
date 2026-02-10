package blox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	defaultKuboAPIAddr = "127.0.0.1:5001"

	// Protocol IDs registered on kubo for stream forwarding
	fulaBlockchainProtocol = "/x/fula-blockchain"
	fulaPingProtocol       = "/x/fula-ping"

	// Target addresses where kubo forwards protocol streams
	blockchainTargetAddr = "/ip4/127.0.0.1/tcp/4020"
	pingTargetAddr       = "/ip4/127.0.0.1/tcp/4021"

	healthCheckInterval = 30 * time.Second
)

type p2pProtocol struct {
	name   string
	target string
}

var defaultProtocols = []p2pProtocol{
	{fulaBlockchainProtocol, blockchainTargetAddr},
	{fulaPingProtocol, pingTargetAddr},
}

// registerKuboProtocols registers p2p protocol listeners on kubo via its HTTP API.
// Each protocol maps a libp2p stream protocol to a local TCP address.
func registerKuboProtocols(kuboAPI string) error {
	for _, p := range defaultProtocols {
		if err := registerSingleProtocol(kuboAPI, p.name, p.target); err != nil {
			return fmt.Errorf("failed to register protocol %s: %w", p.name, err)
		}
		log.Infow("Registered kubo p2p protocol", "protocol", p.name, "target", p.target)
	}
	return nil
}

func registerSingleProtocol(kuboAPI, protocol, target string) error {
	// First close any existing listener for this protocol (idempotent)
	closeURL := fmt.Sprintf("http://%s/api/v0/p2p/close?arg=%s&listen-address=%s",
		kuboAPI, protocol, target)
	closeResp, err := http.Post(closeURL, "", nil)
	if err != nil {
		log.Debugw("Could not close existing p2p listener (may not exist)", "protocol", protocol, "err", err)
	} else {
		closeResp.Body.Close()
	}

	// Register the protocol
	url := fmt.Sprintf("http://%s/api/v0/p2p/listen?arg=%s&arg=%s&allow-custom-protocol=true",
		kuboAPI, protocol, target)
	resp, err := http.Post(url, "", nil)
	if err != nil {
		return fmt.Errorf("kubo API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body []byte
		body = make([]byte, 512)
		n, _ := resp.Body.Read(body)
		return fmt.Errorf("kubo API returned status %d: %s", resp.StatusCode, string(body[:n]))
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

	for _, p := range defaultProtocols {
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
