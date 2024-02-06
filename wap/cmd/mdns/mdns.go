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

func createInfo() []string {

	bloxPeerIdString := "NA"
	poolName := "NA"
	authorizer := "NA"
	hardwareID := "NA"

	hardwareID, err := wifi.GetHardwareID()
	if err != nil {
		log.Errorw("GetHardwareID failed", "err", err)
	}

	data, err := os.ReadFile(config.FULA_CONFIG_PATH)
	if err != nil {
		log.Errorw("ReadFile failed", "err", err)
	}

	// create a new Config
	var config Config

	// unmarshal the YAML data into the config
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Errorw("Unmarshal failed", "err", err)
	}
	authorizer = config.Authorizer

	km, err := base64.StdEncoding.DecodeString(config.Identity)
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

	poolName = config.PoolName

	// Create a slice with the required information in key=value format
	infoSlice := []string{
		"bloxPeerIdString=" + bloxPeerIdString,
		"poolName=" + poolName,
		"authorizer=" + authorizer,
		"hardwareID=" + hardwareID,
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
	log.Info("NewZeroConfService server started")

	// Listen for context done signal to close the server
	go func() {
		<-ctx.Done()
		server.Shutdown()
	}()

	return server
}

func NewZeroConfService(port int) (*MDNSServer, error) {
	meta := createInfo()
	log.Infow("mdns meta created", "meta", meta)

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
	log.Info("NewZeroConfService registered")
	service.TTL(5)

	return &MDNSServer{server: service}, nil
}

func (s *MDNSServer) Shutdown() {
	s.server.Shutdown()
}
