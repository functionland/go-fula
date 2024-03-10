package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
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
	API struct {
		HTTPHeaders map[string]interface{} `json:"HTTPHeaders"`
	} `json:"API"`
	Addresses struct {
		API            string   `json:"API"`
		Announce       []string `json:"Announce"`
		AppendAnnounce []string `json:"AppendAnnounce"`
		Gateway        string   `json:"Gateway"`
		NoAnnounce     []string `json:"NoAnnounce"`
		Swarm          []string `json:"Swarm"`
	} `json:"Addresses"`
	AutoNAT   struct{} `json:"AutoNAT"`
	Bootstrap []string `json:"Bootstrap"`
	DNS       struct {
		Resolvers map[string]interface{} `json:"Resolvers"`
	} `json:"DNS"`
	Datastore struct {
		BloomFilterSize    int           `json:"BloomFilterSize"`
		GCPeriod           string        `json:"GCPeriod"`
		HashOnRead         bool          `json:"HashOnRead"`
		Spec               DatastoreSpec `json:"Spec"`
		StorageGCWatermark int           `json:"StorageGCWatermark"`
		StorageMax         string        `json:"StorageMax"`
	} `json:"Datastore"`
	Discovery struct {
		MDNS struct {
			Enabled bool `json:"Enabled"`
		} `json:"MDNS"`
	} `json:"Discovery"`
	Experimental struct {
		FilestoreEnabled              bool `json:"FilestoreEnabled"`
		Libp2pStreamMounting          bool `json:"Libp2pStreamMounting"`
		OptimisticProvide             bool `json:"OptimisticProvide"`
		OptimisticProvideJobsPoolSize int  `json:"OptimisticProvideJobsPoolSize"`
		P2pHttpProxy                  bool `json:"P2pHttpProxy"`
		StrategicProviding            bool `json:"StrategicProviding"`
		UrlstoreEnabled               bool `json:"UrlstoreEnabled"`
	} `json:"Experimental"`
	Gateway struct {
		DeserializedResponses interface{}            `json:"DeserializedResponses"`
		DisableHTMLErrors     interface{}            `json:"DisableHTMLErrors"`
		ExposeRoutingAPI      interface{}            `json:"ExposeRoutingAPI"`
		HTTPHeaders           map[string]interface{} `json:"HTTPHeaders"`
		NoDNSLink             bool                   `json:"NoDNSLink"`
		NoFetch               bool                   `json:"NoFetch"`
		PublicGateways        interface{}            `json:"PublicGateways"`
		RootRedirect          string                 `json:"RootRedirect"`
	} `json:"Gateway"`
	Identity struct {
		PeerID  string `json:"PeerID"`
		PrivKey string `json:"PrivKey"`
	} `json:"Identity"`
	Internal struct{} `json:"Internal"`
	Ipns     struct {
		RecordLifetime   string `json:"RecordLifetime"`
		RepublishPeriod  string `json:"RepublishPeriod"`
		ResolveCacheSize int    `json:"ResolveCacheSize"`
	} `json:"Ipns"`
	Migration struct {
		DownloadSources []interface{} `json:"DownloadSources"`
		Keep            string        `json:"Keep"`
	} `json:"Migration"`
	Mounts struct {
		FuseAllowOther bool   `json:"FuseAllowOther"`
		IPFS           string `json:"IPFS"`
		IPNS           string `json:"IPNS"`
	} `json:"Mounts"`
	Peering struct {
		Peers interface{} `json:"Peers"`
	} `json:"Peering"`
	Pinning struct {
		RemoteServices map[string]interface{} `json:"RemoteServices"`
	} `json:"Pinning"`
	Plugins struct {
		Plugins interface{} `json:"Plugins"`
	} `json:"Plugins"`
	Provider struct {
		Strategy string `json:"Strategy"`
	} `json:"Provider"`
	Pubsub struct {
		DisableSigning bool   `json:"DisableSigning"`
		Router         string `json:"Router"`
	} `json:"Pubsub"`
	Reprovider struct{} `json:"Reprovider"`
	Routing    struct {
		AcceleratedDHTClient bool        `json:"AcceleratedDHTClient"`
		Methods              interface{} `json:"Methods"`
		Routers              interface{} `json:"Routers"`
	} `json:"Routing"`
	Swarm struct {
		AddrFilters interface{} `json:"AddrFilters"`
		ConnMgr     struct {
			HighWater   int    `json:"HighWater"`
			LowWater    int    `json:"LowWater"`
			GracePeriod string `json:"GracePeriod"`
		} `json:"ConnMgr"`
		DisableBandwidthMetrics bool     `json:"DisableBandwidthMetrics"`
		DisableNatPortMap       bool     `json:"DisableNatPortMap"`
		RelayClient             struct{} `json:"RelayClient"`
		RelayService            struct{} `json:"RelayService"`
		ResourceMgr             struct{} `json:"ResourceMgr"`
		Transports              struct {
			Multiplexers map[string]interface{} `json:"Multiplexers"`
			Network      map[string]interface{} `json:"Network"`
			Security     map[string]interface{} `json:"Security"`
		} `json:"Transports"`
	} `json:"Swarm"`
}

