package blox_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/functionland/go-fula/blox"
	"github.com/functionland/go-fula/exchange"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/client/rpc"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/fluent"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/multiformats/go-multiaddr"
	ma "github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("fula/mockserver")

// requestLoggerMiddleware logs the details of each request
func requestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request
		body, _ := io.ReadAll(r.Body)
		log.Debugw("Received request", "url", r.URL.Path, "method", r.Method, "body", string(body))
		if r.URL.Path == "/fula/pool/vote" {
			fmt.Printf("Voted on 12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q %s", string(body))
		} else if r.URL.Path == "/fula/manifest/batch_storage" {
			fmt.Printf("Stored manifest: %s", string(body))
		}

		// Create a new io.Reader from the read body as the original body is now drained
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}
func startMockServer(addr string) *http.Server {
	handler := http.NewServeMux()

	handler.HandleFunc("/fula/pool/join", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"pool_id": 1,
			"account": "12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q",
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/cancel_join", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"pool_id": 1,
			"account": "12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q",
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/poolrequests", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"poolrequests": []map[string]interface{}{
				{
					"pool_id":        1,
					"account":        "12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q",
					"voted":          []string{},
					"positive_votes": 0,
					"peer_id":        "12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/all", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"pools": []map[string]interface{}{
				{
					"pool_id":   1,
					"creator":   "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY",
					"pool_name": "PoolTest1",
					"region":    "Ontario",
					"parent":    nil,
					"participants": []string{
						"12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM",
						"12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX",
						"12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/users", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"users": []map[string]interface{}{
				{
					"account":         "12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q",
					"pool_id":         nil,
					"request_pool_id": 1,
					"peer_id":         "12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q",
				},
				{
					"account":         "12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM",
					"pool_id":         1,
					"request_pool_id": nil,
					"peer_id":         "12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM",
				},
				{
					"account":         "12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX",
					"pool_id":         1,
					"request_pool_id": nil,
					"peer_id":         "12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX",
				},
				{
					"account":         "12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX",
					"pool_id":         1,
					"request_pool_id": nil,
					"peer_id":         "12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/vote", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"pool_id": 1,
			"account": "12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q",
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/manifest/available", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"manifests": []map[string]interface{}{
				{
					"pool_id": 1,
					"manifest_metadata": map[string]interface{}{
						"job": map[string]string{
							"engine": "IPFS",
							"uri":    "bafyr4ifwexg2ka3kueem7wp36diai4wzqswkdiqscw2su4llkhgwcmq2ji",
							"work":   "Storage",
						},
					},
					"replication_available": 2,
				},
				{
					"pool_id": 1,
					"manifest_metadata": map[string]interface{}{
						"job": map[string]string{
							"engine": "IPFS",
							"uri":    "bafyr4iauqnsshryxfg2262z6mqev5fyef7gmgjk54skmtggnplehusyno4",
							"work":   "Storage",
						},
					},
					"replication_available": 1,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/leave", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"pool_id": 1,
			"account": "12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q",
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/manifest/batch_storage", func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			CIDs   []string `json:"cid"`
			PoolID int      `json:"pool_id"`
		}

		// Decode the JSON body of the request
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Make sure to close the request body
		defer r.Body.Close()

		// Use the CIDs from the request in the response
		response := map[string]interface{}{
			"pool_id": reqBody.PoolID,
			"cid":     reqBody.CIDs,
		}

		// Encode the response as JSON
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	handler.HandleFunc("/cid/", func(w http.ResponseWriter, r *http.Request) {
		// Extract the CID from the URL path
		cid := strings.TrimPrefix(r.URL.Path, "/cid/")

		// Prepare the ContextID based on the CID
		var contextID string
		switch cid {
		case "bafyr4iauqnsshryxfg2262z6mqev5fyef7gmgjk54skmtggnplehusyno4":
			contextID = base64.StdEncoding.EncodeToString([]byte("12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX"))
		case "bafyr4ifwexg2ka3kueem7wp36diai4wzqswkdiqscw2su4llkhgwcmq2ji":
			contextID = base64.StdEncoding.EncodeToString([]byte("12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM"))
		default:
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		// Create the response
		response := map[string]interface{}{
			"MultihashResults": []map[string]interface{}{
				{
					"Multihash": "HiCJpK9N9aiHbWJ40eq3r0Lns3qhnLSUviVYdcBJD4jWjQ==",
					"ProviderResults": []map[string]interface{}{
						{
							"ContextID": contextID,
							"Metadata":  "gcA/",
							"Provider": map[string]interface{}{
								"ID":    "12D3KooWFmfEsXjWotvqJ6B3ASXx1w3p6udj8R9f344a9JTu2k4R",
								"Addrs": []string{"/dns/hub.dev.fx.land/tcp/40004/p2p/12D3KooWFmfEsXjWotvqJ6B3ASXx1w3p6udj8R9f344a9JTu2k4R"},
							},
						},
					},
				},
			},
		}

		// Set Content-Type header and send the response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Wrap the handlers with the logging middleware
	loggedHandler := requestLoggerMiddleware(handler)

	// Create an HTTP server
	server := &http.Server{
		Addr:    addr,
		Handler: loggedHandler,
	}
	// Start the server in a new goroutine
	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			panic(err) // Handle the error as you see fit
		}
	}()

	// Give the server a moment to start
	time.Sleep(time.Millisecond * 100)

	return server
}

func generateIdentity(id int) crypto.PrivKey {
	var pid crypto.PrivKey
	switch id {
	case 1: //12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
		key1 := "CAESQJ5GGDgYMGs8eWNCSotGC/qnuw3pfwtG6XcAumHc4CR33IrywkIsmSlMOK7RdP78RgFmYgyrZxz7fP1xux0I88w="
		km1, err := base64.StdEncoding.DecodeString(key1)
		if err != nil {
			panic(err)
		}
		pid, err = crypto.UnmarshalPrivateKey(km1)
		if err != nil {
			panic(err)
		}

	case 2: //12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
		key2 := "CAESQHSuiy3FbrTSh7MzXI6coF52bTXrtx3ZorFzIbKnZeBAbQGC6PMp90hKgAiM4yW5/TkRBQhqgPN99AwdiLOS27Q="
		km2, err := base64.StdEncoding.DecodeString(key2)
		if err != nil {
			panic(err)
		}
		pid, err = crypto.UnmarshalPrivateKey(km2)
		if err != nil {
			panic(err)
		}

	case 3: //12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX
		key3 := "CAESQHAfwsoKLRHraOpYeV6DBjWeG4B9PpSWLyMym2modqej6vuMoMJ5FiA1ivOyihgJxeqKsVue/9cjKlxSNoMQCrQ="
		km3, err := base64.StdEncoding.DecodeString(key3)
		if err != nil {
			panic(err)
		}
		pid, err = crypto.UnmarshalPrivateKey(km3)
		if err != nil {
			panic(err)
		}

	case 4: //12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q
		key4 := "CAESQCKbGJG9XDbfUEMjie3vZYVk9RgXHXCLjTMeBidltp396IK4gNRCMmGbjZeG+ZN4FC+yCLDNB1Vzbg66DaeHvCU="
		km4, err := base64.StdEncoding.DecodeString(key4)
		if err != nil {
			panic(err)
		}
		pid, err = crypto.UnmarshalPrivateKey(km4)
		if err != nil {
			panic(err)
		}
	}
	return pid
}

// Example_poolDiscoverPeersViaPubSub starts a pool named "1" across three nodes, connects two of the nodes to
// the other one to facilitate a path for pubsub to propagate and shows all three nodes discover
// each other using pubsub.
func Example_poolDiscoverPeersViaPubSub() {
	server := startMockServer("127.0.0.1:4000")
	defer func() {
		// Shutdown the server after test
		if err := server.Shutdown(context.Background()); err != nil {
			panic(err) // Handle the error as you see fit
		}
	}()

	const poolName = "1"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Elevate log level to show internal communications.
	if err := logging.SetLogLevel("*", "info"); err != nil {
		panic(err)
	}

	// Use a deterministic random generator to generate deterministic
	// output for the example.
	identity1 := libp2p.Identity(generateIdentity(1))

	h1, err := libp2p.New(identity1)
	if err != nil {
		panic(err)
	}
	log.Infow("h1 value generated", "h1", h1.ID()) //12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	n1, err := blox.New(blox.WithPoolName(poolName), blox.WithHost(h1))
	if err != nil {
		panic(err)
	}
	if err := n1.Start(ctx); err != nil {
		panic(err)
	}
	defer n1.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h1.ID().String())

	identity2 := libp2p.Identity(generateIdentity(2))

	h2, err := libp2p.New(identity2)
	if err != nil {
		panic(err)
	}
	log.Infow("h2 value generated", "h2", h2.ID()) //12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	n2, err := blox.New(blox.WithPoolName(poolName), blox.WithHost(h2))
	if err != nil {
		panic(err)
	}
	if err := n2.Start(ctx); err != nil {
		panic(err)
	}
	defer n2.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h2.ID().String())

	identity3 := libp2p.Identity(generateIdentity(3))

	h3, err := libp2p.New(identity3)
	if err != nil {
		panic(err)
	}
	log.Infow("h3 value generated", "h3", h3.ID()) //12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX
	if err != nil {
		panic(err)
	}
	n3, err := blox.New(blox.WithPoolName(poolName), blox.WithHost(h3))
	if err != nil {
		panic(err)
	}
	if err := n3.Start(ctx); err != nil {
		panic(err)
	}
	defer n3.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h3.ID().String())

	// Connect n1 to n2 and n3 so that there is a path for gossip propagation.
	// Note that we are not connecting n2 to n3 as they should discover
	// each other via pool's iexist announcements.
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	if err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}); err != nil {
		panic(err)
	}
	h1.Peerstore().AddAddrs(h3.ID(), h3.Addrs(), peerstore.PermanentAddrTTL)
	if err = h1.Connect(ctx, peer.AddrInfo{ID: h3.ID(), Addrs: h3.Addrs()}); err != nil {
		panic(err)
	}

	// Wait until the nodes discover each other
	for {
		if len(h1.Peerstore().Peers()) == 4 &&
			len(h2.Peerstore().Peers()) == 4 &&
			len(h3.Peerstore().Peers()) == 4 {
			break
		} else {
			log.Infow("h1.Peerstore().Peers() is waitting", "h1.Peerstore().Peers()", h1.Peerstore().Peers())
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(time.Second)
		}
	}

	h1Peers := h1.Peerstore().Peers()
	fmt.Printf("%s peerstore contains %d nodes:\n", h1.ID(), len(h1Peers))
	for _, id := range h1Peers {
		fmt.Printf("- %s\n", id)
	}

	h2Peers := h2.Peerstore().Peers()
	fmt.Printf("%s peerstore contains %d nodes:\n", h2.ID(), len(h2Peers))
	for _, id := range h2Peers {
		fmt.Printf("- %s\n", id)
	}

	h3Peers := h3.Peerstore().Peers()
	fmt.Printf("%s peerstore contains %d nodes:\n", h3.ID(), len(h3Peers))
	for _, id := range h2Peers {
		fmt.Printf("- %s\n", id)
	}

	// Unordered output:
	// Instantiated node in pool 1 with ID: 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// Instantiated node in pool 1 with ID: 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	// Instantiated node in pool 1 with ID: 12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX
	// 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM peerstore contains 4 nodes:
	// - 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	// - 12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX
	// - 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// - 12D3KooWFmfEsXjWotvqJ6B3ASXx1w3p6udj8R9f344a9JTu2k4R
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX peerstore contains 4 nodes:
	// - 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	// - 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// - 12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX
	// - 12D3KooWFmfEsXjWotvqJ6B3ASXx1w3p6udj8R9f344a9JTu2k4R
	// 12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX peerstore contains 4 nodes:
	// - 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	// - 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// - 12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX
	// - 12D3KooWFmfEsXjWotvqJ6B3ASXx1w3p6udj8R9f344a9JTu2k4R
}

func updatePoolName(newPoolName string) error {
	return nil
}

/*func Example_announcements() {
	fmt.Println("*********test Started******")
	server := startMockServer("127.0.0.1:4000")
	defer func() {
		// Shutdown the server after test
		if err := server.Shutdown(context.Background()); err != nil {
			panic(err) // Handle the error as you see fit
		}
	}()
	fmt.Println("*********server Started******")
	const poolName = "1"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fmt.Print("*********HERE1******")
	// Elevate log level to show internal communications.
	if err := logging.SetLogLevel("*", "info"); err != nil {
		panic(err)
	}
	fmt.Print("*********HERE2******")
	// Use a deterministic random generator to generate deterministic
	// output for the example.
	rng := rand.New(rand.NewSource(42))

	// Instantiate the first node in the pool
	pid1, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h1, err := libp2p.New(libp2p.Identity(pid1))
	if err != nil {
		panic(err)
	}
	n1, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h1),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		blox.WithPingCount(5),
	)
	if err != nil {
		panic(err)
	}
	fmt.Print("*********HERE3******")
	if err := n1.Start(ctx); err != nil {
		panic(err)
	}
	defer n1.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h1.ID().String())

	// Instantiate the second node in the pool
	pid2, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h2, err := libp2p.New(libp2p.Identity(pid2))
	if err != nil {
		panic(err)
	}
	n2, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h2),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		blox.WithPingCount(5),
	)
	if err != nil {
		panic(err)
	}
	if err := n2.Start(ctx); err != nil {
		panic(err)
	}
	defer n2.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h2.ID().String())

	// Instantiate the third node in the pool
	pid3, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h3, err := libp2p.New(libp2p.Identity(pid3))
	if err != nil {
		panic(err)
	}
	n3, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h3),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		blox.WithPingCount(5),
	)
	if err != nil {
		panic(err)
	}
	if err := n3.Start(ctx); err != nil {
		panic(err)
	}
	defer n3.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h3.ID().String())

	// Connect n1 to n2 and n3 so that there is a path for gossip propagation.
	// Note that we are not connecting n2 to n3 as they should discover
	// each other via pool's iexist announcements.
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	if err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}); err != nil {
		panic(err)
	}
	h1.Peerstore().AddAddrs(h3.ID(), h3.Addrs(), peerstore.PermanentAddrTTL)
	if err = h1.Connect(ctx, peer.AddrInfo{ID: h3.ID(), Addrs: h3.Addrs()}); err != nil {
		panic(err)
	}

	// Wait until the nodes discover each other
	for {
		if len(h1.Peerstore().Peers()) == 5 &&
			len(h2.Peerstore().Peers()) == 5 &&
			len(h3.Peerstore().Peers()) == 5 {
			break
		} else {
			fmt.Printf("%s peerstore contains %d nodes:\n", h1.ID(), len(h1.Peerstore().Peers()))
			fmt.Printf("%s peerstore contains %d nodes:\n", h2.ID(), len(h2.Peerstore().Peers()))
			fmt.Printf("%s peerstore contains %d nodes:\n", h3.ID(), len(h3.Peerstore().Peers()))
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(time.Second)
		}
	}

	h1Peers := h1.Peerstore().Peers()
	fmt.Printf("Finally %s peerstore contains %d nodes:\n", h1.ID(), len(h1Peers))
	for _, id := range h1Peers {
		fmt.Printf("- %s\n", id)
	}

	h2Peers := h2.Peerstore().Peers()
	fmt.Printf("Finally %s peerstore contains %d nodes:\n", h2.ID(), len(h2Peers))
	for _, id := range h2Peers {
		fmt.Printf("- %s\n", id)
	}

	h3Peers := h3.Peerstore().Peers()
	fmt.Printf("Finally %s peerstore contains %d nodes:\n", h3.ID(), len(h3Peers))
	for _, id := range h3Peers {
		fmt.Printf("- %s\n", id)
	}

	// Instantiate the fourth node not in the pool
	pid4, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h4, err := libp2p.New(libp2p.Identity(pid4))
	if err != nil {
		panic(err)
	}
	n4, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h4),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		blox.WithPingCount(5),
	)
	if err != nil {
		panic(err)
	}
	if err := n4.Start(ctx); err != nil {
		panic(err)
	}
	defer n4.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h4.ID().String())

	//Manually adding h4 as it is not in the same pool
	h4.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), peerstore.PermanentAddrTTL)
	if err = h4.Connect(ctx, peer.AddrInfo{ID: h1.ID(), Addrs: h1.Addrs()}); err != nil {
		panic(err)
	}
	h4.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	if err = h4.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}); err != nil {
		panic(err)
	}
	h4.Peerstore().AddAddrs(h3.ID(), h3.Addrs(), peerstore.PermanentAddrTTL)
	if err = h4.Connect(ctx, peer.AddrInfo{ID: h3.ID(), Addrs: h3.Addrs()}); err != nil {
		panic(err)
	}

	// Wait until the fourth node discover others
	for {
		if len(h4.Peerstore().Peers()) == 5 {
			break
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(time.Second)
		}
	}

	h4Peers := h4.Peerstore().Peers()
	fmt.Printf("Finally %s peerstore contains %d nodes:\n", h4.ID(), len(h4Peers))
	for _, id := range h4Peers {
		fmt.Printf("- %s\n", id)
	}

	n4.AnnounceJoinPoolRequestPeriodically(ctx)

	// Wait until the fourth node discover others
	for {
		members := n4.GetBlMembers()
		if len(members) == 4 {
			for id, status := range members {
				memberInfo := fmt.Sprintf("Member ID: %s, Status: %v", id.String(), status)
				fmt.Println(memberInfo)
			}
			break
		} else {
			fmt.Println(members)
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(2 * time.Second)
		}
	}

	// Unordered output:
	//Instantiated node in pool 1 with ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// Instantiated node in pool 1 with ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// Instantiated node in pool 1 with ID: QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// Finally QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT peerstore contains 5 nodes:
	// - 12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// Finally QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF peerstore contains 5 nodes:
	// - 12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// Finally QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA peerstore contains 5 nodes:
	// - 12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// Instantiated node in pool 1 with ID: QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// Finally QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe peerstore contains 5 nodes:
	// - 12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// Member ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT, Status: 2
	// Member ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF, Status: 2
	// Member ID: QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA, Status: 2
	// Member ID: QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe, Status: 1
}*/

func Example_testMockserver() {
	server := startMockServer("127.0.0.1:4000")
	defer func() {
		// Shutdown the server after test
		if err := server.Shutdown(context.Background()); err != nil {
			panic(err) // Handle the error as you see fit
		}
	}()
	// Define the URL
	url := "http://127.0.0.1:4000/cid/bafyr4iauqnsshryxfg2262z6mqev5fyef7gmgjk54skmtggnplehusyno4"

	// Send a GET request to the server
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal("Error making GET request:", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response body:", err)
	}

	// Convert the body to a string and log it
	fmt.Println("Response:", string(body))

	// Unordered output:
	// Response: {"MultihashResults":[{"Multihash":"HiCJpK9N9aiHbWJ40eq3r0Lns3qhnLSUviVYdcBJD4jWjQ==","ProviderResults":[{"ContextID":"MTJEM0tvb1dIOXN3amVDeXVSNnV0ektVMVVzcGlXNVJER3pBRnZORHdxa1Q1YlVId3V4WA==","Metadata":"gcA/","Provider":{"Addrs":["/dns/hub.dev.fx.land/tcp/40004/p2p/12D3KooWFmfEsXjWotvqJ6B3ASXx1w3p6udj8R9f344a9JTu2k4R"],"ID":"12D3KooWFmfEsXjWotvqJ6B3ASXx1w3p6udj8R9f344a9JTu2k4R"}}]}]}
}

func Example_encode64Test() {
	originalString := "12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX"

	// Encode to Base64
	encodedString := base64.StdEncoding.EncodeToString([]byte(originalString))
	fmt.Println("Encoded:", encodedString)

	// Decode from Base64
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedString)
	if err != nil {
		fmt.Println("Decode error:", err)
		return
	}
	decodedString := string(decodedBytes)
	fmt.Println("Decoded:", decodedString)

	// Check if original and decoded are the same
	if originalString == decodedString {
		fmt.Println("Success: Original and decoded strings are the same.")
	} else {
		fmt.Println("Error: Original and decoded strings are different.")
	}

	// Unordered output:
	// Encoded: MTJEM0tvb1dIOXN3amVDeXVSNnV0ektVMVVzcGlXNVJER3pBRnZORHdxa1Q1YlVId3V4WA==
	// Decoded: 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	// Success: Original and decoded strings are the same.
}

func Example_storeManifest() {
	server := startMockServer("127.0.0.1:4000")
	defer func() {
		// Shutdown the server after test
		if err := server.Shutdown(context.Background()); err != nil {
			panic(err) // Handle the error as you see fit
		}
	}()

	const poolName = "1"
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// Elevate log level to show internal communications.
	if err := logging.SetLogLevel("*", "debug"); err != nil {
		panic(err)
	}

	// Use a deterministic random generator to generate deterministic
	// output for the example.
	h1, err := libp2p.New(libp2p.Identity(generateIdentity(1)))
	if err != nil {
		panic(err)
	}
	n1, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h1),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		blox.WithPingCount(5),
		blox.WithExchangeOpts(
			exchange.WithDhtProviderOptions(
				dht.ProtocolExtension(protocol.ID("/"+poolName)),
				dht.ProtocolPrefix("/fula"),
				dht.Resiliency(1),
				dht.Mode(dht.ModeAutoServer),
			),
		),
	)
	if err != nil {
		panic(err)
	}
	if err := n1.Start(ctx); err != nil {
		panic(err)
	}
	defer n1.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h1.ID().String())

	h2, err := libp2p.New(libp2p.Identity(generateIdentity(2)))
	if err != nil {
		panic(err)
	}
	n2, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h2),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		blox.WithPingCount(5),
		blox.WithExchangeOpts(
			exchange.WithDhtProviderOptions(
				dht.ProtocolExtension(protocol.ID("/"+poolName)),
				dht.ProtocolPrefix("/fula"),
				dht.Resiliency(1),
				dht.Mode(dht.ModeAutoServer),
			),
		),
	)
	if err != nil {
		panic(err)
	}
	if err := n2.Start(ctx); err != nil {
		panic(err)
	}
	defer n2.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h2.ID().String())

	// Instantiate the third node in the pool
	h3, err := libp2p.New(libp2p.Identity(generateIdentity(3)))
	if err != nil {
		panic(err)
	}
	n3, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h3),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		blox.WithPingCount(5),
		blox.WithExchangeOpts(
			exchange.WithDhtProviderOptions(
				dht.ProtocolExtension(protocol.ID("/"+poolName)),
				dht.ProtocolPrefix("/fula"),
				dht.Resiliency(1),
				dht.Mode(dht.ModeAutoServer),
			),
			exchange.WithIpniGetEndPoint("http://127.0.0.1:4000/cid/"),
		),
	)
	if err != nil {
		panic(err)
	}
	if err := n3.Start(ctx); err != nil {
		panic(err)
	}
	defer n3.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h3.ID().String())

	// Instantiate the third node in the pool
	h4, err := libp2p.New(libp2p.Identity(generateIdentity(4)))
	if err != nil {
		panic(err)
	}
	n4, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h4),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		blox.WithPingCount(5),
		blox.WithExchangeOpts(
			exchange.WithDhtProviderOptions(
				dht.ProtocolExtension(protocol.ID("/"+poolName)),
				dht.ProtocolPrefix("/fula"),
				dht.Resiliency(1),
				dht.Mode(dht.ModeAutoServer),
			),
			exchange.WithIpniGetEndPoint("http://127.0.0.1:4000/cid/"),
		),
	)
	if err != nil {
		panic(err)
	}
	if err := n4.Start(ctx); err != nil {
		panic(err)
	}
	defer n4.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h4.ID().String())

	// Connect n1 to n2.
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	if err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}); err != nil {
		panic(err)
	}
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), peerstore.PermanentAddrTTL)
	if err = h2.Connect(ctx, peer.AddrInfo{ID: h1.ID(), Addrs: h1.Addrs()}); err != nil {
		panic(err)
	}
	h3.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), peerstore.PermanentAddrTTL)
	if err = h3.Connect(ctx, peer.AddrInfo{ID: h1.ID(), Addrs: h1.Addrs()}); err != nil {
		panic(err)
	}
	h4.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), peerstore.PermanentAddrTTL)
	if err = h4.Connect(ctx, peer.AddrInfo{ID: h1.ID(), Addrs: h1.Addrs()}); err != nil {
		panic(err)
	}

	for {
		if len(h1.Peerstore().Peers()) >= 3 &&
			len(h2.Peerstore().Peers()) >= 3 &&
			len(h3.Peerstore().Peers()) >= 3 &&
			len(h4.Peerstore().Peers()) >= 3 {
			break
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(time.Second)
		}
	}

	// Authorize exchange between the two nodes
	if err := n1.SetAuth(ctx, h1.ID(), h2.ID(), true); err != nil {
		panic(err)
	}
	if err := n2.SetAuth(ctx, h2.ID(), h1.ID(), true); err != nil {
		panic(err)
	}

	// Generate a sample DAG and store it on node 1 (n1) in the pool, which we will pull from n1
	n1leaf := fluent.MustBuildMap(basicnode.Prototype.Map, 1, func(na fluent.MapAssembler) {
		na.AssembleEntry("this").AssignBool(true)
	})
	n1leafLink, err := n1.Store(ctx, n1leaf)
	if err != nil {
		panic(err)
	}
	n1Root := fluent.MustBuildMap(basicnode.Prototype.Map, 2, func(na fluent.MapAssembler) {
		na.AssembleEntry("that").AssignInt(42)
		na.AssembleEntry("oneLeafLink").AssignLink(n1leafLink)
	})
	n1RootLink, err := n1.Store(ctx, n1Root)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s stored IPLD data with links:\n    root: %s\n    leaf:%s\n", h1.ID(), n1RootLink, n1leafLink)

	// Generate a sample DAG and store it on node 2 (n1) in the pool, which we will push to n1
	n2leaf := fluent.MustBuildMap(basicnode.Prototype.Map, 1, func(na fluent.MapAssembler) {
		na.AssembleEntry("that").AssignBool(false)
	})
	n2leafLink, err := n2.Store(ctx, n2leaf)
	if err != nil {
		panic(err)
	}
	n2Root := fluent.MustBuildMap(basicnode.Prototype.Map, 2, func(na fluent.MapAssembler) {
		na.AssembleEntry("this").AssignInt(24)
		na.AssembleEntry("anotherLeafLink").AssignLink(n2leafLink)
	})
	n2RootLink, err := n2.Store(ctx, n2Root)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s stored IPLD data with links:\n    root: %s\n    leaf:%s\n", h1.ID(), n1RootLink, n1leafLink)

	fmt.Println("exchanging by Pull...")
	// Pull the sample DAG stored on node 1 from node 2 by only asking for the root link.
	// Because fetch implementation is recursive, it should fetch the leaf link too.
	if err := n2.Pull(ctx, h1.ID(), n1RootLink); err != nil {
		panic(err)
	}

	// Assert that n2 now has both root and leaf links
	if exists, err := n2.Has(ctx, n1RootLink); err != nil {
		panic(err)
	} else if !exists {
		panic("expected n2 to have fetched the entire sample DAG")
	} else {
		fmt.Printf("%s successfully fetched:\n    link: %s\n    from %s\n", h2.ID(), n1RootLink, h1.ID())
		n, err := n2.Load(ctx, n1RootLink, basicnode.Prototype.Any)
		if err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		if err := dagjson.Encode(n, &buf); err != nil {
			panic(err)
		}
		fmt.Printf("    content: %s\n", buf.String())
	}
	if exists, err := n2.Has(ctx, n1leafLink); err != nil {
		panic(err)
	} else if !exists {
		panic("expected n2 to have fetched the entire sample DAG")
	} else {
		fmt.Printf("%s successfully fetched:\n    link: %s\n    from %s\n", h2.ID(), n1leafLink, h1.ID())
		n, err := n2.Load(ctx, n1leafLink, basicnode.Prototype.Any)
		if err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		if err := dagjson.Encode(n, &buf); err != nil {
			panic(err)
		}
		fmt.Printf("    content: %s\n", buf.String())
	}

	fmt.Println("exchanging by Push...")
	// Push the sample DAG stored on node 2 to node 1 by only pushing the root link.
	// Because Push implementation is recursive, it should push the leaf link too.
	if err := n2.Push(ctx, h1.ID(), n2RootLink); err != nil {
		panic(err)
	}

	// Since push is an asynchronous operation, wait until background push is finished
	// by periodically checking if link is present on node 1.
	for {
		if exists, _ := n1.Has(ctx, n2RootLink); exists {
			break
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(time.Second)
		}
	}

	// Assert that n1 now has both root and leaf links
	if exists, err := n1.Has(ctx, n2RootLink); err != nil {
		panic(err)
	} else if !exists {
		panic("expected n2 to have pushed the entire sample DAG")
	} else {
		fmt.Printf("%s successfully pushed:\n    link: %s\n    from %s\n", h2.ID(), n1RootLink, h1.ID())
		n, err := n1.Load(ctx, n2RootLink, basicnode.Prototype.Any)
		if err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		if err := dagjson.Encode(n, &buf); err != nil {
			panic(err)
		}
		fmt.Printf("    content: %s\n", buf.String())
	}
	if exists, err := n1.Has(ctx, n2leafLink); err != nil {
		panic(err)
	} else if !exists {
		panic("expected n2 to have pushed the entire sample DAG")
	} else {
		fmt.Printf("%s successfully pushed:\n    link: %s\n    from %s\n", h2.ID(), n1leafLink, h1.ID())
		n, err := n1.Load(ctx, n2leafLink, basicnode.Prototype.Any)
		if err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		if err := dagjson.Encode(n, &buf); err != nil {
			panic(err)
		}
		fmt.Printf("    content: %s\n", buf.String())
	}
	err = n1.ProvideLinkByDht(n2leafLink)
	if err != nil {
		fmt.Print("Error happened in ProvideLinkByDht")
		panic(err)
	}
	peerlist3, err := n3.FindLinkProvidersByDht(n2leafLink)
	if err != nil {
		fmt.Print("Error happened in FindLinkProvidersByDht3")
		panic(err)
	}

	// Iterate over the slice and print the peer ID of each AddrInfo
	for _, addrInfo := range peerlist3 {
		fmt.Printf("Found %s on %s\n", n2leafLink, addrInfo.ID.String()) // ID.String() converts the peer ID to a string
	}

	n3.FetchAvailableManifestsAndStore(ctx, 2)
	time.Sleep(1 * time.Second)
	n4.FetchAvailableManifestsAndStore(ctx, 2)

	// Output:
	// Instantiated node in pool 1 with ID: 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// Instantiated node in pool 1 with ID: 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	// Instantiated node in pool 1 with ID: 12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX
	// Instantiated node in pool 1 with ID: 12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q
	// 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM stored IPLD data with links:
	//     root: bafyr4ifwexg2ka3kueem7wp36diai4wzqswkdiqscw2su4llkhgwcmq2ji
	//     leaf:bafyr4iauqnsshryxfg2262z6mqev5fyef7gmgjk54skmtggnplehusyno4
	// 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM stored IPLD data with links:
	//     root: bafyr4ifwexg2ka3kueem7wp36diai4wzqswkdiqscw2su4llkhgwcmq2ji
	//     leaf:bafyr4iauqnsshryxfg2262z6mqev5fyef7gmgjk54skmtggnplehusyno4
	// exchanging by Pull...
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully fetched:
	//     link: bafyr4ifwexg2ka3kueem7wp36diai4wzqswkdiqscw2su4llkhgwcmq2ji
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"oneLeafLink":{"/":"bafyr4iauqnsshryxfg2262z6mqev5fyef7gmgjk54skmtggnplehusyno4"},"that":42}
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully fetched:
	//     link: bafyr4iauqnsshryxfg2262z6mqev5fyef7gmgjk54skmtggnplehusyno4
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"this":true}
	// exchanging by Push...
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully pushed:
	//     link: bafyr4ifwexg2ka3kueem7wp36diai4wzqswkdiqscw2su4llkhgwcmq2ji
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"anotherLeafLink":{"/":"bafyr4iaab3lel4ykjcyzqajx5np2uluetwvfyv3ujupxt5qs57owhpo6ty"},"this":24}
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully pushed:
	//     link: bafyr4iauqnsshryxfg2262z6mqev5fyef7gmgjk54skmtggnplehusyno4
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"that":false}
	// Found bafyr4iaab3lel4ykjcyzqajx5np2uluetwvfyv3ujupxt5qs57owhpo6ty on 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// Voted on 12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q {"pool_id":1,"account":"12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q","vote_value":true}
	// Voted on 12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q {"pool_id":1,"account":"12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q","vote_value":true}
	// Voted on 12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q {"pool_id":1,"account":"12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q","vote_value":true}
	// Stored manifest: {"cid":["bafyr4ifwexg2ka3kueem7wp36diai4wzqswkdiqscw2su4llkhgwcmq2ji"],"pool_id":1}
}

