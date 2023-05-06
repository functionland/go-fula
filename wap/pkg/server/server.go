package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/functionland/go-fula/blockchain"
	"github.com/functionland/go-fula/wap/pkg/config"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/server")
var peerFunction func(clientPeerId string, bloxSeed string) (string, error)

func propertiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/properties" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method == "GET" {
		hardwareID, err := wifi.GetHardwareID()
		if err != nil {
			http.Error(w, fmt.Sprintf("error getting hardwareID, %v", err), http.StatusInternalServerError)
			return
		}

		bloxFreeSpace, err := wifi.GetBloxFreeSpace()
		if err != nil {
			http.Error(w, fmt.Sprintf("error getting bloxFreeSpace, %v", err), http.StatusInternalServerError)
			return
		}

		p, err := config.ReadProperties()
		response := make(map[string]interface{})
		if err == nil {
			response = p
			response["name"] = config.PROJECT_NAME
		}
		response["hardwareID"] = hardwareID
		response["bloxFreeSpace"] = bloxFreeSpace

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
	err := wifi.CheckIfIsConnected(ctx)
	if err != nil {
		log.Errorw("failed to check the wifi status", "err", err)
		connected = false
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	jsonErr := json.NewEncoder(w).Encode(map[string]interface{}{"status": connected})
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
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

func connectWifiHandler(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode("Wifi connected!")
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
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

	keySotre := blockchain.NewSimpleKeyStorer()
	if err := keySotre.SaveKey(r.Context(), []byte(seed)); err != nil {
		http.Error(w, "saving the seed", http.StatusBadRequest)
		log.Errorw("saving the seed", "err", err)
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

	return "", fmt.Errorf("No non-loopback IP address found")
}

// This function accepts an ip and port that it runs the webserver on. Default is 192.168.88.1:3500 and if it fails reverts to 0.0.0.0:3500
// - /wifi/list endpoint: shows the list of available wifis
func Serve(peerFn func(clientPeerId string, bloxSeed string) (string, error), ip string, port string) io.Closer {
	peerFunction = peerFn
	mux := http.NewServeMux()
	mux.HandleFunc("/readiness", readinessHandler)
	mux.HandleFunc("/wifi/list", listWifiHandler)
	mux.HandleFunc("/wifi/status", wifiStatusHandler)
	mux.HandleFunc("/wifi/connect", connectWifiHandler)
	mux.HandleFunc("/ap/enable", enableAccessPointHandler)
	mux.HandleFunc("/ap/disable", disableAccessPointHandler)
	mux.HandleFunc("/properties", propertiesHandler)
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
		ip, err = getNonLoopbackIP()
		if err != nil {
			log.Errorw("Failed to get non-loopback IP address", "err", err)
			ip = "0.0.0.0"
		}
		listenAddr = ip + ":" + port

		ln, err = net.Listen("tcp", listenAddr)
		if err != nil {
			listenAddr = "0.0.0.0:" + port
			ln, err = net.Listen("tcp", listenAddr)
			if err != nil {
				log.Errorw("Listen could not initialize", "err", err)
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
