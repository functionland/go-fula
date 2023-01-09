package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (bl *FxBlockchain) Seeded(ctx context.Context, to peer.ID, r SeededRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionSeeded, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) handleSeeded(from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionSeeded, "from", from)
	defer r.Body.Close()

	var p SeededRequest
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		log.Debugw("cannot parse request body", "err", err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	//go func() {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*30)
	defer cancel()
	response, err := bl.callBlockchain(ctx, actionSeeded, p)
	if err != nil {
		log.Errorw("failed to process seeded request", "err", err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	var res SeededResponse
	err1 := json.Unmarshal(response, &res)
	if err1 != nil {
		log.Errorw("failed to format response", "err", err1)
	}

	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Errorw("failed to write response", "err", err)
	}
	//}()
}
