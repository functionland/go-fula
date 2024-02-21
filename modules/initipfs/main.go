package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	"gopkg.in/yaml.v3"
)

type ConfigYAML struct {
	Identity                  string   `yaml:"identity"`
	PoolName                  string   `yaml:"poolName"`
	StaticRelays              []string `yaml:"staticRelays"`
	IpfsBootstrapNodes        []string `yaml:"ipfsBootstrapNodes"`
	IpniPublisherIdentity     string   `yaml:"ipniPublisherIdentity"`
	IpniPublishDirectAnnounce []string `yaml:"IpniPublishDirectAnnounce"`
}

type IPFSConfig struct {
	Identity struct {
		PeerID  string `json:"PeerID"`
		PrivKey string `json:"PrivKey"`
	} `json:"Identity"`
	Bootstrap []string `json:"Bootstrap"`
}

type ApiResponse struct {
	Users []struct {
		PeerID string `json:"peer_id"`
	} `json:"users"`
}

func main() {
	configPath := "/internal/config.yaml"
	ipfsConfigPath := "/internal/ipfs_data/config"

	// Check directories and create if necessary
	ensureDirectories("/internal", "/internal/ipfs_data")

	config := readConfigYAML(configPath)
	ipfsCfg := readIPFSConfig(ipfsConfigPath)

	// Fetch users that are in the same pool
	users := []string{}
	if config.PoolName != "0" && config.PoolName != "" {
		users = fetchPoolUsers(config.PoolName)
	}

	updateIPFSConfigIdentity(&ipfsCfg, config)
	updateIPFSConfigBootstrap(&ipfsCfg, config.IpfsBootstrapNodes, users)

	writeIPFSConfig(ipfsConfigPath, ipfsCfg)
	writePredefinedFiles()

	fmt.Println("Setup completed.")
}

func ensureDirectories(paths ...string) {
	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0755); err != nil {
				panic(fmt.Sprintf("Failed to create directory %s: %v", path, err))
			}
		}
	}
}

func readConfigYAML(path string) ConfigYAML {
	var cfg ConfigYAML
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("Failed to read YAML config: %v", err))
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		panic(fmt.Sprintf("Failed to unmarshal YAML config: %v", err))
	}
	return cfg
}

func readIPFSConfig(path string) IPFSConfig {
	var cfg IPFSConfig
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("Failed to read IPFS config: %v", err))
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		panic(fmt.Sprintf("Failed to unmarshal IPFS config: %v", err))
	}
	return cfg
}

func fetchPoolUsers(poolName string) []string {
	url := "https://api.node3.functionyard.fula.network/fula/pool/users"
	payload := map[string]interface{}{
		"pool_id": poolName,
	}
	payloadBytes, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		panic(fmt.Sprintf("Failed to fetch pool users: %v", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("Unexpected status code: %d", resp.StatusCode))
	}

	body, _ := io.ReadAll(resp.Body)
	var apiResponse ApiResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		panic(fmt.Sprintf("Failed to parse ApiResponse: %v", err))
	}

	userIDs := []string{}
	for _, user := range apiResponse.Users {
		userIDs = append(userIDs, user.PeerID)
	}
	return userIDs
}

func updateIPFSConfigIdentity(ipfsCfg *IPFSConfig, cfg ConfigYAML) {
	ipfsCfg.Identity.PrivKey = cfg.Identity
	ipfsCfg.Identity.PeerID = generatePeerIDFromIdentity(cfg.Identity)
}

func generatePeerIDFromIdentity(identity string) string {
	// Decode the base64 encoded identity to get the private key bytes
	privateKeyBytes, err := base64.StdEncoding.DecodeString(identity)
	if err != nil {
		panic(err)
	}

	// Assuming the private key is in a format understood by the libp2p crypto package
	priv, err := crypto.UnmarshalPrivateKey(privateKeyBytes)
	if err != nil {
		panic(err)
	}

	// Generate the PeerID from the public key
	peerID, err := peer.IDFromPublicKey(priv.GetPublic())
	if err != nil {
		panic(err)
	}
	return peerID.String()
}

func updateIPFSConfigBootstrap(ipfsCfg *IPFSConfig, predefinedBootstraps, bootstrapPeers []string) {
	ipfsCfg.Bootstrap = append(predefinedBootstraps, bootstrapPeers...)
}

func writeIPFSConfig(path string, cfg IPFSConfig) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal IPFS config: %v", err))
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		panic(fmt.Sprintf("Failed to write IPFS config: %v", err))
	}
}

func writePredefinedFiles() {
	// Write version file
	if err := os.WriteFile("/internal/ipfs_data/version", []byte("15"), 0644); err != nil {
		fmt.Printf("Failed to write version file: %v\n", err)
		os.Exit(1)
	}

	// Write datastore_spec file
	dsSpec := `{"path":"badgerds","type":"badgerds"}`
	if err := os.WriteFile("/internal/ipfs_data/datastore_spec", []byte(dsSpec), 0644); err != nil {
		fmt.Printf("Failed to write datastore_spec file: %v\n", err)
		os.Exit(1)
	}
}
