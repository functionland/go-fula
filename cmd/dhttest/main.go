package main

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
	"github.com/ipld/go-ipld-prime/fluent"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
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

func updatePoolName(newPoolName string) error {
	return nil
}

func main() {
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
		blox.WithSelfPeerID(h1.ID()),
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
		blox.WithSelfPeerID(h2.ID()),
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
		blox.WithSelfPeerID(h3.ID()),
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
		blox.WithSelfPeerID(h4.ID()),
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
		} else {
			fmt.Printf("%s peerstore contains %d nodes:\n", h4.ID(), len(h4.Peerstore().Peers()))
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
		fmt.Print("Error happened here")
		panic(err)
	}
	peerlist1, err := n2.FindLinkProvidersByDht(n1RootLink)
	if err != nil {
		panic(err)
	}
	fmt.Println(peerlist1)

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
	// [{QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT: [/ip4/127.0.0.1/udp/62495/quic-v1/webtransport/certhash/uEiA8nNwsF3xgNvpbRn7TBQgFKFmTFHka5os9D8yXYaeK2Q/certhash/uEiCDw6zn5RGqUe6FHv_bz45QnLPkCoZDwNdx6wETFMx4pA /ip4/192.168.2.14/tcp/42433 /ip4/192.168.2.14/udp/62494/quic-v1 /ip6/::1/udp/62497/quic-v1/webtransport/certhash/uEiA8nNwsF3xgNvpbRn7TBQgFKFmTFHka5os9D8yXYaeK2Q/certhash/uEiCDw6zn5RGqUe6FHv_bz45QnLPkCoZDwNdx6wETFMx4pA /ip4/127.0.0.1/tcp/42433 /ip4/127.0.0.1/udp/62494/quic-v1 /ip4/192.168.2.14/udp/62495/quic-v1/webtransport/certhash/uEiA8nNwsF3xgNvpbRn7TBQgFKFmTFHka5os9D8yXYaeK2Q/certhash/uEiCDw6zn5RGqUe6FHv_bz45QnLPkCoZDwNdx6wETFMx4pA /ip6/::1/tcp/42434 /ip6/::1/udp/62496/quic /ip6/::1/udp/62496/quic-v1 /dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835/p2p-circuit /ip4/127.0.0.1/udp/62494/quic /ip4/192.168.2.14/udp/62494/quic]}]
}
