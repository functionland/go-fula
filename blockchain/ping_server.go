package blockchain

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"
)

const (
	// PingListenAddr is the TCP address where go-fula listens for
	// kubo-forwarded ping requests.
	PingListenAddr = "0.0.0.0:4021"
)

// PingResponse is the JSON response returned by the ping server.
type PingServerResponse struct {
	Success   bool   `json:"success"`
	Timestamp int64  `json:"timestamp"`
	PeerID    string `json:"peer_id"`
	UptimeMs  int64  `json:"uptime_ms"`
}

// StartPingProxy starts a lightweight TCP HTTP server on PingListenAddr that
// responds to ping requests forwarded by kubo's /x/fula-ping p2p protocol.
// Any HTTP request to the server returns a JSON response with basic node info.
func (bl *FxBlockchain) StartPingProxy(ctx context.Context) error {
	listener, err := net.Listen("tcp", PingListenAddr)
	if err != nil {
		return err
	}

	startTime := time.Now()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := PingServerResponse{
			Success:   true,
			Timestamp: time.Now().UnixMilli(),
			PeerID:    bl.selfPeerID.String(),
			UptimeMs:  time.Since(startTime).Milliseconds(),
		}
		json.NewEncoder(w).Encode(resp)
	})

	bl.pingServer = &http.Server{Handler: mux}
	if bl.wg != nil {
		log.Debug("called wg.Add in blockchain StartPingProxy")
		bl.wg.Add(1)
	}
	go func() {
		if bl.wg != nil {
			log.Debug("called wg.Done in StartPingProxy blockchain")
			defer bl.wg.Done()
		}
		defer log.Debug("StartPingProxy blockchain go routine is ending")
		if err := bl.pingServer.Serve(listener); err != http.ErrServerClosed {
			log.Errorw("Ping server stopped erroneously", "err", err)
		}
	}()
	log.Infow("Ping proxy server started", "addr", PingListenAddr)
	return nil
}

// ShutdownPingProxy gracefully shuts down the ping server.
func (bl *FxBlockchain) ShutdownPingProxy(ctx context.Context) error {
	if bl.pingServer != nil {
		return bl.pingServer.Shutdown(ctx)
	}
	return nil
}
