package server

import (
	"context"
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
)

var log = logging.Logger("fula/wap/server")
var peerFunction func(clientPeerId string, bloxSeed string) (string, error)

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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		jsonErr := json.NewEncoder(w).Encode(response)
		if jsonErr != nil {
			http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
			return
		}
		return
	} else if r.Method == "POST" {
		ssid := r.FormValue("ssid")
		password := r.FormValue("password")

		err := config.WriteProperties(map[string]interface{}{
			"ssid":     ssid,
			"password": password,
		})
		if err != nil {
			log.Errorw("failed to write the properties", "err", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		if jsonErr != nil {
			http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
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

func connectWifiHandler(w http.ResponseWriter, r *http.Request, mdnsRestartCh chan bool) {
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
	mdnsRestartCh <- true

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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"peer_id": bloxPeerID})
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
		cmdString := "iwconfig " + iface.Name
		out, _, err := wifi.RunCommand(ctx, cmdString)

		// If the interface is connected to the specified network, return its IP.
		if err == nil && strings.Contains(out, ssid) {
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

// This function accepts an ip and port that it runs the webserver on. Default is 10.42.0.1:3500 and if it fails reverts to 0.0.0.0:3500
// - /wifi/list endpoint: shows the list of available wifis
func Serve(peerFn func(clientPeerId string, bloxSeed string) (string, error), ip string, port string, mdnsRestartCh chan bool) io.Closer {
	ctx := context.Background()
	peerFunction = peerFn
	mux := http.NewServeMux()
	mux.HandleFunc("/readiness", readinessHandler)
	mux.HandleFunc("/wifi/list", listWifiHandler)
	mux.HandleFunc("/wifi/status", wifiStatusHandler)
	mux.HandleFunc("/wifi/connect", func(w http.ResponseWriter, r *http.Request) {
		connectWifiHandler(w, r, mdnsRestartCh)
	})
	mux.HandleFunc("/ap/enable", enableAccessPointHandler)
	mux.HandleFunc("/ap/disable", disableAccessPointHandler)
	mux.HandleFunc("/properties", propertiesHandler)
	mux.HandleFunc("/partition", partitionHandler)
	mux.HandleFunc("/delete-fula-config", deleteFulaConfigHandler)
	mux.HandleFunc("/peer/exchange", exchangePeersHandler)

	listenAddr := ""

	if ip == "" {
		ip = config.IPADDRESS
	}

	if port == "" {
		port = config.API_PORT
	}

	listenAddr = ip + ":" + port

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Errorw("Failed to use default IP address for serve", "err", err)
		ip, err = getIPFromSpecificNetwork(ctx, config.HOTSPOT_SSID)
		if err != nil {
			log.Errorw("Failed to use IP of hotspot", "err", err)
			ip, err = getNonLoopbackIP()
			if err != nil {
				log.Errorw("Failed to get non-loopback IP address for serve", "err", err)
				ip = "0.0.0.0"
			}
		}
		listenAddr = ip + ":" + port

		ln, err = net.Listen("tcp", listenAddr)
		if err != nil {
			listenAddr = "0.0.0.0:" + port
			ln, err = net.Listen("tcp", listenAddr)
			if err != nil {
				log.Errorw("Listen could not initialize for serve", "err", err)
			}
		}
	}

	log.Info("Starting server at " + listenAddr)
	go func() {
		if err := http.Serve(ln, mux); err != nil {
			log.Errorw("Serve could not initialize", "err", err)
		}
	}()
	return ln
}
