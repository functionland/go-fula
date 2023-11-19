package blox_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/functionland/go-fula/blox"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/fluent"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/multiformats/go-multiaddr"
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
			"account": "QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe",
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/cancel_join", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"pool_id": 1,
			"account": "QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe",
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/poolrequests", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"poolrequests": []map[string]interface{}{
				{
					"pool_id":        1,
					"account":        "QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe",
					"voted":          []string{},
					"positive_votes": 0,
					"peer_id":        "QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe",
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
						"QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT",
						"QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF",
						"QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA",
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
					"account":         "QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe",
					"pool_id":         nil,
					"request_pool_id": 1,
					"peer_id":         "QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe",
				},
				{
					"account":         "QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT",
					"pool_id":         1,
					"request_pool_id": nil,
					"peer_id":         "QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT",
				},
				{
					"account":         "QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF",
					"pool_id":         1,
					"request_pool_id": nil,
					"peer_id":         "QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF",
				},
				{
					"account":         "QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA",
					"pool_id":         1,
					"request_pool_id": nil,
					"peer_id":         "QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/vote", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"pool_id": 1,
			"account": "QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe",
		}
		json.NewEncoder(w).Encode(response)
	})

	handler.HandleFunc("/fula/pool/leave", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"pool_id": 1,
			"account": "QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe",
		}
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
	pid2, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h2, err := libp2p.New(libp2p.Identity(pid2))
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

	// Instantiate the third node in the pool
	pid3, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h3, err := libp2p.New(libp2p.Identity(pid3))
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
		if len(h1.Peerstore().Peers()) == 3 &&
			len(h2.Peerstore().Peers()) == 3 &&
			len(h3.Peerstore().Peers()) == 3 {
			break
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
	// Instantiated node in pool 1 with ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// Instantiated node in pool 1 with ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// Instantiated node in pool 1 with ID: QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT peerstore contains 3 nodes:
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF peerstore contains 3 nodes:
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA peerstore contains 3 nodes:
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
}

// Example_poolExchangeDagBetweenPoolNodes starts up a pool with 2 nodes, stores a sample DAG in
// one node and fetches it via GraphSync from the other node.
func Example_poolExchangeDagBetweenPoolNodes() {
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
	pid2, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h2, err := libp2p.New(libp2p.Identity(pid2))
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
	// Instantiated node in pool 1 with ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// Instantiated node in pool 1 with ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT stored IPLD data with links:
	//     root: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     leaf:bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	// QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT stored IPLD data with links:
	//     root: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     leaf:bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	// exchanging by Pull...
	// QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF successfully fetched:
	//     link: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     from QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	//     content: {"oneLeafLink":{"/":"bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34"},"that":42}
	// QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF successfully fetched:
	//     link: bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	//     from QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	//     content: {"this":true}
	// exchanging by Push...
	// QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF successfully pushed:
	//     link: bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy
	//     from QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	//     content: {"anotherLeafLink":{"/":"bafyreibzxn3zdk6e53h7cvx2sfbbroozp5e3kuvz6t4jfo2hfu4ic2ooc4"},"this":24}
	// QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF successfully pushed:
	//     link: bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	//     from QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	//     content: {"that":false}
}

func updatePoolName(newPoolName string) error {
	return nil
}

func Example_ping() {
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
	pid4, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	h4, err := libp2p.New(libp2p.Identity(pid4))
	if err != nil {
		panic(err)
	}
	n4, err := blox.New(
		blox.WithPoolName("0"),
		blox.WithTopicName("0"),
		blox.WithHost(h4),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithRelays([]string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"}),
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
	rng := rand.New(rand.NewSource(42))
	cfg := NewConfig()

	hopts := []libp2p.Option{
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
	pid1, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	hopts1 := append(hopts, libp2p.Identity(pid1))
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
	fmt.Printf("n1 Instantiated node in pool %s with ID: %s\n", poolName, h1.ID().String())

	// Instantiate the second node in the pool
	pid2, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	hopts2 := append(hopts, libp2p.Identity(pid2))
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
	fmt.Printf("n2 Instantiated node in pool %s with ID: %s\n", poolName, h2.ID().String())

	// Instantiate the third node in the pool
	pid3, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		panic(err)
	}
	hopts3 := append(hopts, libp2p.Identity(pid3))
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
	fmt.Printf("n3 Instantiated node in pool %s with ID: %s\n", poolName, h3.ID().String())

	if err = h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()}); err != nil {
		panic(err)
	}
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
			fmt.Printf("n1 Finally %s peerstore contains %d nodes:\n", h1.ID(), len(h1.Peerstore().Peers()))
		}
		select {
		case <-ctx.Done():
			panic(ctx.Err())
		default:
			time.Sleep(time.Second)
		}
	}

	h1Peers := h1.Peerstore().Peers()
	fmt.Printf("n1 Finally %s peerstore contains %d nodes:\n", h1.ID(), len(h1Peers))
	for _, id := range h1Peers {
		fmt.Printf("- %s\n", id)
	}

	h2Peers := h2.Peerstore().Peers()
	fmt.Printf("n2 Finally %s peerstore contains %d nodes:\n", h2.ID(), len(h2Peers))
	for _, id := range h2Peers {
		fmt.Printf("- %s\n", id)
	}

	h3Peers := h3.Peerstore().Peers()
	fmt.Printf("n3 Finally %s peerstore contains %d nodes:\n", h3.ID(), len(h3Peers))
	for _, id := range h3Peers {
		fmt.Printf("- %s\n", id)
	}

	// Instantiate the fourth node not in the pool
	log.Debug("Now creating pid of n4")
	pid4, _, err := crypto.GenerateECDSAKeyPair(rng)
	if err != nil {
		log.Errorw("An error happened in creating keypair of n4", "Err", err)
		panic(err)
	}
	log.Debug("Now creating host of n4")
	hopts4 := append(hopts, libp2p.Identity(pid4))
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
	fmt.Printf("n4 Instantiated node in pool %s with ID: %s\n", poolName, h4.ID().String())

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
			fmt.Println(memberInfo)
		}
		if len(members) >= 2 {
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

	h4Peers := h4.Peerstore().Peers()
	fmt.Printf("n4 Finally %s peerstore contains %d nodes:\n", h4.ID(), len(h4Peers))
	for _, id := range h4Peers {
		fmt.Printf("- %s\n", id)
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
	// n1 Instantiated node in pool 1 with ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// n2 Instantiated node in pool 1 with ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// n3 Instantiated node in pool 1 with ID: QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// n1 Finally QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT peerstore contains 4 nodes:
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// n2 Finally QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF peerstore contains 4 nodes:
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// n3 Finally QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA peerstore contains 4 nodes:
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// n4 Instantiated node in pool 1 with ID: QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// n4 Finally QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe peerstore contains 4 nodes:
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// Member ID: QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe, Status: 1
	// Member ID: QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA, Status: 2
	// Member ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT, Status: 2
	// Member ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF, Status: 2
	// Voted on QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe {"pool_id":1,"account":"QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe","vote_value":true}
	// Voted on QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe {"pool_id":1,"account":"QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe","vote_value":true}
	// Voted on QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe {"pool_id":1,"account":"QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe","vote_value":true}
}