type Config struct {
	StaticRelays []string

	// ForceReachabilityPrivate configures weather the libp2p should always think that it is behind
	// NAT.
	ForceReachabilityPrivate bool

	// AllowTransientConnection allows transient connectivity via relay when direct connection is
	// not possible. Defaults to enabled if unspecified.
	AllowTransientConnection bool
}

//const devRelay = "/dns/relay2.functionyard.fula.network/tcp/4001/p2p/12D3KooW9rygvHzDeciGf1DmPLfnNWb9GDPAzuvLydUdxizCsNay"

const devRelay = "/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"

func NewConfig() *Config {
	return &Config{
		StaticRelays:             []string{devRelay},
		ForceReachabilityPrivate: true,
		AllowTransientConnection: true,
	}
}

func Example_blserver() {
	server := startMockServer("127.0.0.1:4000")
	defer func() {
		// Shutdown the server after test
		if err := server.Shutdown(context.Background()); err != nil {
			panic(err) // Handle the error as you see fit
		}
	}()

	const poolName = "1"
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	//ctx = network.WithUseTransient(ctx, "fx.exchange")
	defer cancel()

	// Elevate log level to show internal communications.
	if err := logging.SetLogLevel("*", "info"); err != nil {
		panic(err)
	}

	// Use a deterministic random generator to generate deterministic
	// output for the example.
	cfg := NewConfig()

	// ListenAddr configure
	ListenAddrsConfig1 := []string{"/ip4/0.0.0.0/tcp/40001", "/ip4/0.0.0.0/udp/40001/quic", "/ip4/0.0.0.0/udp/40001/quic-v1", "/ip4/0.0.0.0/udp/40001/quic-v1/webtransport"}
	listenAddrs1 := make([]multiaddr.Multiaddr, 0, len(ListenAddrsConfig1)+1)
	// Convert string addresses to multiaddr and append to listenAddrs
	for _, addrString := range ListenAddrsConfig1 {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			panic(fmt.Errorf("invalid multiaddress: %w", err))
		}
		listenAddrs1 = append(listenAddrs1, addr)
	}
	// Add the relay multiaddress
	relayAddr2, err := multiaddr.NewMultiaddr("/p2p-circuit")
	if err != nil {
		panic(fmt.Errorf("error creating relay multiaddress: %w", err))
	}
	listenAddrs1 = append(listenAddrs1, relayAddr2)
	///
	ListenAddrsConfig2 := []string{"/ip4/0.0.0.0/tcp/40002", "/ip4/0.0.0.0/udp/40002/quic"}
	listenAddrs2 := make([]multiaddr.Multiaddr, 0, len(ListenAddrsConfig2)+1)
	// Convert string addresses to multiaddr and append to listenAddrs
	for _, addrString := range ListenAddrsConfig2 {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			panic(fmt.Errorf("invalid multiaddress: %w", err))
		}
		listenAddrs2 = append(listenAddrs2, addr)
	}
	// Add the relay multiaddress
	listenAddrs2 = append(listenAddrs2, relayAddr2)
	///
	ListenAddrsConfig3 := []string{"/ip4/0.0.0.0/tcp/40003", "/ip4/0.0.0.0/udp/40003/quic"}
	listenAddrs3 := make([]multiaddr.Multiaddr, 0, len(ListenAddrsConfig3)+1)
	// Convert string addresses to multiaddr and append to listenAddrs
	for _, addrString := range ListenAddrsConfig3 {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			panic(fmt.Errorf("invalid multiaddress: %w", err))
		}
		listenAddrs3 = append(listenAddrs3, addr)
	}
	// Add the relay multiaddress
	listenAddrs3 = append(listenAddrs3, relayAddr2)
	///
	ListenAddrsConfig4 := []string{"/ip4/0.0.0.0/tcp/40004", "/ip4/0.0.0.0/udp/40004/quic"}
	listenAddrs4 := make([]multiaddr.Multiaddr, 0, len(ListenAddrsConfig4)+1)
	// Convert string addresses to multiaddr and append to listenAddrs
	for _, addrString := range ListenAddrsConfig4 {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			panic(fmt.Errorf("invalid multiaddress: %w", err))
		}
		listenAddrs4 = append(listenAddrs4, addr)
	}
	// Add the relay multiaddress
	listenAddrs4 = append(listenAddrs4, relayAddr2)

	//End of ListenAddr configure
	hopts := []libp2p.Option{
		//libp2p.ListenAddrs(listenAddrs...),
		libp2p.EnableNATService(),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
	}
	sr := make([]peer.AddrInfo, 0, len(cfg.StaticRelays))
	for _, relay := range cfg.StaticRelays {
		rma, err := multiaddr.NewMultiaddr(relay)
		if err != nil {
			fmt.Println(err)
		}
		rai, err := peer.AddrInfoFromP2pAddr(rma)
		if err != nil {
			fmt.Println(err)
		}
		sr = append(sr, *rai)
	}
	libp2p.EnableAutoRelayWithStaticRelays(sr, autorelay.WithNumRelays(1))
	hopts = append(hopts, libp2p.ForceReachabilityPrivate())

	// Instantiate the first node in the pool
	hopts1 := append(hopts, libp2p.Identity(generateIdentity(1)))
	hopts1 = append(hopts1, libp2p.ListenAddrs(listenAddrs1...))
	h1, err := libp2p.New(hopts1...)
	if err != nil {
		panic(err)
	}

	n1, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h1),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{devRelay}),
		blox.WithPingCount(5),
		blox.WithMaxPingTime(100),
		blox.WithMinSuccessPingRate(80),
	)
	if err != nil {
		panic(err)
	}
	if err := n1.Start(ctx); err != nil {
		panic(err)
	}
	defer n1.Shutdown(ctx)
	log.Debugf("n1 Instantiated node in pool %s with ID: %s\n", poolName, h1.ID().String())

	// Instantiate the second node in the pool
	hopts2 := append(hopts, libp2p.Identity(generateIdentity(2)))
	hopts2 = append(hopts2, libp2p.ListenAddrs(listenAddrs2...))
	h2, err := libp2p.New(hopts2...)
	if err != nil {
		panic(err)
	}

	n2, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h2),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{devRelay}),
		blox.WithPingCount(5),
		blox.WithMaxPingTime(100),
		blox.WithMinSuccessPingRate(80),
	)
	if err != nil {
		panic(err)
	}
	if err := n2.Start(ctx); err != nil {
		panic(err)
	}
	defer n2.Shutdown(ctx)
	log.Debugf("n2 Instantiated node in pool %s with ID: %s\n", poolName, h2.ID().String())

	// Instantiate the third node in the pool
	hopts3 := append(hopts, libp2p.Identity(generateIdentity(3)))
	hopts3 = append(hopts3, libp2p.ListenAddrs(listenAddrs3...))
	h3, err := libp2p.New(hopts3...)
	if err != nil {
		panic(err)
	}

	n3, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h3),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{devRelay}),
		blox.WithPingCount(5),
		blox.WithMaxPingTime(100),
		blox.WithMinSuccessPingRate(80),
	)
	if err != nil {
		panic(err)
	}
	if err := n3.Start(ctx); err != nil {
		panic(err)
	}
	defer n3.Shutdown(ctx)
	log.Debugf("n3 Instantiated node in pool %s with ID: %s\n", poolName, h3.ID().String())

	if err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}); err != nil {
		panic(err)
	}
	if err = h1.Connect(ctx, peer.AddrInfo{ID: h3.ID(), Addrs: h3.Addrs()}); err != nil {
		panic(err)
	}

	// Wait until the nodes discover each other
	for {
		if len(h1.Peerstore().Peers()) >= 3 &&
			len(h2.Peerstore().Peers()) >= 3 &&
			len(h3.Peerstore().Peers()) >= 3 {
			break
		} else {
			h1Peers := h1.Peerstore().Peers()
			log.Debugf("n1 Only %s peerstore contains %d nodes:\n", h1.ID(), len(h1Peers))
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(time.Second)
		}
	}

	h1Peers := h1.Peerstore().Peers()
	log.Debugf("n1 Finally %s peerstore contains %d nodes:\n", h1.ID(), len(h1Peers))
	for _, id := range h1Peers {
		log.Debugf("- %s\n", id)
	}

	h2Peers := h2.Peerstore().Peers()
	log.Debugf("n2 Finally %s peerstore contains %d nodes:\n", h2.ID(), len(h2Peers))
	for _, id := range h2Peers {
		log.Debugf("- %s\n", id)
	}

	h3Peers := h3.Peerstore().Peers()
	log.Debugf("n3 Finally %s peerstore contains %d nodes:\n", h3.ID(), len(h3Peers))
	for _, id := range h3Peers {
		log.Debugf("- %s\n", id)
	}

	// Instantiate the fourth node not in the pool
	log.Debug("Now creating pid of n4")
	log.Debug("Now creating host of n4")
	hopts4 := append(hopts, libp2p.Identity(generateIdentity(4)))
	hopts4 = append(hopts4, libp2p.ListenAddrs(listenAddrs4...))
	h4, err := libp2p.New(hopts4...)
	if err != nil {
		log.Errorw("An error happened in creating libp2p  instance of n4", "Err", err)
		panic(err)
	}
	log.Debug("Now creating blox for n4")
	n4, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h4),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{devRelay}),
		blox.WithPingCount(5),
		blox.WithMaxPingTime(100),
		blox.WithMinSuccessPingRate(80),
	)
	if err != nil {
		log.Errorw("An error happened in creating blox instance of n4", "Err", err)
		panic(err)
	}
	log.Debug("Now starting n4")
	if err := n4.Start(ctx); err != nil {
		log.Errorw("An error happened in starting instance of n4", "Err", err)
		panic(err)
	}
	defer n4.Shutdown(ctx)
	log.Debugf("n4 Instantiated node in pool %s with ID: %s\n", poolName, h4.ID().String())

	n4.AnnounceJoinPoolRequestPeriodically(ctx)

	if err = h1.Connect(ctx, peer.AddrInfo{ID: h4.ID(), Addrs: h4.Addrs()}); err != nil {
		panic(err)
	}
	if err = h2.Connect(ctx, peer.AddrInfo{ID: h4.ID(), Addrs: h4.Addrs()}); err != nil {
		panic(err)
	}
	if err = h3.Connect(ctx, peer.AddrInfo{ID: h4.ID(), Addrs: h4.Addrs()}); err != nil {
		panic(err)
	}

	// Wait until the fourth node discover others
	for {
		members := n4.GetBlMembers()

		for id, status := range members {
			memberInfo := fmt.Sprintf("Member ID: %s, Status: %v", id.String(), status)
			log.Debugln(memberInfo)
		}
		if len(members) >= 2 {
			break
		} else {
			log.Debugln(members)
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(2 * time.Second)
		}
	}

	h4Peers := h4.Peerstore().Peers()
	log.Debugf("n4 Finally %s peerstore contains %d nodes:\n", h4.ID(), len(h4Peers))
	for _, id := range h4Peers {
		log.Debugf("- %s\n", id)
	}

	//wait for 60 seconds
	count := 1
	for {
		count = count + 1
		if count > 60 {
			break
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(1 * time.Second)
		}
	}

	// Unordered output:
	// Voted on 12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q {"pool_id":1,"account":"12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q","vote_value":true}
	// Voted on 12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q {"pool_id":1,"account":"12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q","vote_value":true}
	// Voted on 12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q {"pool_id":1,"account":"12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q","vote_value":true}
}

