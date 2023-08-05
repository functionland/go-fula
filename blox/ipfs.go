package blox

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	wifi "github.com/functionland/go-fula/wap/pkg/wifi"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
)

type (
	pinListResp struct {
		PinLsList struct {
			Keys map[string]pinListKeysType `json:"Keys,omitempty"`
		} `json:"PinLsList"`
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
		var resp pinListResp
		resp.PinLsList.Keys = make(map[string]pinListKeysType)
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
			c, err := cid.Cast([]byte(result.Key))
			if err != nil {
				log.Debugw("failed to cast key to cid", "key", result.Key, "err", err)
				continue
			}
			resp.PinLsList.Keys[c.String()] = pinListKeysType{Type: "fx"} //TODO: what should the type be?
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to pin ls", "err", err)
		}
	})
	// https://docs.ipfs.tech/reference/kubo/rpc/#api-v0-block-stat
	mux.HandleFunc("/api/v0/block/stat", func(w http.ResponseWriter, r *http.Request) {
		c := r.URL.Query().Get("cid")
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
		//Get string value of addresses
		addresses := p.h.Addrs()
		addressStrings := make([]string, len(addresses))
		for i, addr := range addresses {
			addressStrings[i] = addr.String()
		}

		//Get Public Key
		pubKey, err := p.h.ID().ExtractPublicKey()
		if err != nil {
			log.Errorw("Public key is not available", err)
			return
		}
		pubKeyBytes, err := pubKey.Raw()
		if err != nil {
			log.Errorw("Error getting raw public key:", err)
			return
		}

		pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKeyBytes)

		resp := struct {
			Addresses       []string `json:"Addresses"`
			AgentVersion    string   `json:"AgentVersion"`
			ID              string   `json:"ID"`
			ProtocolVersion string   `json:"ProtocolVersion"`
			Protocols       []string `json:"Protocols"`
			PublicKey       string   `json:"PublicKey"`
		}{
			Addresses:       addressStrings,
			AgentVersion:    Version0,
			ID:              p.h.ID().String(),
			ProtocolVersion: "fx_exchange/" + Version0,
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
			Version: "fx-repo@" + Version0,
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
			Hash:           "",
			Size:           uint64(repoSize),
			CumulativeSize: uint64(repoSize),
			Blocks:         numObjects,
			Type:           "directory",
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Errorw("failed to encode response to files stat", "err", err)
		}
	})

	mux.HandleFunc("/", notFoundHandler)

	return mux
}
