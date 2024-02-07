package mdns

import (
	"context"
	"encoding/base64"
	"os"

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
	PoolName         string
	Authorizer       string
	HardwareID       string
}

var globalConfig *Meta // To store the loaded config globally
// Load and parse the config file, then store it globally
func LoadConfig() error {
	if _, err := os.Stat(config.FULA_CONFIG_PATH); os.IsNotExist(err) {
		log.Errorf("Config file does not exist: %s", config.FULA_CONFIG_PATH)
		return err
	}

	data, err := os.ReadFile(config.FULA_CONFIG_PATH)
	if err != nil {
		log.Errorw("ReadFile failed", "err", err)
		return err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Errorw("Unmarshal failed", "err", err)
		return err
	}
	bloxPeerIdString := "NA"
	poolName := "NA"
	authorizer := "NA"
	hardwareID, err := wifi.GetHardwareID()
	if err != nil {
		log.Errorw("GetHardwareID failed", "err", err)
		hardwareID = "NA"
	}

	authorizer = cfg.Authorizer

	km, err := base64.StdEncoding.DecodeString(cfg.Identity)
	if err != nil {
		log.Errorw("DecodeString failed", "err", err)
	} else {
		key, err := crypto.UnmarshalPrivateKey(km)
		if err != nil {
			log.Errorw("UnmarshalPrivateKey failed", "err", err)
		} else {
			bloxPeerId, err := peer.IDFromPrivateKey(key)
			if err != nil {
				log.Errorw("IDFromPrivateKey failed", "err", err)
				bloxPeerIdString = ""
			} else {
				bloxPeerIdString = bloxPeerId.String()
			}
		}
	}
	poolName = cfg.PoolName

	// Create a slice with the required information in key=value format
	infoSlice := Meta{
		BloxPeerIdString: bloxPeerIdString,
		PoolName:         poolName,
		Authorizer:       authorizer,
		HardwareID:       hardwareID,
	}

	log.Infow("mdns info loaded from config file", "infoSlice", infoSlice)

	globalConfig = &infoSlice // Store the config globally
	return nil
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
		"bloxPeerIdString=" + globalConfig.BloxPeerIdString, // Just an example, adjust according to actual data structure
		"poolName=" + globalConfig.PoolName,
		"authorizer=" + globalConfig.Authorizer,
		"hardwareID=" + globalConfig.HardwareID, // Assuming you handle hardwareID differently
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

func NewZeroConfService(port int) (*MDNSServer, error) {
	meta := createInfo()
	log.Debugw("mdns meta created", "meta", meta)

	service, err := zeroconf.Register(
		"fulatower",       // service instance name
		"_fulatower._tcp", // service type and protocol
		"local.",          // service domain
		port,              // service port
		meta,              // service metadata
		nil,               // register on all network interfaces
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
