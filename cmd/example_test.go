package cmd_test

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
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/fluent"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipni/index-provider/engine"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("fula/mockserver")

type AppConfig struct {
	PoolName                  string
	StaticRelays              []string
	AllowTransientConnection  bool
	IpniPublishDisabled       bool
	IpniPublishInterval       time.Duration
	IpniPublishDirectAnnounce []string
	Authorizer                string
	AuthorizedPeers           []string
}

// App holds the main application settings
type App struct {
	config             AppConfig
	blockchainEndpoint string
}

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

	handler.HandleFunc("/cid/", func(w http.ResponseWriter, r *http.Request) {
		// Extract the CID from the URL path
		cid := strings.TrimPrefix(r.URL.Path, "/cid/")

		// Prepare the ContextID based on the CID
		var contextID string
		switch cid {
		case "bafyreibzsetfhqrayathm5tkmm7axuljxcas3pbqrncrosx2fiky4wj5gy":
			contextID = base64.StdEncoding.EncodeToString([]byte("12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX"))
		case "bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34":
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
func updatePoolName(newPoolName string) error {
	return nil
}
func updateConfig(p []peer.ID) error {
	return nil
}

// Example_poolDiscoverPeersViaPubSub starts a pool named "1" across three nodes, connects two of the nodes to
// the other one to facilitate a path for pubsub to propagate and shows all three nodes discover
// each other using pubsub.
func Example_main() {
	if err := logging.SetLogLevel("*", "info"); err != nil {
		panic(err)
	}
	log.Info("start")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	log.Info("creating app")
	app := App{
		config: AppConfig{
			PoolName:                  "1",
			StaticRelays:              []string{"/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835"},
			AllowTransientConnection:  true,
			IpniPublishDisabled:       false,
			IpniPublishInterval:       10 * time.Second,
			IpniPublishDirectAnnounce: []string{"https://cid.contact/ingest/announce"},
			AuthorizedPeers:           []string{"12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM", "12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX"},
			Authorizer:                "12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM",
		},
		blockchainEndpoint: "127.0.0.1:4003",
	}
	log.Info("starting server")
	server := startMockServer("127.0.0.1:4003")
	defer func() {
		// Shutdown the server after test
		if err := server.Shutdown(context.Background()); err != nil {
			panic(err) // Handle the error as you see fit
		}
	}()
	log.Info("creating authorizedPeers")
	authorizedPeers := make([]peer.ID, len(app.config.AuthorizedPeers))
	for i, authorizedPeer := range app.config.AuthorizedPeers {
		id, err := peer.Decode(authorizedPeer)
		if err != nil {
			panic(fmt.Errorf("unable to decode authorized peer: %w", err))
		}
		authorizedPeers[i] = id
	}
	log.Info("creating authorizer")
	authorizer, err := peer.Decode(app.config.Authorizer)
	if err != nil {
		panic(err)
	}
	log.Info("creating identity1")
	identity1 := libp2p.Identity(generateIdentity(1))
	log.Info("creating h1")
	h1, err := libp2p.New(identity1)
	if err != nil {
		panic(err)
	}
	log.Infow("h1 value generated", "h1", h1.ID()) //12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	n1, err := blox.New(
		blox.WithHost(h1),
		blox.WithPoolName(app.config.PoolName),
		blox.WithRelays(app.config.StaticRelays),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithBlockchainEndPoint(app.blockchainEndpoint),
		blox.WithPingCount(5),
		blox.WithExchangeOpts(
			exchange.WithUpdateConfig(updateConfig),
			exchange.WithAuthorizer(authorizer),
			exchange.WithAuthorizedPeers(authorizedPeers),
			exchange.WithAllowTransientConnection(app.config.AllowTransientConnection),
			exchange.WithIpniPublishDisabled(app.config.IpniPublishDisabled),
			exchange.WithIpniPublishInterval(app.config.IpniPublishInterval),
			exchange.WithIpniGetEndPoint("http://127.0.0.1:4003/cid/"),
			exchange.WithIpniProviderEngineOptions(
				engine.WithPublisherKind(engine.DataTransferPublisher),
				engine.WithDirectAnnounce(app.config.IpniPublishDirectAnnounce...),
			),
			exchange.WithDhtProviderOptions(
				dht.ProtocolExtension(protocol.ID("/"+app.config.PoolName)),
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
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", app.config.PoolName, h1.ID().String())

	identity2 := libp2p.Identity(generateIdentity(2))

	h2, err := libp2p.New(identity2)
	if err != nil {
		panic(err)
	}
	log.Infow("h2 value generated", "h2", h2.ID()) //12D3KooWH9swjeCyuR6utzKU1UspiW5RDGzAFvNDwqkT5bUHwuxX
	n2, err := blox.New(
		blox.WithHost(h2),
		blox.WithPoolName(app.config.PoolName),
		blox.WithRelays(app.config.StaticRelays),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithBlockchainEndPoint(app.blockchainEndpoint),
		blox.WithPingCount(5),
		blox.WithExchangeOpts(
			exchange.WithUpdateConfig(updateConfig),
			exchange.WithAuthorizer(authorizer),
			exchange.WithAuthorizedPeers(authorizedPeers),
			exchange.WithAllowTransientConnection(app.config.AllowTransientConnection),
			exchange.WithIpniPublishDisabled(app.config.IpniPublishDisabled),
			exchange.WithIpniPublishInterval(app.config.IpniPublishInterval),
			exchange.WithIpniGetEndPoint("http://127.0.0.1:4003/cid/"),
			exchange.WithIpniProviderEngineOptions(
				engine.WithPublisherKind(engine.DataTransferPublisher),
				engine.WithDirectAnnounce(app.config.IpniPublishDirectAnnounce...),
			),
			exchange.WithDhtProviderOptions(
				dht.ProtocolExtension(protocol.ID("/"+app.config.PoolName)),
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
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", app.config.PoolName, h2.ID().String())

	//Connect test
	// Your relayed libp2p address
	h1Addr := "/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835/p2p-circuit/p2p/" + h1.ID().String()

	// Parse the multiaddress
	ma, err := multiaddr.NewMultiaddr(h1Addr)
	if err != nil {
		panic(err)
	}

	// Create peer.AddrInfo
	ai := peer.AddrInfo{
		ID:    h1.ID(),
		Addrs: []multiaddr.Multiaddr{ma},
	}
	if err := h2.Connect(ctx, ai); err != nil {
		panic(err)
	}
	fmt.Println("Connected to peer:", h1.ID())

	//Blockchain test
	res, err := n1.BloxFreeSpace(ctx, h2.ID())
	if err != nil {
		panic(err)
	}
	fmt.Printf("Got free space of node: %s\n", string(res))
	//Careting DAG

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
	fmt.Printf("%s stored IPLD data with links:\n    root: %s\n    leaf:%s\n", h2.ID(), n2RootLink, n2leafLink)

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
