package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	wifi "github.com/functionland/go-fula/wap/pkg/wifi"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// nasCredentialsMaxBytes caps how much of the credentials file is read.
const nasCredentialsMaxBytes = 4096

var (
	// nasCredentialsPath is where fula-ota's nas-credentials.sh stores the
	// per-device Samba password (host /home/pi/.internal/nas, mounted into
	// fula_go as /internal/nas). Deliberately not configurable from the
	// environment; tests replace the variable directly.
	nasCredentialsPath = func() string { return "/internal/nas/credentials.json" }
	// kuboPeerIDFunc returns this blox's kubo peer ID; a variable so tests can stub it.
	kuboPeerIDFunc = wifi.GetKuboPeerID
)

// nasCredentialsFile mirrors the fields of nas-credentials.sh's JSON file that
// are returned to the owner.
type nasCredentialsFile struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Share     string `json:"share"`
	CreatedAt string `json:"created_at"`
}

func writeNasCredentialsStatus(w http.ResponseWriter, code int, status string) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(NasCredentialsResponse{Status: status})
}

// handleNasCredentials returns the device's Samba NAS credentials. It is only
// reachable by the blox owner (see authorized()) and never logs the password.
func (bl *FxBlockchain) handleNasCredentials(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionNasCredentials, "from", from)
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	var req NasCredentialsRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, nasCredentialsMaxBytes)).Decode(&req); err != nil || req.BloxPeerID == "" {
		writeNasCredentialsStatus(w, http.StatusBadRequest, "bad_request")
		return
	}

	self, err := kuboPeerIDFunc()
	if err != nil || self == "" {
		log.Warnw("cannot determine this blox's kubo peer ID", "err", err)
		writeNasCredentialsStatus(w, http.StatusServiceUnavailable, "unavailable")
		return
	}
	if req.BloxPeerID != self {
		log.Warnw("nas-credentials request targets a different blox", "blox_peer_id", req.BloxPeerID)
		writeNasCredentialsStatus(w, http.StatusBadRequest, "blox_peer_mismatch")
		return
	}

	f, err := os.Open(nasCredentialsPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeNasCredentialsStatus(w, http.StatusNotFound, "not_provisioned")
			return
		}
		log.Errorw("cannot open NAS credentials file", "err", err)
		writeNasCredentialsStatus(w, http.StatusInternalServerError, "error")
		return
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, nasCredentialsMaxBytes+1))
	if err != nil || len(data) > nasCredentialsMaxBytes {
		log.Errorw("cannot read NAS credentials file", "err", err, "bytes", len(data))
		writeNasCredentialsStatus(w, http.StatusInternalServerError, "error")
		return
	}
	var creds nasCredentialsFile
	if err := json.Unmarshal(data, &creds); err != nil || creds.Username == "" || creds.Password == "" {
		// Never log the file contents: they hold the password.
		log.Errorw("NAS credentials file is malformed")
		writeNasCredentialsStatus(w, http.StatusInternalServerError, "malformed")
		return
	}

	hostname, _ := os.Hostname()
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(NasCredentialsResponse{
		Status:    "ok",
		Username:  creds.Username,
		Password:  creds.Password,
		Share:     creds.Share,
		CreatedAt: creds.CreatedAt,
		Hostname:  hostname,
	})
	log.Infow("NAS credentials returned to the owner")
}

// NasCredentials is the P2P client-side method for retrieving the NAS credentials.
func (bl *FxBlockchain) NasCredentials(ctx context.Context, to peer.ID, r NasCredentialsRequest) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionNasCredentials, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}
}