type DatastoreSpec struct {
	Mounts []Mount `json:"mounts"`
	Type   string  `json:"type"`
}
type Mount struct {
	Child      Child  `json:"child"`
	Mountpoint string `json:"mountpoint"`
	Prefix     string `json:"prefix"`
	Type       string `json:"type"`
}

// Define Child struct to match the "child" object inside each mount
type Child struct {
	Path        string `json:"path"`
	ShardFunc   string `json:"shardFunc,omitempty"` // Include omitempty to omit the field if empty
	Sync        bool   `json:"sync,omitempty"`      // Include omitempty for optional fields
	Type        string `json:"type"`
	Compression string `json:"compression,omitempty"` // For levelds type child
}

type ApiResponse struct {
	Users []struct {
		PeerID string `json:"peer_id"`
	} `json:"users"`
}

type DatastoreConfig struct {
	Mounts []MountSingle `json:"mounts"`
	Type   string        `json:"type"`
}
type MountSingle struct {
	Mountpoint string `json:"mountpoint"`
	Path       string `json:"path"`
	ShardFunc  string `json:"shardFunc,omitempty"` // Omitempty will omit this field if it's empty
	Type       string `json:"type"`
}

func main() {
	internalPathPtr := flag.String("internal", "/internal", "Path to the internal disk")
	externalPathPtr := flag.String("external", "/uniondrive", "Path to the external disk")
	defaultIpfsConfigPtr := flag.String("defaultIpfsConfig", "/internal/ipfs_config", "Path to default ipfs config")
	apiIpAddrPtr := flag.String("apiIp", "0.0.0.0", "Defalut address for listening to api. If running outside of docker change it to 127.0.0.1")
	// Parse flags
	flag.Parse()

	// Use the flag values (replace hardcoded paths)
	internalPath := *internalPathPtr
	externalPath := *externalPathPtr
	defaultIpfsConfig := *defaultIpfsConfigPtr
	apiIpAddr := *apiIpAddrPtr

	configPath := internalPath + "/config.yaml"
	ipfsDataPath := internalPath + "/ipfs_data"
	ipfsConfigPath := ipfsDataPath + "/config"
	ipfsDatastorePath := externalPath + "/ipfs_datastore"
	ipfsCfg := IPFSConfig{} // Initialize to empty

	// Check directories and create if necessary
	ensureDirectories(internalPath, ipfsDataPath, ipfsDatastorePath)

	config := readConfigYAML(configPath)
	// Read or create IPFS config
	var err error
	ipfsCfg, err = readIPFSConfig(defaultIpfsConfig, ipfsConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create it in main
			if err := copyDefaultIPFSConfig(defaultIpfsConfig, ipfsConfigPath); err != nil {
				panic(fmt.Sprintf("Failed to create empty config file: %v", err))
			}
		} else {
			// Unexpected error, handle appropriately (e.g., logging, retrying)
			// For now, panic for demonstration:
			panic(fmt.Sprintf("Failed to read or create IPFS config: %v", err))
		}
	}

	updateIPFSConfigIdentity(&ipfsCfg, config)

	/*
		// Fetch users that are in the same pool
		users := []string{}
		if config.PoolName != "0" && config.PoolName != "" {
			users = fetchPoolUsers(config.PoolName)
		}
		updateIPFSConfigBootstrap(&ipfsCfg, config.IpfsBootstrapNodes, users) // We cannot do this as we dont know the ip of these nodes
	*/
	updateDatastorePath(&ipfsCfg, ipfsDatastorePath, apiIpAddr)

	writeIPFSConfig(ipfsConfigPath, ipfsCfg)
	writePredefinedFiles(ipfsDataPath, ipfsDatastorePath)

	fmt.Println("Setup completed.")
}

