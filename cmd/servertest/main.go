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
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/autorelay"
	"github.com/multiformats/go-multiaddr"
)

func updatePoolName(newPoolName string) error {
	return nil
}

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

func main() {
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
	ListenAddrsConfig := []string{"/ip4/0.0.0.0/tcp/40001", "/ip4/0.0.0.0/udp/40001/quic"}
	listenAddrs := make([]multiaddr.Multiaddr, 0, len(ListenAddrsConfig)+1)
	// Convert string addresses to multiaddr and append to listenAddrs
	for _, addrString := range ListenAddrsConfig {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			panic(fmt.Errorf("invalid multiaddress: %w", err))
		}
		listenAddrs = append(listenAddrs, addr)
	}
	// Add the relay multiaddress
	relayAddr2, err := multiaddr.NewMultiaddr("/p2p-circuit")
	if err != nil {
		panic(fmt.Errorf("error creating relay multiaddress: %w", err))
	}
	listenAddrs = append(listenAddrs, relayAddr2)

	hopts := []libp2p.Option{
		libp2p.ListenAddrs(listenAddrs...),
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
		blox.WithMaxPingTime(10),
		blox.WithMinSuccessPingRate(60),
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
		blox.WithMaxPingTime(10),
		blox.WithMinSuccessPingRate(60),
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
		blox.WithMaxPingTime(10),
		blox.WithMinSuccessPingRate(60),
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
		blox.WithMaxPingTime(10),
		blox.WithMinSuccessPingRate(60),
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
	// n4 Finally QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe peerstore contains 5 nodes:
	// - QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// - 12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835
	// Member ID: QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe, Status: 1
	// Member ID: QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA, Status: 2
	// Member ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT, Status: 2
	// Member ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF, Status: 2
	// Voted on QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe {"pool_id":1,"account":"QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe","vote_value":true}
	// Voted on QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe {"pool_id":1,"account":"QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe","vote_value":true}
	// Voted on QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe {"pool_id":1,"account":"QmUg1bGBZ1rSNt3LZR7kKf9RDy3JtJLZZDZGKrzSP36TMe","vote_value":true}
}
