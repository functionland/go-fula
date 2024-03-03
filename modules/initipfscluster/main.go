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

	// Use the flag values (replace hardcoded paths)
	internalPath := *internalPathPtr
	externalPath := *externalPathPtr

	fulaConfigPath := internalPath + "/config.yaml"

	var cfg Config
	// Assuming config.yaml is in the current directory
	configFile, err := os.ReadFile(fulaConfigPath)
	if err != nil {
		panic(fmt.Errorf("reading config.yaml: %v", err))
	}

	if err := yaml.Unmarshal(configFile, &cfg); err != nil {
		panic(fmt.Errorf("parsing config.yaml: %v", err))
	}

	// Decode the base64 encoded identity to get the private key bytes
	privKeyBytes, err := base64.StdEncoding.DecodeString(cfg.Identity)
	if err != nil {
		panic(fmt.Errorf("decoding private key: %v", err))
	}

	// Assuming the private key is in a format understood by the libp2p crypto package
	priv, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		panic(fmt.Errorf("unmarshaling private key: %v", err))
	}

	// Generate the PeerID from the public key
	peerID, err := peer.IDFromPublicKey(priv.GetPublic())
	if err != nil {
		panic(fmt.Errorf("generating PeerID from public key: %v", err))
	}

	identity := map[string]string{
		"id":          peerID.String(),
		"private_key": cfg.Identity,
	}

	identityJSON, err := json.MarshalIndent(identity, "", "    ")
	if err != nil {
		panic(fmt.Errorf("marshaling identity to JSON: %v", err))
	}

	// Write the identity.json to the /internal/ipfs-cluster/ directory
	if err := os.MkdirAll(externalPath+"/ipfs-cluster", 0755); err != nil {
		panic(fmt.Errorf("creating /internal/ipfs-cluster directory: %v", err))
	}
	if err := os.WriteFile(externalPath+"/ipfs-cluster/identity.json", identityJSON, 0644); err != nil {
		panic(fmt.Errorf("writing identity.json: %v", err))
	}

	fmt.Println("IPFS Cluster identity.json created successfully.")
}
