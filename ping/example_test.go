package ping_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
)

var log = logging.Logger("fula/mockserver")

// requestLoggerMiddleware logs the details of each request
func requestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the request
		body, _ := io.ReadAll(r.Body)
		log.Debugw("Received request", "url", r.URL.Path, "method", r.Method, "body", string(body))
		if r.URL.Path == "/fula/pool/vote" {
			fmt.Printf("Voted on QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe %s", string(body))
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
							"uri":    "bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34",
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
							"uri":    "bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy",
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
func updatePoolName(newPoolName string) error {
	return nil
}

func Example_ping() {
	/*
		server := startMockServer("127.0.0.1:4002")
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
		// Instantiate the first node in the pool
		h1, err := libp2p.New(libp2p.Identity(generateIdentity(1)))
		if err != nil {
			panic(err)
		}
		n1, err := blox.New(
			blox.WithPoolName(poolName),
			blox.WithTopicName(poolName),
			blox.WithHost(h1),
			blox.WithUpdatePoolName(updatePoolName),
			blox.WithBlockchainEndPoint("127.0.0.1:4002"),
			blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
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

		// Instantiate the second node in the pool
		h2, err := libp2p.New(libp2p.Identity(generateIdentity(2)))
		if err != nil {
			panic(err)
		}
		n2, err := blox.New(
			blox.WithPoolName(poolName),
			blox.WithTopicName(poolName),
			blox.WithHost(h2),
			blox.WithUpdatePoolName(updatePoolName),
			blox.WithBlockchainEndPoint("127.0.0.1:4002"),
			blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
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
			blox.WithBlockchainEndPoint("127.0.0.1:4002"),
			blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
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
		if err := n3.Start(ctx); err != nil {
			panic(err)
		}
		defer n3.Shutdown(ctx)
		fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h3.ID().String())

		// Instantiate the fourth node not in the pool
		h4, err := libp2p.New(libp2p.Identity(generateIdentity(4)))
		if err != nil {
			panic(err)
		}
		n4, err := blox.New(
			blox.WithPoolName("0"),
			blox.WithTopicName("0"),
			blox.WithHost(h4),
			blox.WithUpdatePoolName(updatePoolName),
			blox.WithBlockchainEndPoint("127.0.0.1:4002"),
			blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
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
		if err := n4.Start(ctx); err != nil {
			panic(err)
		}
		defer n4.Shutdown(ctx)
		fmt.Printf("Instantiated node in pool %s with ID: %s\n", "0", h4.ID().String())

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
				h1Peers := h1.Peerstore().Peers()
				fmt.Printf("%s peerstore contains %d nodes:\n", h1.ID(), len(h1Peers))
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

		//Manually adding h4 as it is not in the same pool
		h1.Peerstore().AddAddrs(h4.ID(), h4.Addrs(), peerstore.PermanentAddrTTL)
		if err = h1.Connect(ctx, peer.AddrInfo{ID: h4.ID(), Addrs: h4.Addrs()}); err != nil {
			panic(err)
		}
		//Manually adding h4 as it is not in the same pool
		h2.Peerstore().AddAddrs(h4.ID(), h4.Addrs(), peerstore.PermanentAddrTTL)
		if err = h2.Connect(ctx, peer.AddrInfo{ID: h4.ID(), Addrs: h4.Addrs()}); err != nil {
			panic(err)
		}
		//Manually adding h4 as it is not in the same pool
		h3.Peerstore().AddAddrs(h4.ID(), h4.Addrs(), peerstore.PermanentAddrTTL)
		if err = h3.Connect(ctx, peer.AddrInfo{ID: h4.ID(), Addrs: h4.Addrs()}); err != nil {
			panic(err)
		}

		// Wait until the fourth node discover others
		for {
			if len(h4.Peerstore().Peers()) >= 4 {
				break
			}
			select {
			case <-ctx.Done():
				panic(ctx.Err())
			default:
				time.Sleep(time.Second)
			}
		}

		if err = n4.StartPingServer(ctx); err != nil {
			panic(err)
		} else {
			fmt.Print("Node 4 ping server started\n")
		}

		average, rate, err := n1.Ping(ctx, h4.ID())
		if err != nil {
			fmt.Println("Error occured in Ping", "err", err)
			panic(err)
		}
		fmt.Printf("%s ping results success_count: %d:\n", h1.ID(), rate)
		if average < 0 {
			panic("average is 0 for first ping")
		}

		average, rate, err = n2.Ping(ctx, h4.ID())
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s ping results success_count: %d:\n", h2.ID(), rate)
		if average < 0 {
			panic("average is 0 for second ping")
		}

		average, rate, err = n3.Ping(ctx, h4.ID())
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s ping results success_count: %d:", h3.ID(), rate)
		if average < 0 {
			panic("average is 0 for third ping")
		}

		// Unordered output:
		// Instantiated node in pool 1 with ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
		// Instantiated node in pool 1 with ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
		// Instantiated node in pool 1 with ID: QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
		// Instantiated node in pool 0 with ID: QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
		// Finally QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT peerstore contains 4 nodes:
		// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
		// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
		// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
		// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
		// Finally QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF peerstore contains 4 nodes:
		// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
		// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
		// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
		// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
		// Finally QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA peerstore contains 4 nodes:
		// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
		// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
		// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
		// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
		// Node 4 ping server started
		// QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT ping results success_count: 5:
		// QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF ping results success_count: 5:
		// QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA ping results success_count: 5:
	*/
}
