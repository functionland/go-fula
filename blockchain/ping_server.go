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

	// clusterAPIURL is the ipfs-cluster REST API endpoint for identity info.
	clusterAPIURL = "http://127.0.0.1:9096/id"
	// clusterTimeout is how long to wait for ipfs-cluster to respond.
	clusterTimeout = 2 * time.Second
)

// PingServerResponse is the JSON response returned by the ping server.
type PingServerResponse struct {
	Success        bool   `json:"success"`
	Timestamp      int64  `json:"timestamp"`
	PeerID         string `json:"peer_id"`
	UptimeMs       int64  `json:"uptime_ms"`
	ClusterPeerID  string `json:"cluster_peer_id,omitempty"`
	ClusterHealthy bool   `json:"cluster_healthy"`
}

// StartPingProxy starts a lightweight TCP HTTP server on PingListenAddr that
// responds to ping requests forwarded by kubo's /x/fula-ping p2p protocol.
// Any HTTP request to the server returns a JSON response with basic node info,
// including ipfs-cluster health when the cluster is reachable.
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

		// Query ipfs-cluster health (non-blocking, best-effort)
		clusterCtx, clusterCancel := context.WithTimeout(r.Context(), clusterTimeout)
		defer clusterCancel()
		clusterReq, err := http.NewRequestWithContext(clusterCtx, "GET", clusterAPIURL, nil)
		if err == nil {
			clusterResp, err := http.DefaultClient.Do(clusterReq)
			if err == nil {
				defer clusterResp.Body.Close()
				if clusterResp.StatusCode == http.StatusOK {
					var clusterID struct {
						ID string `json:"id"`
					}
					if json.NewDecoder(clusterResp.Body).Decode(&clusterID) == nil && clusterID.ID != "" {
						resp.ClusterPeerID = clusterID.ID
						resp.ClusterHealthy = true
					}
				}
			}
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
