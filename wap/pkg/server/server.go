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
	"sync"
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

// multiCloser owns every listener the server opened. The hotspot watcher can add one long after Serve() has
// returned, while Close() may be running, so access is guarded.
type multiCloser struct {
	mu        sync.Mutex
	listeners []io.Closer
	closed    bool
	// stopWatch is closed by Close() to end the background hotspot watcher. Deliberately not Serve()'s
	// context: that one is `defer cancel()`-ed, so it dies the moment Serve returns, and a watcher hung off
	// it would exit on its first tick without ever retrying. The watcher lives until the server is closed.
	stopWatch chan struct{}
}

// add registers a listener, or closes it immediately and reports false if the server is already shutting down
// (otherwise a listener opened by the watcher during shutdown would leak and keep the port bound).
func (mc *multiCloser) add(l io.Closer) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.closed {
		_ = l.Close()
		return false
	}
	mc.listeners = append(mc.listeners, l)
	return true
}

// Implement Close method for multiCloser
func (mc *multiCloser) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.closed = true
	if mc.stopWatch != nil {
		close(mc.stopWatch)
		mc.stopWatch = nil
	}
	var err error
	for _, l := range mc.listeners {
		if cerr := l.Close(); cerr != nil {
			err = cerr
		}
	}
	mc.listeners = nil
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
		addKuboIdentityState(response, kuboPeerID)

		clusterInfo, err := wifi.GetClusterInfo()
		if err == nil {
			response["ipfs_cluster_peer_id"] = clusterInfo.ClusterPeerID
		}

		// Include authorizer from config.yaml if not already in box_props.json.
		// The PC installer's mDNS advertiser reads this to broadcast pairing state.
		if response["authorizer"] == nil || response["authorizer"] == "" {
			if cfgData, err := os.ReadFile(config.FULA_CONFIG_PATH); err == nil {
				var fulaConfig Config
				if err := yaml.Unmarshal(cfgData, &fulaConfig); err == nil && fulaConfig.Authorizer != "" {
					response["authorizer"] = fulaConfig.Authorizer
				}
			}
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
	addKuboIdentityState(p, kuboPeerID)

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

	// Derive ipfs-cluster peerID (direct from identity, no HMAC)
	clusterPeerID := ""
	privKeyBytes, err := base64.StdEncoding.DecodeString(bloxPrivKey)
	if err == nil {
		// Distinct names rather than a shadowing `err` ladder: this package is gated by the shadow analyzer
		// (see .github/workflows/go-check.yml) because a shadowed err here once left the WAP server refusing
		// to start after a successful bind. These particular shadows were harmless; the gate cannot tell.
		privKey, keyErr := crypto.UnmarshalPrivateKey(privKeyBytes)
		if keyErr == nil {
			pid, pidErr := peer.IDFromPrivateKey(privKey)
			if pidErr == nil {
				clusterPeerID = pid.String()
			}
		}
	}

	err = config.WriteProperties(map[string]interface{}{
		"client_peer_id":       peerID,
		"blox_peer_id":         bloxPeerID,
		"blox_seed":            bloxPrivKey,
		"ipfs_cluster_peer_id": clusterPeerID,
	})
	if err != nil {
		http.Error(w, "failed to write the properties", http.StatusBadRequest)
		return
	}

	// bloxPeerID is the kubo-derived peer ID (from deriveKuboKey in /app --initOnly).
	// Use it directly â€” kubo hasn't started yet during initial setup, so GetKuboPeerID() would fail.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"peer_id": bloxPeerID})
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

	// Bring up the listeners that do NOT depend on the hotspot before waiting for it. The wait loop below
	// blocks this function for up to 10 minutes, and the on-box scripts talk to 127.0.0.1:3500 â€” so starting
	// loopback afterwards meant that after any restart with the AP down, nothing on the box could reach the
	// API for ten minutes, and a box whose AP never returned got no server at all.
	startAuxListeners(mc, &successfulAddresses, mux, port, ip)

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

				// Check if the interface with the target IP exists.
				// NOTE: these must NOT be `err` â€” the outer `err` is what line ~956 checks to decide whether the
				// server starts, and shadowing it here meant a successful late bind below never cleared the
				// original "cannot assign requested address" failure. See the bind block for the full story.
				interfaces, ifaceErr := net.Interfaces()
				if ifaceErr != nil {
					log.Warnw("Failed to get network interfaces", "err", ifaceErr)
					time.Sleep(attemptDelay)
					continue
				}

				interfaceFound := false
				for _, iface := range interfaces {
					addrs, addrErr := iface.Addrs()
					if addrErr != nil {
						log.Warnw("Failed to get addresses for interface", "interface", iface.Name, "err", addrErr)
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

				// If interface is found, try to bind.
				// `err` here is deliberately the OUTER err (assigned, not declared): it is what the check after
				// this loop tests. Before this was fixed, `net.Interfaces()` above shadowed it, so a successful
				// bind left the outer err holding the original failure and the server logged
				// "Successfully bound ..." immediately followed by "... Server will not start." â€” the whole
				// wait-for-hotspot path could only ever fail, and the bound listener was dropped unused.
				// Any fula_go restart while the AP was down therefore left the box with no WAP API until reboot.
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

		// If we still can't bind to the target IP after waiting, carry on WITHOUT the hotspot listener.
		//
		// This used to `return mc`, which also skipped the loopback and LAN listeners below â€” so a box whose
		// AP was down (the normal state once it has joined Wi-Fi, and what a container restart leaves behind,
		// since the FxBlox connection has autoconnect=no) ended up with no WAP API at all, not even on
		// 127.0.0.1. The on-box scripts talk to 127.0.0.1:3500, so they lost it too. Losing the hotspot
		// address is not a reason to serve nothing.
		if err != nil {
			log.Errorf("Failed to bind to %s after waiting; will keep watching for the hotspot in the background.", listenAddr)
			ln = nil
		}
	}

	// Start the server on the main interface
	if ln != nil {
		mc.listeners = append(mc.listeners, ln)
		successfulAddresses = append(successfulAddresses, listenAddr)
		log.Info("Starting server at " + listenAddr)
		go func() {
			if err := http.Serve(ln, withCORS(mux)); err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
				log.Errorw("Serve could not initialize", "err", err)
			}
		}()
	}

	// (loopback and the unowned-box LAN listeners were started before the hotspot wait â€” see startAuxListeners)

	// If the hotspot never showed up, keep watching for it instead of giving up for the life of the process.
	// The 10-minute wait above expiring is not the end of the story: FxBlox has autoconnect=no, so the AP is
	// routinely down (and restarting fula_go tears it down), and it may be enabled minutes or hours later by
	// readiness-check, /ap/enable, or a person. Without this, a box in that state serves only loopback forever
	// â€” someone joins the FxBlox hotspot, gets nothing on 10.42.0.1:3500, and the only cure is another restart,
	// which itself drops the AP again. Observed on hardware.
	//
	// Note it does NOT take `ctx`: this function's context is `defer cancel()`-ed, so it is already dead by
	// the time a watcher's first tick arrives. startHotspotWatch owns a lifetime that ends at Close().
	if ln == nil {
		startHotspotWatch(mc, mux, listenAddr)
	}

	// Print summary of successful listeners
	if len(successfulAddresses) > 0 {
		log.Infof("Server successfully listening on: %s", strings.Join(successfulAddresses, ", "))
	} else {
		log.Error("Failed to start server on any address")
	}

	return mc
}
