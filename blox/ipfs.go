package blox

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/functionland/go-fula/common"
	wifi "github.com/functionland/go-fula/wap/pkg/wifi"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

type (
	pinListResp struct {
		Keys map[string]pinListKeysType `json:"Keys,omitempty"`
	}
	pinListKeysType struct {
		Type string
	}
)

type SizeStat struct {
	RepoSize   uint64 `json:"RepoSize"`
	StorageMax uint64 `json:"StorageMax"`
}

type RepoInfo struct {
	NumObjects uint64   `json:"NumObjects"`
	RepoPath   string   `json:"RepoPath"`
	SizeStat   SizeStat `json:"SizeStat"`
	Version    string   `json:"Version"`
	RepoSize   uint64   `json:"RepoSize"`
}

type FilesStat struct {
	Blocks         int    `json:"Blocks"`
	CumulativeSize uint64 `json:"CumulativeSize"`
	Hash           string `json:"Hash"`
	Local          bool   `json:"Local,omitempty"` // Optional field. 'omitempty' keyword is used to exclude the field from the output if it's default/zero value
	Size           uint64 `json:"Size"`
	SizeLocal      uint64 `json:"SizeLocal,omitempty"` // Optional field.
	Type           string `json:"Type"`
	WithLocality   bool   `json:"WithLocality,omitempty"` // Optional field.
}

type CidStruct struct {
	Root string `json:"/"`
}

type StatsBitswap struct {
	BlocksReceived   uint64      `json:"BlocksReceived"`
	BlocksSent       uint64      `json:"BlocksSent"`
	DataReceived     uint64      `json:"DataReceived"`
	DataSent         uint64      `json:"DataSent"`
	DupBlksReceived  uint64      `json:"DupBlksReceived"`
	DupDataReceived  uint64      `json:"DupDataReceived"`
	MessagesReceived uint64      `json:"MessagesReceived"`
	Peers            []string    `json:"Peers"`
	ProvideBufLen    int         `json:"ProvideBufLen"`
	Wantlist         []CidStruct `json:"Wantlist"`
}

type StatsBw struct {
	RateIn   float64 `json:"RateIn"`
	RateOut  float64 `json:"RateOut"`
	TotalIn  int64   `json:"TotalIn"`
	TotalOut int64   `json:"TotalOut"`
}

type PeerStats struct {
	Exchanged uint64  `json:"Exchanged"`
	Peer      string  `json:"Peer"`
	Recv      uint64  `json:"Recv"`
	Sent      uint64  `json:"Sent"`
	Value     float64 `json:"Value"`
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	log.Errorw("404 Not Found",
		"method", r.Method,
		"url", r.URL.String(),
		"params", params,
	)
	http.NotFound(w, r)
}