// Function to create an empty file
func createEmptyFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func copyDefaultIPFSConfig(sourcePath, path string) error {
	// Open the source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err // Handle errors opening the source file
	}
	defer sourceFile.Close()

	// Create the destination file at 'path'
	destFile, err := os.Create(path)
	if err != nil {
		return err // Handle errors creating the destination file
	}
	defer destFile.Close()

	// Copy the contents from source to destination
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err // Handle errors during the copy process
	}

	return nil // Return nil if the copy is successful
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

func readIPFSConfig(sourceIpfsConfig, path string) (IPFSConfig, error) {
	var cfg IPFSConfig

	// Check if the file exists
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return an error to handle in main
			return cfg, err
		} else {
			// Unexpected error, panic
			panic(fmt.Sprintf("Failed to check IPFS config file: %v", err))
		}
	}

	// Check if the target is a directory
	if fileInfo.IsDir() {
		// Target is a directory, attempt to delete it
		err := os.RemoveAll(path)
		if err != nil {
			// Failed to delete directory, return an error
			return cfg, fmt.Errorf("failed to delete directory at %s: %v", path, err)
		}
		if err := copyDefaultIPFSConfig(sourceIpfsConfig, path); err != nil {
			panic(fmt.Sprintf("Failed to create empty config file: %v", err))
		}
	}

	// Read the existing file
	data, err := os.ReadFile(sourceIpfsConfig)
	if err != nil {
		panic(fmt.Sprintf("Failed to read IPFS config: %v", err))
	} else if len(data) == 0 {
		// File is empty, return an empty struct and nil error
		return cfg, nil
	} else {
		// Unmarshal only if the file has content
		if err := json.Unmarshal(data, &cfg); err != nil {
			// Unmarshaling failed, replace file with empty one and return empty cfg
			if err := createEmptyFile(path); err != nil {
				return cfg, err
			}
			return cfg, nil
		}
	}
	return cfg, nil
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

func updateDatastorePath(ipfsCfg *IPFSConfig, newPath string, apiIp string) {
	// Update the path to the new specified path
	for i, mount := range ipfsCfg.Datastore.Spec.Mounts {
		// Update the path for flatfs
		if mount.Child.Type == "flatfs" {
			ipfsCfg.Datastore.Spec.Mounts[i].Child.Path = newPath + "/blocks"
		}
		// Update the path for levelds
		if mount.Child.Type == "levelds" {
			ipfsCfg.Datastore.Spec.Mounts[i].Child.Path = newPath + "/datastore"
		}
	}
	// Update the path to the new specified path
	ipfsCfg.Addresses.Gateway = "/ip4/127.0.0.1/tcp/8081"
	// Update the path to the new specified path
	ipfsCfg.Addresses.API = "/ip4/" + apiIp + "/tcp/5001"
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
	if cfg.Bootstrap == nil {
		cfg.Bootstrap = []string{}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal IPFS config: %v", err))
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		panic(fmt.Sprintf("Failed to write IPFS config: %v", err))
	}
}

func writePredefinedFiles(ipfsDataPath, ipfsDatastorePath string) {
	// Write version file
	if err := os.WriteFile(ipfsDataPath+"/version", []byte("15"), 0644); err != nil {
		fmt.Printf("Failed to write version file: %v\n", err)
		os.Exit(1)
	}

	// Write datastore_spec file
	// Populate the new structure

	dsConfig := DatastoreConfig{
		Mounts: []MountSingle{
			{
				Mountpoint: "/blocks",
				Path:       ipfsDatastorePath + "/blocks",
				ShardFunc:  "/repo/flatfs/shard/v1/next-to-last/2",
				Type:       "flatfs",
			},
			{
				Mountpoint: "/",
				Path:       ipfsDatastorePath + "/datastore",
				Type:       "levelds",
			},
		},
		Type: "mount",
	}

	// Marshal the structure into JSON
	dsConfigJSON, err := json.MarshalIndent(dsConfig, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal datastore config: %v\n", err)
		os.Exit(1)
	}

	// Write the JSON to the datastore_spec file
	if err := os.WriteFile(ipfsDataPath+"/datastore_spec", dsConfigJSON, 0644); err != nil {
		fmt.Printf("Failed to write datastore_spec file: %v\n", err)
		os.Exit(1)
	}
}
