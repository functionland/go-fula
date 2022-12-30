package wap

import (
	"encoding/json"
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

	wifis, err := Scan(true, "")
	if err != nil {
		log.Errorw("failed to scan the network", "err", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(wifis)
}

// This function accepts an ip and port that it runs the webserver on. Default is 0.0.0.0:3500
// - /wifi/list endpoint: shows the list of available wifis
func Serve(ip string, port string) {

	mux := http.NewServeMux()
	mux.HandleFunc("/wifi/list", listWifiHandler)

	listenAddr := ""

	if ip != "" {
		listenAddr = listenAddr + ip
	} else {
		listenAddr = listenAddr + "0.0.0.0"
	}

	if port != "" {
		listenAddr = listenAddr + ":" + port
	} else {
		listenAddr = listenAddr + ":3500"
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Errorw("Listen could not initialize", "err", err)
	}

	log.Info("Starting server at port 3500\n")
	if err := http.Serve(ln, mux); err != nil {
		log.Errorw("Serve could not initialize", "err", err)
	}
}
