package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/functionland/go-fula/wap/pkg/config"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"gopkg.in/yaml.v3"
)

var log = logging.Logger("fula/wap/server")
var peerFunction func(clientPeerId string, bloxSeed string) (string, error)

type Config struct {
	Identity                  string   `yaml:"identity"`
	StoreDir                  string   `yaml:"storeDir"`
	PoolName                  string   `yaml:"poolName"`
	LogLevel                  string   `yaml:"logLevel"`
	ListenAddrs               []string `yaml:"listenAddrs"`
	Authorizer                string   `yaml:"authorizer"`
	AuthorizedPeers           []string `yaml:"authorizedPeers"`
	IpfsBootstrapNodes        []string `yaml:"ipfsBootstrapNodes"`
	StaticRelays              []string `yaml:"staticRelays"`
	ForceReachabilityPrivate  bool     `yaml:"forceReachabilityPrivate"`
	AllowTransientConnection  bool     `yaml:"allowTransientConnection"`
	DisableResourceManager    bool     `yaml:"disableResourceManager"`
	MaxCIDPushRate            int      `yaml:"maxCIDPushRate"`
	IpniPublishDisabled       bool     `yaml:"ipniPublishDisabled"`
	IpniPublishInterval       string   `yaml:"ipniPublishInterval"`
	IpniPublishDirectAnnounce []string `yaml:"IpniPublishDirectAnnounce"`
	IpniPublisherIdentity     string   `yaml:"ipniPublisherIdentity"`
}

type multiCloser struct {
	listeners []io.Closer
}

// Implement Close method for multiCloser
func (mc *multiCloser) Close() error {
	var err error
	for _, l := range mc.listeners {
		if cerr := l.Close(); cerr != nil {
			err = cerr
		}
	}
	return err
}

func checkPathExistAndFileNotExist(path string) string {
	dir := filepath.Dir(path)

	// Check if the directory exists
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		// The directory does not exist, so return false
		return "true"
	}
	if err != nil {
		// There was an error other than the directory not existing, so return false
		return "true"
	}

	// If we get here, the directory exists. Now check for the file.
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		// The file does not exist, which is what we want, so return true
		return "false"
	}
	if err != nil {
		// There was an error other than the file not existing, so return false
		return "true"
	}

	// If we get here, the file exists, so return false
	return "true"
}

func propertiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/properties" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method == "GET" {
		hardwareID, err := wifi.GetHardwareID()
		if err != nil {
			hardwareID = ""
		}

		bloxFreeSpace, err := wifi.GetBloxFreeSpace()
		if err != nil {
			bloxFreeSpace = wifi.BloxFreeSpaceResponse{
				DeviceCount:    0,
				Size:           0,
				Used:           0,
				Avail:          0,
				UsedPercentage: 0,
			}
		}
		fulaContainerInfo, err := wifi.GetContainerInfo("fula_go")
		if err != nil {
			fulaContainerInfo = wifi.DockerInfo{
				Image:       "",
				Version:     "",
				ID:          "",
				Labels:      map[string]string{},
				Created:     "",
				RepoDigests: []string{},
			}
		}

		fxsupportContainerInfo, err := wifi.GetContainerInfo("fula_fxsupport")
		if err != nil {
			fulaContainerInfo = wifi.DockerInfo{
				Image:       "",
				Version:     "",
				ID:          "",
				Labels:      map[string]string{},
				Created:     "",
				RepoDigests: []string{},
			}
		}

		nodeContainerInfo, err := wifi.GetContainerInfo("fula_node")
		if err != nil {
			nodeContainerInfo = wifi.DockerInfo{
				Image:       "",
				Version:     "",
				ID:          "",
				Labels:      map[string]string{},
				Created:     "",
				RepoDigests: []string{},
			}
		}

		p, err := config.ReadProperties()
		response := make(map[string]interface{})
		if err == nil {
			response = p
			response["name"] = config.PROJECT_NAME
		}
		response["hardwareID"] = hardwareID
		response["bloxFreeSpace"] = bloxFreeSpace
		response["containerInfo_fula"] = fulaContainerInfo
		response["containerInfo_fxsupport"] = fxsupportContainerInfo
		response["containerInfo_node"] = nodeContainerInfo
		var restartNeeded = checkPathExistAndFileNotExist(config.RESTART_NEEDED_PATH)

		response["restartNeeded"] = restartNeeded
		response["ota_version"] = config.OTA_VERSION

		kuboPeerID, err := wifi.GetKuboPeerID()
		if err == nil {
			response["kubo_peer_id"] = kuboPeerID
		}

		clusterInfo, err := wifi.GetClusterInfo()
		if err == nil {
			response["ipfs_cluster_peer_id"] = clusterInfo.ClusterPeerID
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		jsonErr := json.NewEncoder(w).Encode(response)
		if jsonErr != nil {
			http.Error(w, fmt.Sprintf("error building the response, %v", jsonErr), http.StatusInternalServerError)
			return
		}
		return
	} else if r.Method == "POST" {

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		if jsonErr != nil {
			http.Error(w, fmt.Sprintf("error building the response, %v", jsonErr), http.StatusInternalServerError)
			return
		}
	}

}

func wifiStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/wifi/status" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	connected := true
	ctx, cl := context.WithTimeout(r.Context(), time.Second*10)
	defer cl()
	err := wifi.CheckIfIsConnected(ctx, "")
	if err != nil {
		log.Errorw("failed to check the wifi status", "err", err)
		connected = false
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"status": connected})
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", jsonErr), http.StatusInternalServerError)
		return
	}
}

func partitionHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/partition" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	ctx, cl := context.WithTimeout(r.Context(), time.Second*10)
	defer cl()
	res := wifi.Partition(ctx)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"status": res.Status, "message": res.Msg})
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", jsonErr), http.StatusInternalServerError)
		return
	}
}

func deleteFulaConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/delete-fula-config" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	ctx, cl := context.WithTimeout(r.Context(), time.Second*10)
	defer cl()
	res := wifi.DeleteFulaConfig(ctx)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"status": res.Status, "message": res.Msg})
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", jsonErr), http.StatusInternalServerError)
		return
	}
}

func readinessHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/readiness" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	p, err := config.ReadProperties()
	if err != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
		return
	}
	p["name"] = config.PROJECT_NAME

	kuboPeerID, err := wifi.GetKuboPeerID()
	if err == nil {
		p["kubo_peer_id"] = kuboPeerID
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonErr := json.NewEncoder(w).Encode(p)
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
		return
	}
}

func listWifiHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/wifi/list" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	wifis, err := wifi.Scan(false, "")
	if err != nil {
		log.Errorw("failed to scan the network", "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode(wifis)
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
		return
	}
}

func connectWifiHandler(w http.ResponseWriter, r *http.Request, connectedCh chan bool) {
	if r.URL.Path != "/wifi/connect" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	ssid := r.FormValue("ssid")
	password := r.FormValue("password")

	if ssid == "" {
		http.Error(w, "missing ssid", http.StatusBadRequest)
		return
	}
	if password == "" {
		http.Error(w, "missing password", http.StatusBadRequest)
		return
	}
	credential := wifi.Credentials{
		SSID:        ssid,
		Password:    password,
		CountryCode: config.COUNTRY,
	}
	ctx, cl := context.WithTimeout(r.Context(), time.Second*10)
	defer cl()
	err := wifi.ConnectWifi(ctx, credential)
	if err != nil {
		log.Errorw("failed to connect to wifi", "err", err)
		http.Error(w, "couldn't connect", http.StatusBadRequest)
		return
	}
	log.Info("wifi connected. Calling mdns restart")
	connectedCh <- true

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode("Wifi connected!")
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", jsonErr), http.StatusInternalServerError)
		return
	}
}

func exchangePeersHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/peer/exchange" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	peerID := r.FormValue("peer_id")
	if peerID == "" {
		http.Error(w, "missing peer_id", http.StatusBadRequest)
		return
	}

	seed := r.FormValue("seed")
	if seed == "" {
		http.Error(w, "missing seed", http.StatusBadRequest)
		return
	}

	hardwareID, err := wifi.GetHardwareID()
	if err != nil || hardwareID == "" {
		hardwareID, err = wifi.GenerateRandomString(32)
		if err != nil || hardwareID == "" {
			http.Error(w, "failed to create a random ID or get hardwareID", http.StatusBadRequest)
			return
		}
	}

	seedByte := []byte(seed)
	// Convert byte slice to string
	seedString := string(seedByte)

	combinedSeed := hardwareID + seedString
	bloxPrivKey, err := wifi.GeneratePrivateKeyFromSeed(combinedSeed)
	if err != nil {
		http.Error(w, "failed to create bloxPrivKey", http.StatusBadRequest)
		return
	}

	bloxPeerID, err := peerFunction(peerID, bloxPrivKey)
	if err != nil {
		http.Error(w, "error while exchanging peers", http.StatusBadRequest)

		return
	}

	err = config.WriteProperties(map[string]interface{}{
		"client_peer_id": peerID,
		"blox_peer_id":   bloxPeerID,
		"blox_seed":      bloxPrivKey,
	})
	if err != nil {
		http.Error(w, "failed to write the properties", http.StatusBadRequest)
		return
	}

	// Return the kubo peerID — mobile connects through kubo's libp2p, not the blox identity directly
	kuboPeerID, err := wifi.GetKuboPeerID()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get kubo peer ID: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"peer_id": kuboPeerID})
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
		return
	}
}

func generateIdentityHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/peer/generate-identity" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	seed := r.FormValue("seed")
	if seed == "" {
		http.Error(w, "missing seed", http.StatusBadRequest)
		return
	}

	seedByte := []byte(seed)
	// Convert byte slice to string
	seedString := string(seedByte)

	privKeyString, err := wifi.GeneratePrivateKeyFromSeed(seedString)
	if err != nil {
		http.Error(w, "failed to create privKeyString", http.StatusBadRequest)
		return
	}
	privKeyBytes, err := base64.StdEncoding.DecodeString(privKeyString)
	if err != nil {
		http.Error(w, "failed to StdEncoding.DecodeString", http.StatusBadRequest)
		return
	}

	// Unmarshal the byte slice to get the crypto.PrivKey
	privKey, err := crypto.UnmarshalPrivateKey(privKeyBytes)
	if err != nil {
		http.Error(w, "failed to UnmarshalPrivateKey", http.StatusBadRequest)
		return
	}
	peerID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		http.Error(w, "failed to create peer id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"peer_id": peerID.String(), "seed": privKeyString})
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
		return
	}
}

func enableAccessPointHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ap/enable" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	ctx, cl := context.WithTimeout(r.Context(), time.Second*10)
	defer cl()
	err := wifi.StartHotspot(ctx, true)
	if err != nil {
		log.Errorw("failed to enable the access point", "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"status": "enabled"})
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
		return
	}
}

func disableAccessPointHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ap/disable" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	ctx, cl := context.WithTimeout(r.Context(), time.Second*10)
	defer cl()
	err := wifi.StopHotspot(ctx)
	if err != nil {
		log.Errorw("failed to enable the access point", "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"status": "disable"})
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
		return
	}
}

