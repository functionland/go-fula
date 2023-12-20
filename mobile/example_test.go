package fulamobile_test

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
	fulamobile "github.com/functionland/go-fula/mobile"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("fula/fulamobile")

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

func Example_poolExchangeDagBetweenClientBlox() {
	server := startMockServer("127.0.0.1:4004")
	defer func() {
		// Shutdown the server after test
		if err := server.Shutdown(context.Background()); err != nil {
			log.Error("Error happened in server.Shutdown")
			panic(err) // Handle the error as you see fit
		}
	}()

	const poolName = "1"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Elevate log level to show internal communications.
	if err := logging.SetLogLevel("*", "debug"); err != nil {
		log.Error("Error happened in logging.SetLogLevel")
		panic(err)
	}

	// Use a deterministic random generator to generate deterministic
	// output for the example.

	// Instantiate the first node in the pool
	h1, err := libp2p.New(libp2p.Identity(generateIdentity(1)))
	if err != nil {
		log.Errorw("Error happened in libp2p.New", "err", err)
		panic(err)
	}
	n1, err := blox.New(
		blox.WithHost(h1),
		blox.WithPoolName("1"),
		blox.WithUpdatePoolName(updatePoolName),
		blox.WithBlockchainEndPoint("127.0.0.1:4004"),
		blox.WithPingCount(5),
		blox.WithExchangeOpts(
			exchange.WithIpniGetEndPoint("http://127.0.0.1:4004/cid/"),
		),
	)
	if err != nil {
		log.Errorw("Error happened in blox.New", "err", err)
		panic(err)
	}
	if err := n1.Start(ctx); err != nil {
		log.Errorw("Error happened in n1.Start", "err", err)
		panic(err)
	}
	defer n1.Shutdown(ctx)
	fmt.Printf("Instantiated node in pool %s with ID: %s\n", poolName, h1.ID().String())

	mcfg := fulamobile.NewConfig()
	mcfg.AllowTransientConnection = true
	bloxAddrString := ""
	if len(h1.Addrs()) > 0 {
		// Convert the first multiaddr to a string
		bloxAddrString = h1.Addrs()[0].String()
		fmt.Println(bloxAddrString)
	} else {
		log.Errorw("Error happened in h1.Addrs", "err", "No addresses in slice")
		panic("No addresses in slice")
	}
	mcfg.BloxAddr = bloxAddrString + "/p2p/" + h1.ID().String()
	mcfg.PoolName = "1"
	mcfg.Exchange = bloxAddrString
	mcfg.BlockchainEndpoint = "127.0.0.1:4004"
	log.Infow("bloxAdd string created", "addr", bloxAddrString+"/p2p/"+h1.ID().String())

	c1, err := fulamobile.NewClient(mcfg)
	if err != nil {
		log.Errorw("Error happened in fulamobile.NewClient", "err", err)
		panic(err)
	}
	// Authorize exchange between the two nodes
	mobilePeerIDString := c1.ID()
	log.Infof("first client created with ID: %s", mobilePeerIDString)
	mpid, err := peer.Decode(mobilePeerIDString)
	if err != nil {
		log.Errorw("Error happened in peer.Decode", "err", err)
		panic(err)
	}
	if err := n1.SetAuth(ctx, h1.ID(), mpid, true); err != nil {
		log.Error("Error happened in n1.SetAuth")
		panic(err)
	}

	err = c1.ConnectToBlox()
	if err != nil {
		log.Errorw("Error happened in c1.ConnectToBlox", "err", err)
		panic(err)
	}
	_, err = c1.BloxFreeSpace()
	if err != nil {
		panic(err)
	}

	rawData := []byte("some raw data")
	fmt.Printf("Original Val is: %s\n", string(rawData))
	rawCodec := int64(0x55)
	linkBytes, err := c1.Put(rawData, rawCodec)
	if err != nil {
		fmt.Printf("Error storing the raw data: %v", err)
		return
	}
	c, err := cid.Cast(linkBytes)
	if err != nil {
		fmt.Printf("Error casting bytes to CID: %v", err)
		return
	}
	fmt.Printf("Stored raw data link: %s\n", c.String())
	log.Infof("Stored raw data link: %s", c.String())

	recentCids, err := c1.ListRecentCidsAsString()
	if err != nil {
		log.Errorw("Error happened in ListRecentCidsAsString", "err", err)
		panic(err)
	}
	for recentCids.HasNext() {
		cid, err := recentCids.Next()
		if err != nil {
			fmt.Printf("Error retrieving next CID: %v", err)
			log.Errorf("Error retrieving next CID: %v", err)
			// Decide if you want to break or continue based on your error handling strategy
			break
		}
		fmt.Printf("recentCid link: %s\n", cid) // Print each CID
		log.Infof("recentCid link: %s", cid)
	}
	fmt.Print("Waiting for 5 seconds\n")
	time.Sleep(5 * time.Second)
	fmt.Printf("Now fetching the link %x\n", linkBytes)
	log.Infof("Now fetching the link %x", linkBytes)

	c2, err := fulamobile.NewClient(mcfg)
	if err != nil {
		log.Errorw("Error happened in fulamobile.NewClient2", "err", err)
		panic(err)
	}
	mobilePeerIDString2 := c2.ID()
	log.Infof("second client created with ID: %s", mobilePeerIDString2)
	mpid2, err := peer.Decode(mobilePeerIDString2)
	if err != nil {
		log.Errorw("Error happened in peer.Decode2", "err", err)
		panic(err)
	}
	if err := n1.SetAuth(ctx, h1.ID(), mpid2, true); err != nil {
		log.Errorw("Error happened in n1.SetAuth2", "err", err)
		panic(err)
	}

	err = c2.ConnectToBlox()
	if err != nil {
		log.Errorw("Error happened in c2.ConnectToBlox", "err", err)
		panic(err)
	}
	/*
		//If you uncomment this section, it just fetches the cid that blox stored and not with the key that mobile has
		ct, err := cid.Decode("bafyreibcwjmj25zyylzw36xaglokmmcbk7c4tqtouaz7qmw6nskx5iqgqi")
		if err != nil {
			fmt.Println("Error decoding CID:", err)
			return
		}
		linkBytes = ct.Bytes()
		//
	*/
	val, err := c2.Get(linkBytes)
	if err != nil {
		log.Errorw("Error happened in c2.Get", "err", err)
		panic(err)
	}
	fmt.Printf("Fetched Val is: %s\n", string(val))
	log.Infof("Fetched Val is: %v", val)
	log.Infof("Original Val is: %v", rawData)
	if !bytes.Equal(val, rawData) {
		panic(fmt.Sprintf("Original data is not equal to fetched data: [original] %s != [fetch] %s", string(val), string(rawData)))
	}

	// Output:
	// Instantiated node in pool 1 with ID: 12D3KooWQfGkPUkoLDEeJE3H3ZTmu9BZvAdbJpmhha8WpjeSLKMM
	// Original Val is: some raw data
	// Stored raw data link: bafkr4ifmwbmdrkxep3mci37ionvgturlylvganap4ch7ouia2ui5tmr4iy
	// recentCid link: bafkr4ifmwbmdrkxep3mci37ionvgturlylvganap4ch7ouia2ui5tmr4iy
	// Waiting for 5 seconds
	// Now fetching the link 01551e20acb05838aae47ed8246fe8736a69d22bc2ea60340fe08ff75100d511d9b23c46
	// Fetched Val is: some raw data
}
