package mdns

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"

	wifi "github.com/functionland/go-fula/wap/pkg/wifi"
	"github.com/hashicorp/mdns"
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
	server *mdns.Server
}
type Config struct {
	Identity   string `yaml:"identity"`
	PoolName   string `yaml:"poolName"`
	Authorizer string `yaml:"authorizer"`
	// include other fields as needed
}

func createInfo() string {

	bloxPeerIdString := "NA"
	poolName := "NA"
	authorizer := "NA"
	hardwareID := "NA"

	hardwareID, err := wifi.GetHardwareID()
	if err != nil {
		log.Errorw("GetHardwareID failed", "err", err)
	}

	data, err := os.ReadFile("/internal/config.yaml")
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

	// Create a map with the required information
	infoMap := map[string]string{
		"bloxPeerIdString": bloxPeerIdString,
		"poolName":         poolName,
		"authorizer":       authorizer,
		"hardwareID":       hardwareID,
	}

	// Convert the map into JSON
	infoJson, err := json.Marshal(infoMap)
	if err != nil {
		log.Errorw("json.Marshal failed", "err", err)
		return ""
	}

	// Return the JSON string
	return string(infoJson)
}

// Add this function
func StartServer(ctx context.Context, port int) *MDNSServer {
	server, err := NewMDNSServer(port)
	if err != nil {
		log.Errorw("NewMDNSServer failed", "err", err)
	}

	// Listen for context done signal to close the server
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	return server
}

func NewMDNSServer(port int) (*MDNSServer, error) {
	host, _ := os.Hostname()
	// Call createInfo() to get the service info
	info := []string{createInfo()}

	service, err := mdns.NewMDNSService(host, ServiceName, "", "", port, []net.IP{net.ParseIP("0.0.0.0")}, info)
	if err != nil {
		return nil, err
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, err
	}

	return &MDNSServer{server: server}, nil
}

func (s *MDNSServer) Close() {
	s.server.Shutdown()
}
