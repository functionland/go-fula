package mdns

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net"
	"os"
	"strings"

	"github.com/functionland/go-fula/wap/pkg/config"
	wifi "github.com/functionland/go-fula/wap/pkg/wifi"
	"github.com/grandcat/zeroconf"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"gopkg.in/yaml.v3"
)

var log = logging.Logger("fula/wap/cmd/mdns")

const (
	ServiceName = "_fxblox._tcp"
)

type MDNSServer struct {
	server *zeroconf.Server
}
type Config struct {
	Identity   string `yaml:"identity"`
	PoolName   string `yaml:"poolName"`
	Authorizer string `yaml:"authorizer"`
	// include other fields as needed
}

type Meta struct {
	BloxPeerIdString string
	IpfsClusterID    string
	PoolName         string
	Authorizer       string
	HardwareID       string
}

var globalConfig *Meta // To store the loaded config globally
// Load and parse the config file, then store it globally
// Modified LoadConfig function to use default values if config file does not exist
func LoadConfig() {
	defaultValue := "NA" // Default value for all fields

	// Initialize with default values
	globalConfig = &Meta{
		BloxPeerIdString: defaultValue,
		IpfsClusterID:    defaultValue,
		PoolName:         defaultValue,
		Authorizer:       defaultValue,
		HardwareID:       defaultValue,
	}

	// Attempt to read hardware ID regardless of config file existence
	hardwareID, err := wifi.GetHardwareID()
	if err != nil {
		log.Errorw("GetHardwareID failed", "err", err)
		globalConfig.HardwareID = defaultValue
	} else {
		globalConfig.HardwareID = hardwareID
	}

	// Check if config file exists
	if _, err := os.Stat(config.FULA_CONFIG_PATH); os.IsNotExist(err) {
		log.Infof("Config file does not exist, using default values: %s", config.FULA_CONFIG_PATH)
		return // Continue with default values
	}

	// Config file exists, attempt to read and parse it
	data, err := os.ReadFile(config.FULA_CONFIG_PATH)
	if err != nil {
		log.Errorw("ReadFile failed", "err", err)
		return // Continue with default values upon read failure
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Errorw("Unmarshal failed", "err", err)
		return // Continue with default values upon unmarshal failure
	}

	// Successfully loaded config, attempt to decode identity
	km, err := base64.StdEncoding.DecodeString(cfg.Identity)
	if err != nil {
		log.Errorw("DecodeString failed", "err", err)
	} else {
		key, err := crypto.UnmarshalPrivateKey(km)
		if err != nil {
			log.Errorw("UnmarshalPrivateKey failed", "err", err)
		} else {
			// ipfs-cluster peer ID (direct from identity)
			clusterPeerId, err := peer.IDFromPrivateKey(key)
			if err != nil {
				log.Errorw("IDFromPrivateKey failed for cluster", "err", err)
			} else {
				globalConfig.IpfsClusterID = clusterPeerId.String()
			}

			// kubo peer ID (derived via HMAC, same as deriveKuboKey in cmd/blox/main.go)
			rawKey, err := key.Raw()
			if err != nil {
				log.Errorw("Raw key extraction failed", "err", err)
			} else {
				mac := hmac.New(sha256.New, []byte("fula-kubo-identity-v1"))
				mac.Write(rawKey[:32])
				derivedSeed := mac.Sum(nil)

				kuboPriv, _, err := crypto.GenerateEd25519Key(bytes.NewReader(derivedSeed))
				if err != nil {
					log.Errorw("GenerateEd25519Key for kubo failed", "err", err)
				} else {
					kuboPeerId, err := peer.IDFromPublicKey(kuboPriv.GetPublic())
					if err != nil {
						log.Errorw("IDFromPublicKey for kubo failed", "err", err)
					} else {
						globalConfig.BloxPeerIdString = kuboPeerId.String()
					}
				}
			}
		}
	}

	// Fallback: if HMAC derivation failed, try reading kubo config directly
	if globalConfig.BloxPeerIdString == defaultValue {
		if kuboPeerID, err := wifi.GetKuboPeerID(); err == nil {
			globalConfig.BloxPeerIdString = kuboPeerID
		}
	}

	// Update the rest of the fields if they were successfully loaded
	if cfg.PoolName != "" {
		globalConfig.PoolName = cfg.PoolName
	}
	if cfg.Authorizer != "" {
		globalConfig.Authorizer = cfg.Authorizer
	}

	log.Infow("mdns info loaded from config file", "infoSlice", globalConfig)
}

// Utilize the global config to create metadata info
func createInfo() []string {
	if globalConfig == nil {
		log.Error("Config not loaded")
		return nil
	}

	// Use the loaded globalConfig here to create your metadata
	// Example:
	infoSlice := []string{
		"bloxPeerIdString=" + globalConfig.BloxPeerIdString,
		"ipfsClusterID=" + globalConfig.IpfsClusterID,
		"poolName=" + globalConfig.PoolName,
		"authorizer=" + globalConfig.Authorizer,
		"hardwareID=" + globalConfig.HardwareID,
	}

	return infoSlice
}

// Add this function
func StartServer(ctx context.Context, port int) *MDNSServer {
	server, err := NewZeroConfService(port)
	if err != nil {
		log.Errorw("NewMDNSServer failed", "err", err)
		return nil
	}
	log.Debug("NewZeroConfService server started")

	// Listen for context done signal to close the server
	go func() {
		<-ctx.Done()
		server.Shutdown()
	}()

	return server
}

// getLANInterfaces returns network interfaces excluding Docker/virtual bridges.
// Returns nil (all interfaces) as fallback if no physical interfaces found.
func getLANInterfaces() []net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var filtered []net.Interface
	for _, iface := range ifaces {
		// Skip down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// Skip Docker bridges and virtual ethernet pairs
		if iface.Name == "docker0" ||
			strings.HasPrefix(iface.Name, "br-") ||
			strings.HasPrefix(iface.Name, "veth") {
			continue
		}
		filtered = append(filtered, iface)
	}

	if len(filtered) == 0 {
		return nil // fallback: let library use all interfaces
	}
	return filtered
}

func instanceName() string {
	if globalConfig == nil {
		return "fulatower_NEW"
	}
	id := globalConfig.BloxPeerIdString
	if id == "" || id == "NA" {
		return "fulatower_NEW"
	}
	suffix := id
	if len(suffix) > 5 {
		suffix = suffix[len(suffix)-5:]
	}
	return "fulatower_" + suffix
}

func NewZeroConfService(port int) (*MDNSServer, error) {
	meta := createInfo()
	log.Debugw("mdns meta created", "meta", meta)

	name := instanceName()
	log.Debugw("mdns instance name", "name", name)

	service, err := zeroconf.Register(
		name,              // unique instance name per device
		"_fulatower._tcp", // service type and protocol
		"local.",          // service domain
		port,              // service port
		meta,              // service metadata
		getLANInterfaces(), // only physical/LAN interfaces
	)

	if err != nil {
		log.Errorw("zeroconf.Register failed", "err", err)
		return nil, err
	}
	log.Debug("NewZeroConfService registered")
	service.TTL(2)

	return &MDNSServer{server: service}, nil
}

func (s *MDNSServer) Shutdown() {
	s.server.Shutdown()
}
