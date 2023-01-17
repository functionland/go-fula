package wap

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

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

	wifis, err := Scan(false, "")
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

func createHotspot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/wifi/start" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Unsupported method type.", http.StatusMethodNotAllowed)
		log.Errorw("Method is not supported.", "StatusNotFound", http.StatusMethodNotAllowed, "w", w)
		return
	}

	err := startHotspot(true)
	if err != nil {
		log.Errorw("failed to start the hotspot", "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	jsonErr := json.NewEncoder(w).Encode("Hotspot started")
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
	mux.HandleFunc("/wifi/start", createHotspot)

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
