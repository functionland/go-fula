package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Identity string `yaml:"identity"`
}

func main() {
	internalPathPtr := flag.String("internal", "/internal", "Path to the internal disk")
	externalPathPtr := flag.String("external", "/uniondrive", "Path to the internal disk")
	flag.Parse()

	internalPath := *internalPathPtr
	externalPath := *externalPathPtr

	fulaConfigPath := internalPath + "/config.yaml"
	clusterDir := externalPath + "/ipfs-cluster"
	identityPath := clusterDir + "/identity.json"

	var cfg Config
	configFile, err := os.ReadFile(fulaConfigPath)
	if err != nil {
		panic(fmt.Errorf("reading config.yaml: %v", err))
	}

	if err := yaml.Unmarshal(configFile, &cfg); err != nil {
		panic(fmt.Errorf("parsing config.yaml: %v", err))
	}

	// Use the original identity directly for ipfs-cluster (keeps cluster PeerID intact)
	privKeyBytes, err := base64.StdEncoding.DecodeString(cfg.Identity)
	if err != nil {
		panic(fmt.Errorf("decoding private key: %v", err))
	}
	clusterPriv, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		panic(fmt.Errorf("unmarshaling private key: %v", err))
	}
	clusterPeerID, err := peer.IDFromPublicKey(clusterPriv.GetPublic())
	if err != nil {
		panic(fmt.Errorf("generating cluster peer ID: %v", err))
	}
	clusterPrivB64 := cfg.Identity

	// Migration: check if existing identity.json has a different key
	if existingData, err := os.ReadFile(identityPath); err == nil {
		var existing map[string]string
		if err := json.Unmarshal(existingData, &existing); err == nil {
			if existing["private_key"] != clusterPrivB64 {
				fmt.Println("Existing cluster identity differs from derived key. Wiping cluster state for migration...")
				if err := os.RemoveAll(clusterDir); err != nil {
					panic(fmt.Errorf("removing old cluster state: %v", err))
				}
				fmt.Println("Old cluster state removed.")
			}
		}
	}

	// Docker bind-mount footgun guard: a plugin's *file* bind-mount (blox-ai) can
	// create identity.json as a DIRECTORY when the real file is absent at
	// container-create time. os.WriteFile below would then fail ("is a directory")
	// and panic, leaving the cluster wedged. Remove a directory-shaped path so the
	// real (deterministic) file can be written.
	if fi, err := os.Stat(identityPath); err == nil && fi.IsDir() {
		fmt.Println("identity.json is a directory (bind-mount artifact); removing before write")
		if err := os.RemoveAll(identityPath); err != nil {
			panic(fmt.Errorf("removing directory-shaped identity.json: %v", err))
		}
	}

	// Create directory and write identity.json
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		panic(fmt.Errorf("creating ipfs-cluster directory: %v", err))
	}

	identity := map[string]string{
		"id":          clusterPeerID.String(),
		"private_key": clusterPrivB64,
	}

	identityJSON, err := json.MarshalIndent(identity, "", "    ")
	if err != nil {
		panic(fmt.Errorf("marshaling identity to JSON: %v", err))
	}

	if err := os.WriteFile(identityPath, identityJSON, 0644); err != nil {
		panic(fmt.Errorf("writing identity.json: %v", err))
	}

	fmt.Printf("IPFS Cluster identity.json created successfully. Cluster PeerID: %s\n", clusterPeerID.String())
}