func getIPFromSpecificNetwork(ctx context.Context, ssid string) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		var cmdString string
		cmdString = "iw dev " + iface.Name + " link"

		out, _, err := wifi.RunCommand(ctx, cmdString)

		// Check network connection based on the command used
		var isConnectedToNetwork bool
		if err == nil {
			isConnectedToNetwork = strings.Contains(out, ssid)
		}

		// If the interface is connected to the specified network, return its IP
		if isConnectedToNetwork {
			addrs, err := iface.Addrs()
			if err != nil {
				return "", err
			}

			for _, addr := range addrs {
				ip, _, err := net.ParseCIDR(addr.String())
				if err != nil {
					continue
				}

				if !ip.IsLoopback() && ip.To4() != nil {
					return ip.String(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no non-loopback IP address found for network %s", ssid)
}

// This finds the ip address of the device
func getNonLoopbackIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}

		if !ip.IsLoopback() && ip.To4() != nil {
			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("no non-loopback IP address found")
}

func joinPoolHandler(w http.ResponseWriter, r *http.Request) {
	var poolID string

	// Check content type and parse accordingly
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// Parse JSON request
		var req struct {
			PoolID string `json:"poolID"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		poolID = req.PoolID
	} else {
		// Read the poolID from form data
		poolID = r.FormValue("poolID")
	}

	// Read the existing config.yaml file
	configFilePath := "/internal/config.yaml"
	configData, err := os.ReadFile(configFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read config file: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse the config.yaml file
	var config Config
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse config file: %v", err), http.StatusInternalServerError)
		return
	}

	// Update the poolName field
	config.PoolName = poolID

	// Marshal the updated config back to YAML
	updatedConfigData, err := yaml.Marshal(&config)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal updated config: %v", err), http.StatusInternalServerError)
		return
	}

	// Write the updated config back to the file
	err = os.WriteFile(configFilePath, updatedConfigData, 0644)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write updated config file: %v", err), http.StatusInternalServerError)
		return
	}

	// Send response
	response := map[string]string{"status": "joined", "poolID": poolID}
	json.NewEncoder(w).Encode(response)
}

func leavePoolHandler(w http.ResponseWriter, r *http.Request) {
	var poolID string

	// Check content type and parse accordingly
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// Parse JSON request
		var req struct {
			PoolID string `json:"poolID"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		poolID = req.PoolID
	} else {
		// Read the poolID from form data
		poolID = r.FormValue("poolID")
	}

	// Leave pool logic
	response := map[string]string{"status": "left", "poolID": poolID}
	json.NewEncoder(w).Encode(response)
}

func cancelJoinPoolHandler(w http.ResponseWriter, r *http.Request) {
	var poolID string

	// Check content type and parse accordingly
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// Parse JSON request
		var req struct {
			PoolID string `json:"poolID"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
			return
		}
		poolID = req.PoolID
	} else {
		// Read the poolID from form data
		poolID = r.FormValue("poolID")
	}

	// Cancel join pool logic
	response := map[string]string{"status": "cancelled", "poolID": poolID}
	json.NewEncoder(w).Encode(response)
}

func chainStatusHandler(w http.ResponseWriter, r *http.Request) {
	// Check chain sync status logic
	status := map[string]interface{}{
		"isSynced":     true,
		"syncProgress": 100,
	}
	json.NewEncoder(w).Encode(status)
}

func accountIdHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the account file exists
	filePath := "/internal/.secrets/account.txt"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Account file not found", http.StatusNotFound)
		return
	}

	// Read the account file
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Failed to read account file", http.StatusInternalServerError)
		return
	}

	// Convert byte slice to string and trim any whitespace
	accountID := strings.TrimSpace(string(data))

	// Create the account map
	account := map[string]interface{}{
		"accountId": accountID,
	}

	// Return the account as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(account)
}

func accountSeedHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the account file exists
	filePath := "/internal/.secrets/secret_seed.txt"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Account Seed file not found", http.StatusNotFound)
		return
	}

	// Read the account seed file
	data, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "Failed to read account seed file", http.StatusInternalServerError)
		return
	}

	// Convert byte slice to string and trim any whitespace
	accountSeed := strings.TrimSpace(string(data))

	// Create the account map
	account := map[string]interface{}{
		"accountSeed": accountSeed,
	}

	// Return the account seed as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(account)
}

// This function accepts an ip and port that it runs the webserver on. Default is 10.42.0.1:3500 and if it fails reverts to 0.0.0.0:3500
// - /wifi/list endpoint: shows the list of available wifis
func Serve(peerFn func(clientPeerId string, bloxSeed string) (string, error), ip string, port string, connectedCh chan bool) io.Closer {
	// Create a context with a reasonable timeout (e.g., 10 minutes)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel() // Ensure resources are cleaned up

	peerFunction = peerFn
	mux := http.NewServeMux()
	mux.HandleFunc("/readiness", readinessHandler)
	mux.HandleFunc("/wifi/list", listWifiHandler)
	mux.HandleFunc("/wifi/status", wifiStatusHandler)
	mux.HandleFunc("/wifi/connect", func(w http.ResponseWriter, r *http.Request) {
		connectWifiHandler(w, r, connectedCh)
	})
	mux.HandleFunc("/ap/enable", enableAccessPointHandler)
	mux.HandleFunc("/ap/disable", disableAccessPointHandler)
	mux.HandleFunc("/properties", propertiesHandler)
	mux.HandleFunc("/partition", partitionHandler)
	mux.HandleFunc("/delete-fula-config", deleteFulaConfigHandler)
	mux.HandleFunc("/peer/exchange", exchangePeersHandler)
	mux.HandleFunc("/peer/generate-identity", generateIdentityHandler)

	mux.HandleFunc("/pools/join", joinPoolHandler)
	mux.HandleFunc("/pools/leave", leavePoolHandler)
	mux.HandleFunc("/pools/cancel", cancelJoinPoolHandler)
	mux.HandleFunc("/chain/status", chainStatusHandler)

	mux.HandleFunc("/account/id", accountIdHandler)
	mux.HandleFunc("/account/seed", accountSeedHandler)

	mc := &multiCloser{
		listeners: []io.Closer{},
	}

	// Track successful addresses
	var successfulAddresses []string

	// Set up the target address
	listenAddr := ""
	if ip == "" {
		ip = config.IPADDRESS
	}
	if port == "" {
		port = config.API_PORT
	}
	listenAddr = ip + ":" + port

	// Try to listen on the target address
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Errorw("Failed to use default IP address for serve", "err", err)

		// If the error is "cannot assign requested address", it means the interface doesn't exist
		// This happens when the FxBlox hotspot is not yet ready
		if strings.Contains(err.Error(), "cannot assign requested address") {
			log.Info("Waiting for FxBlox hotspot interface to become available...")

			// Maximum number of attempts to wait for the hotspot
			maxAttempts := 60 // 60 attempts with 10 second delay = up to 10 minute of waiting
			attemptDelay := 10 * time.Second

			for attempt := 0; attempt < maxAttempts; attempt++ {
				// Check if we've exceeded the context timeout
				if ctx.Err() != nil {
					log.Warnw("Context timeout while waiting for hotspot", "err", ctx.Err())
					break
				}

				// Check if the interface with the target IP exists
				interfaces, err := net.Interfaces()
				if err != nil {
					log.Warnw("Failed to get network interfaces", "err", err)
					time.Sleep(attemptDelay)
					continue
				}

				interfaceFound := false
				for _, iface := range interfaces {
					addrs, err := iface.Addrs()
					if err != nil {
						log.Warnw("Failed to get addresses for interface", "interface", iface.Name, "err", err)
						continue
					}

					for _, addr := range addrs {
						ipNet, ok := addr.(*net.IPNet)
						if !ok {
							continue
						}

						if ipNet.IP.String() == ip {
							log.Infof("Found interface with IP %s: %s (attempt %d)",
								ip, iface.Name, attempt+1)
							interfaceFound = true
							break
						}
					}
					if interfaceFound {
						break
					}
				}

				// If interface is found, try to bind
				if interfaceFound {
					ln, err = net.Listen("tcp", listenAddr)
					if err == nil {
						log.Infof("Successfully bound to %s after waiting", listenAddr)
						break
					} else {
						log.Warnw("Interface found but binding failed", "err", err)
					}
				}

				// If still waiting and not the last attempt
				if attempt < maxAttempts-1 {
					log.Infof("Still waiting for FxBlox hotspot (attempt %d/%d)...",
						attempt+1, maxAttempts)

					// Use a timer that respects context cancellation
					select {
					case <-ctx.Done():
						log.Warnw("Context cancelled while waiting", "err", ctx.Err())
						break
					case <-time.After(attemptDelay):
						// Continue with next attempt
					}
				}
			}
		}

		// If we still can't bind to the target IP after waiting, return without starting the server
		if err != nil {
			log.Errorf("Failed to bind to %s after waiting. Server will not start.", listenAddr)
			return mc // Return empty closer
		}
	}

	// Start the server on the main interface
	if ln != nil {
		mc.listeners = append(mc.listeners, ln)
		successfulAddresses = append(successfulAddresses, listenAddr)
		log.Info("Starting server at " + listenAddr)
		go func() {
			if err := http.Serve(ln, mux); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				log.Errorw("Serve could not initialize", "err", err)
			}
		}()
	}

	// Try second listener (localhost)
	localhostAddr := "127.0.0.1:" + port
	ln1, err1 := net.Listen("tcp", localhostAddr)
	if err1 == nil {
		mc.listeners = append(mc.listeners, ln1)
		successfulAddresses = append(successfulAddresses, localhostAddr)
		go func() {
			if err := http.Serve(ln1, mux); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				log.Errorw("Serve could not initialize on 127.0.0.1", "err", err)
			}
		}()
	} else {
		log.Errorw("Failed to use 127.0.0.1 for serve", "err", err1)
	}

	// Print summary of successful listeners
	if len(successfulAddresses) > 0 {
		log.Infof("Server successfully listening on: %s", strings.Join(successfulAddresses, ", "))
	} else {
		log.Error("Failed to start server on any address")
	}

	return mc
}
