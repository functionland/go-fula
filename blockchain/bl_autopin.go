package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/functionland/go-fula/wap/pkg/config"
	wifi "github.com/functionland/go-fula/wap/pkg/wifi"
	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// handleAutoPinPair handles the auto-pin-pair action on the device side.
// It stores pinning credentials in box_props.json and returns a pairing secret.
func (bl *FxBlockchain) handleAutoPinPair(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionAutoPinPair, "from", from)
	defer r.Body.Close()

	var req AutoPinPairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debugw("cannot parse request body", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	if req.PinningToken == "" || req.PinningEndpoint == "" {
		http.Error(w, `{"message":"pinning_token and pinning_endpoint are required"}`, http.StatusBadRequest)
		return
	}

	// Read current properties
	props, err := config.ReadProperties()
	if err != nil {
		// File may not exist yet; start fresh
		props = make(map[string]interface{})
	}

	// Reject if already paired
	if existingToken, ok := props["auto_pin_token"]; ok && existingToken != nil && existingToken != "" {
		http.Error(w, `{"message":"already paired, unpair first"}`, http.StatusConflict)
		return
	}

	// Generate pairing secret
	pairingSecret := uuid.New().String()

	// Store credentials
	props["auto_pin_token"] = req.PinningToken
	props["auto_pin_endpoint"] = req.PinningEndpoint
	props["auto_pin_pairing_secret"] = pairingSecret

	if err := config.WriteProperties(props); err != nil {
		log.Errorw("failed to write properties", "err", err)
		http.Error(w, `{"message":"failed to save pairing config"}`, http.StatusInternalServerError)
		return
	}

	// Get hardware ID for response
	hardwareID, err := wifi.GetHardwareID()
	if err != nil {
		log.Warnw("failed to get hardware ID", "err", err)
		hardwareID = ""
	}

	resp := AutoPinPairResponse{
		Status:        "paired",
		PairingSecret: pairingSecret,
		HardwareID:    hardwareID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// handleAutoPinRefresh handles refreshing the pinning service token.
func (bl *FxBlockchain) handleAutoPinRefresh(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionAutoPinRefresh, "from", from)
	defer r.Body.Close()

	var req AutoPinRefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debugw("cannot parse request body", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	if req.PinningToken == "" {
		http.Error(w, `{"message":"pinning_token is required"}`, http.StatusBadRequest)
		return
	}

	props, err := config.ReadProperties()
	if err != nil {
		http.Error(w, `{"message":"not paired"}`, http.StatusNotFound)
		return
	}

	if _, ok := props["auto_pin_token"]; !ok {
		http.Error(w, `{"message":"not paired"}`, http.StatusNotFound)
		return
	}

	props["auto_pin_token"] = req.PinningToken
	if err := config.WriteProperties(props); err != nil {
		log.Errorw("failed to write properties", "err", err)
		http.Error(w, `{"message":"failed to update token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AutoPinRefreshResponse{Status: "refreshed"})
}

// handleAutoPinUnpair removes auto-pin configuration.
func (bl *FxBlockchain) handleAutoPinUnpair(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionAutoPinUnpair, "from", from)
	defer r.Body.Close()

	props, err := config.ReadProperties()
	if err != nil {
		http.Error(w, `{"message":"not paired"}`, http.StatusNotFound)
		return
	}

	delete(props, "auto_pin_token")
	delete(props, "auto_pin_endpoint")
	delete(props, "auto_pin_pairing_secret")

	if err := config.WriteProperties(props); err != nil {
		log.Errorw("failed to write properties", "err", err)
		http.Error(w, `{"message":"failed to remove pairing config"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AutoPinUnpairResponse{Status: "unpaired"})
}

// AutoPinPair is the P2P client-side method for the mobile bridge.
func (bl *FxBlockchain) AutoPinPair(ctx context.Context, to peer.ID, r AutoPinPairRequest) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionAutoPinPair, &buf)
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

// AutoPinRefresh is the P2P client-side method for the mobile bridge.
func (bl *FxBlockchain) AutoPinRefresh(ctx context.Context, to peer.ID, r AutoPinRefreshRequest) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionAutoPinRefresh, &buf)
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

// AutoPinUnpair is the P2P client-side method for the mobile bridge.
func (bl *FxBlockchain) AutoPinUnpair(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionAutoPinUnpair, nil)
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
