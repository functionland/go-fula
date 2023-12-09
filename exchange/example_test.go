package exchange_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/functionland/go-fula/blox"
	"github.com/functionland/go-fula/exchange"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/fluent"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var log = logging.Logger("fula/dhttest")

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

func Example_provideAfterPull() {
	server := startMockServer("127.0.0.1:4001")
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
		blox.WithBlockchainEndPoint("127.0.0.1:4001"),
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
		blox.WithBlockchainEndPoint("127.0.0.1:4001"),
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
		blox.WithBlockchainEndPoint("127.0.0.1:4001"),
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
		blox.WithBlockchainEndPoint("127.0.0.1:4001"),
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
		if len(h1.Peerstore().Peers()) >= 3 &&
			len(h2.Peerstore().Peers()) >= 3 &&
			len(h3.Peerstore().Peers()) >= 3 {
			break
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(time.Second)
		}
	}

	fmt.Printf("Finally %s peerstore contains >=3 nodes:\n", h1.ID())

	fmt.Printf("Finally %s peerstore contains >=3 nodes:\n", h2.ID())

	fmt.Printf("Finally %s peerstore contains >=3 nodes:\n", h3.ID())

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

	//Store a link in h1 and find providers from h2

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
	fmt.Printf("%s stored IPLD data with links:\n    root: %s\n    leaf:%s\n", h1.ID(), n2RootLink, n2leafLink)

	//n1.UpdateDhtPeers(h2.Peerstore().Peers())
	//n2.UpdateDhtPeers(h1.Peerstore().Peers())

	err = n1.ProvideLinkByDht(n1RootLink)
	if err != nil {
		fmt.Print("Error happened in ProvideLinkByDht")
		panic(err)
	}
	peerlist1, err := n2.FindLinkProvidersByDht(n1RootLink)
	if err != nil {
		fmt.Print("Error happened in FindLinkProvidersByDht")
		panic(err)
	}
	// Iterate over the slice and print the peer ID of each AddrInfo
	for _, addrInfo := range peerlist1 {
		fmt.Printf("Found %s on %s\n", n1RootLink, addrInfo.ID.String()) // ID.String() converts the peer ID to a string
	}

	err = n1.PingDht(h3.ID())
	if err != nil {
		fmt.Print("Error happened in PingDht")
		panic(err)
	}

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
	if err := n2.Push(ctx, h1.ID(), n2leafLink); err != nil {
		panic(err)
	}

	peerlist3, err := n3.FindLinkProvidersByDht(n2leafLink)
	if err != nil {
		fmt.Print("Error happened in FindLinkProvidersByDht3")
		panic(err)
	}

	// Iterate over the slice and print the peer ID of each AddrInfo
	for _, addrInfo := range peerlist3 {
		fmt.Printf("Found %s on %s", n2leafLink, addrInfo.ID.String()) // ID.String() converts the peer ID to a string
	}

	// Unordered output:
	// Instantiated node in pool 1 with ID: 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// Instantiated node in pool 1 with ID: 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	// Instantiated node in pool 1 with ID: 12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX
	// Instantiated node in pool 0 with ID: 12D3KooWRTzN7HfmjoUBHokyRZuKdyohVVSGqKBMF24ZC3tGK78Q
	// Finally 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM peerstore contains >=3 nodes:
	// Finally 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX peerstore contains >=3 nodes:
	// Finally 12D3KooWRde3N9rHE8vEyzTiPMVBvs1RpjS4oaWjVkfAt17412vX peerstore contains >=3 nodes:
	// 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM stored IPLD data with links:
	//     root: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     leaf:bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	// 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM stored IPLD data with links:
	//     root: bafyreiekrzm6lpylw7hhrgvmzqfsek7og6ucqgpzns3ysbc5wj3imfrsge
	//     leaf:bafyreibzxn3zdk6e53h7cvx2sfbbroozp5e3kuvz6t4jfo2hfu4ic2ooc4
	// Found bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy on 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// exchanging by Pull...
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully fetched:
	//     link: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"oneLeafLink":{"/":"bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34"},"that":42}
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully fetched:
	//     link: bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"this":true}
	// exchanging by Push...
	// Found bafyreibzxn3zdk6e53h7cvx2sfbbroozp5e3kuvz6t4jfo2hfu4ic2ooc4 on 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
}

// Example_poolExchangeDagBetweenPoolNodes starts up a pool with 2 nodes, stores a sample DAG in
// one node and fetches it via GraphSync from the other node.
func Example_poolExchangeDagBetweenPoolNodes() {
	server := startMockServer("127.0.0.1:4001")
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
	n1, err := blox.New(blox.WithPoolName(poolName), blox.WithHost(h1))
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
	n2, err := blox.New(blox.WithPoolName(poolName), blox.WithHost(h2))
	if err != nil {
		panic(err)
	}
	if err := n2.Start(ctx); err != nil {
		panic(err)
	}
	defer n2.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h2.ID().String())

	// Connect n1 to n2.
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), peerstore.PermanentAddrTTL)
	if err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}); err != nil {
		panic(err)
	}
	h2.Peerstore().AddAddrs(h1.ID(), h1.Addrs(), peerstore.PermanentAddrTTL)
	if err = h2.Connect(ctx, peer.AddrInfo{ID: h1.ID(), Addrs: h1.Addrs()}); err != nil {
		panic(err)
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

	// Output:
	// Instantiated node in pool 1 with ID: 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// Instantiated node in pool 1 with ID: 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	// 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM stored IPLD data with links:
	//     root: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     leaf:bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	// 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM stored IPLD data with links:
	//     root: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     leaf:bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	// exchanging by Pull...
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully fetched:
	//     link: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"oneLeafLink":{"/":"bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34"},"that":42}
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully fetched:
	//     link: bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"this":true}
	// exchanging by Push...
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully pushed:
	//     link: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"anotherLeafLink":{"/":"bafyreibzxn3zdk6e53h7cvx2sfbbroozp5e3kuvz6t4jfo2hfu4ic2ooc4"},"this":24}
	// 12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX successfully pushed:
	//     link: bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	//     from 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	//     content: {"that":false}
}