func (p *Blox) ServeIpfsRpc() http.Handler {
	mux := http.NewServeMux()
	// https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-pin-ls
	mux.HandleFunc("/api/v0/pin/ls", func(w http.ResponseWriter, r *http.Request) {
		log.Debug("pin/ls request received")
		queryOptions := query.Query{
			KeysOnly: true,
			Filters:  []query.Filter{},
		}
		// Extract the 'arg' query parameter
		cidStr := r.URL.Query().Get("arg")
		log.Debugw("pin/ls request received with arg", "arg", cidStr)
		// If cidStr is not empty, construct the prefix and create a filter
		if cidStr != "" {
			c, err := cid.Decode(cidStr)
			if err != nil {
				log.Errorw("failed to decode cidStr", "cidStr", cidStr, "err", err)
				http.Error(w, "invalid cid: "+err.Error(), http.StatusBadRequest)
				return
			}

			// Construct the prefix with byte 47 and the CID bytes
			prefix := string(append([]byte{47}, c.Bytes()...))

			// Use FilterKeyPrefix
			queryOptions.Filters = append(queryOptions.Filters, query.FilterKeyPrefix{Prefix: prefix})
			queryOptions.Limit = 1 // Set limit to 1 if arg is provided
		}

		var resp pinListResp
		resp.Keys = make(map[string]pinListKeysType)
		results, err := p.ds.Query(r.Context(), queryOptions)
		if err != nil {
			log.Errorw("failed to query datastore", "err", err)
			http.Error(w, "internal error while querying datastore: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for result := range results.Next() {
			if result.Error != nil {
				log.Errorw("failed to traverse results", "err", err)
				http.Error(w, "internal error while traversing datastore results: "+err.Error(), http.StatusInternalServerError)
				return
			}
			keyBytes := []byte(result.Key) // Convert the key to a byte slice

			if len(keyBytes) > 1 {
				// Slice the keyBytes to remove the first byte
				slicedKeyBytes := keyBytes[1:]

				c, err := cid.Cast(slicedKeyBytes)
				if err != nil {
					log.Debugw("failed to cast sliced key to cid", "slicedKeyBytes", slicedKeyBytes, "err", err)
					continue
				}

				resp.Keys[c.String()] = pinListKeysType{Type: "recursive"}
			} else {
				log.Debugw("key too short to be a valid CID", "keyBytes", keyBytes)
			}
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to pin ls", "err", err)
		}
	})
	// https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-block-stat
	mux.HandleFunc("/api/v0/block/stat", func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("arg")
		if c == "" {
			http.Error(w, "no cid specified", http.StatusBadRequest)
			return
		}
		cd, err := cid.Decode(c)
		if err != nil {
			http.Error(w, "invalid cid: "+err.Error(), http.StatusBadRequest)
			return
		}
		key := toDatastoreKey(cidlink.Link{Cid: cd})
		switch value, err := p.ds.Get(r.Context(), key); {
		case errors.Is(err, datastore.ErrNotFound):
			http.NotFound(w, r)
			return
		case err != nil:
			http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
			return
		default:
			resp := struct {
				Key  string `json:"Key"`
				Size int    `json:"Size"`
			}{
				Key:  cd.String(),
				Size: len(value),
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				log.Errorw("failed to encode response to block stat", "err", err)
			}
		}
	})
	// https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-id
	mux.HandleFunc("/api/v0/id", func(w http.ResponseWriter, r *http.Request) {
		// No libp2p host — return selfPeerID with empty addresses
		pubKeyBase64 := ""
		if p.selfPeerID != "" {
			pubKey, err := p.selfPeerID.ExtractPublicKey()
			if err == nil {
				pubKeyBytes, err := pubKey.Raw()
				if err == nil {
					pubKeyBase64 = base64.StdEncoding.EncodeToString(pubKeyBytes)
				}
			}
		}

		resp := struct {
			Addresses       []string `json:"Addresses"`
			AgentVersion    string   `json:"AgentVersion"`
			ID              string   `json:"ID"`
			ProtocolVersion string   `json:"ProtocolVersion"`
			Protocols       []string `json:"Protocols"`
			PublicKey       string   `json:"PublicKey"`
		}{
			Addresses:       []string{},
			AgentVersion:    common.Version0,
			ID:              p.selfPeerID.String(),
			ProtocolVersion: "fx_exchange/" + common.Version0,
			Protocols:       []string{"fx_exchange"},
			PublicKey:       pubKeyBase64,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to id", "err", err)
		}

	})

	// https://docs.ipfs.tech/reference/kubo/rpc/#api-log-level
	mux.HandleFunc("/api/v0/log/level", func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Message string `json:"Message"`
		}{
			Message: "ignored",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to log level", "err", err)
		}

	})

	// https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-stats-repo Modified for Sugarfunge by adding RepoSize in the root of response
	mux.HandleFunc("/api/v0/stats/repo", func(w http.ResponseWriter, r *http.Request) {
		//get StorageMax
		storage, err := wifi.GetBloxFreeSpace()
		if err != nil {
			log.Errorw("failed to get storage stats", "err", err)
			http.Error(w, "internal error while getting storage stats: "+err.Error(), http.StatusInternalServerError)
			return
		}

		//Get RepoSize and NumObjects
		repoSize := 0
		numObjects := 0
		results, err := p.ds.Query(r.Context(), query.Query{
			KeysOnly: true,
		})
		if err != nil {
			log.Errorw("failed to query datastore", "err", err)
			http.Error(w, "internal error while querying datastore: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for result := range results.Next() {
			if result.Error != nil {
				log.Errorw("failed to traverse results", "err", err)
				http.Error(w, "internal error while traversing datastore results: "+err.Error(), http.StatusInternalServerError)
				return
			}
			repoSize = repoSize + result.Size
			numObjects = numObjects + 1
		}

		resp := RepoInfo{
			NumObjects: uint64(numObjects),
			RepoPath:   p.storeDir,
			RepoSize:   uint64(repoSize),
			SizeStat: SizeStat{
				RepoSize:   uint64(repoSize),
				StorageMax: uint64(storage.Size),
			},
			Version: "fx-repo@" + common.Version0,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to stats repo", "err", err)
		}
	})

	// https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-files-stat
	mux.HandleFunc("/api/v0/files/stat", func(w http.ResponseWriter, r *http.Request) {
		//Get RepoSize and NumObjects
		repoSize := 0
		numObjects := 0
		results, err := p.ds.Query(r.Context(), query.Query{
			KeysOnly: true,
		})
		if err != nil {
			log.Errorw("failed to query datastore", "err", err)
			http.Error(w, "internal error while querying datastore: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for result := range results.Next() {
			if result.Error != nil {
				log.Errorw("failed to traverse results", "err", err)
				http.Error(w, "internal error while traversing datastore results: "+err.Error(), http.StatusInternalServerError)
				return
			}
			repoSize = repoSize + result.Size
			numObjects = numObjects + 1
		}

		resp := FilesStat{
			Hash:           "", //TODO: Get hash of root directory
			Size:           uint64(repoSize),
			CumulativeSize: uint64(repoSize),
			Blocks:         numObjects,
			Type:           "directory",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to files stat", "err", err)
		}
	})

	// https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-stats-bitswap
	mux.HandleFunc("/api/v0/stats/bitswap", func(w http.ResponseWriter, r *http.Request) {
		//Get RepoSize and NumObjects
		repoSize := 0
		numObjects := 0
		results, err := p.ds.Query(r.Context(), query.Query{
			KeysOnly: true,
		})
		if err != nil {
			log.Errorw("failed to query datastore", "err", err)
			http.Error(w, "internal error while querying datastore: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for result := range results.Next() {
			if result.Error != nil {
				log.Errorw("failed to traverse results", "err", err)
				http.Error(w, "internal error while traversing datastore results: "+err.Error(), http.StatusInternalServerError)
				return
			}
			repoSize = repoSize + result.Size
			numObjects = numObjects + 1
		}

		// First, create a slice to store the string representations
		peerStrings := make([]string, len(p.authorizedPeers))

		// Then convert each peer.ID to a string
		for i, peerID := range p.authorizedPeers {
			peerStrings[i] = peerID.String()
		}

		resp := StatsBitswap{
			Wantlist:         []CidStruct{}, // empty slice
			Peers:            peerStrings,
			BlocksReceived:   uint64(numObjects),
			DataReceived:     0,
			DupBlksReceived:  0,
			DupDataReceived:  0,
			MessagesReceived: uint64(numObjects),
			BlocksSent:       0,
			DataSent:         0,
			ProvideBufLen:    0,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to stats bitswap", "err", err)
		}
	})

	// https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-stats-bw
	mux.HandleFunc("/api/v0/stats/bw", func(w http.ResponseWriter, r *http.Request) {
		//Get NumObjects
		numObjects := 0
		results, err := p.ds.Query(r.Context(), query.Query{
			KeysOnly: true,
		})
		if err != nil {
			log.Errorw("failed to query datastore", "err", err)
			http.Error(w, "internal error while querying datastore: "+err.Error(), http.StatusInternalServerError)
			return
		}
		for result := range results.Next() {
			if result.Error != nil {
				log.Errorw("failed to traverse results", "err", err)
				http.Error(w, "internal error while traversing datastore results: "+err.Error(), http.StatusInternalServerError)
				return
			}
			numObjects = numObjects + 1
		}

		resp := StatsBw{
			TotalIn:  int64(numObjects),
			TotalOut: 0,
			RateIn:   0,
			RateOut:  0,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to stats bw", "err", err)
		}
	})

	// https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-bitswap-ledger
	mux.HandleFunc("/api/v0/bitswap/ledger", func(w http.ResponseWriter, r *http.Request) {

		resp := PeerStats{
			Exchanged: 0,
			Peer:      p.selfPeerID.String(),
			Recv:      0,
			Sent:      0,
			Value:     0,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to bitswap ledger", "err", err)
		}
	})

	mux.HandleFunc("/", notFoundHandler)

	return mux
}
