package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/functionland/go-fula/wap/pkg/config"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/server")

func propertiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/properties" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method == "GET" {
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		jsonErr := json.NewEncoder(w).Encode("Couldn't Connect")
		if jsonErr != nil {
			http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
			return
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode("Wifi Connected")
	if jsonErr != nil {
		http.Error(w, fmt.Sprintf("error building the response, %v", err), http.StatusInternalServerError)
		return
	}
}

func enableAccessPoint(w http.ResponseWriter, r *http.Request) {
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
	err := wifi.EnableAccessPoint(ctx)
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

func disableAccessPoint(w http.ResponseWriter, r *http.Request) {
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
	err := wifi.DisableAccessPoint(ctx)
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

// This function accepts an ip and port that it runs the webserver on. Default is 192.168.88.1:3500 and if it fails reverts to 0.0.0.0:3500
// - /wifi/list endpoint: shows the list of available wifis
func Serve(ip string, port string) {

	mux := http.NewServeMux()
	mux.HandleFunc("/wifi/list", listWifiHandler)
	mux.HandleFunc("/wifi/status", wifiStatusHandler)
	mux.HandleFunc("/wifi/connect", connectWifiHandler)
	mux.HandleFunc("/ap/enable", enableAccessPoint)
	mux.HandleFunc("/ap/disable", disableAccessPoint)
	mux.HandleFunc("/properties", propertiesHandler)

	listenAddr := ""

	if ip == "" {
		ip = "192.168.88.1"
	}

	if port == "" {
		port = "3500"
	}

	listenAddr = ip + ":" + port

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		listenAddr = "0.0.0.0:" + port
		ln, err = net.Listen("tcp", listenAddr)
		if err != nil {
			log.Errorw("Listen could not initialize", "err", err)
		}
	}

	log.Info("Starting server at " + listenAddr)
	if err := http.Serve(ln, mux); err != nil {
		log.Errorw("Serve could not initialize", "err", err)
	}
}