type PingResponse struct {
	Success bool   `json:"Success"`
	Time    int64  `json:"Time"`
	Text    string `json:"Text"`
}

func Example_pingtest() {
	averageDuration := float64(2000)
	successCount := 0
	server := startMockServer("127.0.0.1:4000")
	defer func() {
		// Shutdown the server after test
		if err := server.Shutdown(context.Background()); err != nil {
			panic(err) // Handle the error as you see fit
		}
	}()

	const poolName = "1"
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// Elevate log level to show internal communications.
	if err := logging.SetLogLevel("*", "debug"); err != nil {
		panic(err)
	}

	nodeMultiAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5001")
	if err != nil {
		panic(fmt.Errorf("invalid multiaddress: %w", err))
	}
	node, err := rpc.NewApi(nodeMultiAddr)

	// Use a deterministic random generator to generate deterministic
	// output for the example.
	h1, err := libp2p.New(libp2p.Identity(generateIdentity(1)))
	if err != nil {
		panic(err)
	}
	n1, err := blox.New(
		blox.WithPoolName(poolName),
		blox.WithTopicName(poolName),
		blox.WithHost(h1),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
		blox.WithPingCount(5),
		blox.WithIpfsClient(node),
		blox.WithExchangeOpts(
			exchange.WithDhtProviderOptions(
				dht.ProtocolExtension(protocol.ID("/"+poolName)),
				dht.ProtocolPrefix("/fula"),
				dht.Resiliency(1),
				dht.Mode(dht.ModeAutoServer),
			),
		),
	)
	if err != nil {
		panic(err)
	}
	if err := n1.Start(ctx); err != nil {
		panic(err)
	}
	defer n1.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h1.ID().String())
	PingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Send the ping request
	n1.GetBlMembers()
	rpc := n1.GetIPFSRPC()
	res, err := rpc.Request("ping", "12D3KooWHb38UxY8akVGWZBuFtS3NJ7rJUwd36t3cfkoY7EbgNt9").Send(PingCtx)
	if err != nil {
		fmt.Printf("ping was unsuccessful: %s", err)
		return
	}
	if res.Error != nil {
		fmt.Printf("ping was unsuccessful: %s", res.Error)
		return
	}

	// Process multiple responses using a decoder
	decoder := json.NewDecoder(res.Output)

	var totalTime int64
	var count int

	for decoder.More() {
		var pingResp PingResponse
		err := decoder.Decode(&pingResp)
		if err != nil {
			log.Errorf("error decoding JSON response: %s", err)
			continue
		}

		if pingResp.Text == "" && pingResp.Time > 0 { // Check for empty Text field and Time
			totalTime += pingResp.Time
			count++
		}
	}

	if count > 0 {
		averageDuration = float64(totalTime) / float64(count) / 1e6 // Convert nanoseconds to milliseconds
		successCount = count
	} else {
		fmt.Println("No valid ping responses received")
		return
	}
	if int(averageDuration) > 1 {
		fmt.Printf("Final response: successCount=%d", successCount)
	}
	// Unordered output:
	// Final response: successCount=10
}
