package pool_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/functionland/go-fula/pool"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/fluent"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

// ExamplePool_DiscoverPeersViaPubSub starts a pool named "my-pool" across three nodes, connects two of the nodes to
// the other one to facilitate a path for pubsub to propagate and shows all three nodes discover
// each other using pubsub.
func ExamplePool_DiscoverPeersViaPubSub() {
	const poolName = "my-pool"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	n1, err := pool.New(pool.WithPoolName(poolName), pool.WithHost(h1))
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
	n2, err := pool.New(pool.WithPoolName(poolName), pool.WithHost(h2))
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
	n3, err := pool.New(pool.WithPoolName(poolName), pool.WithHost(h3))
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
			break
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
	// Instantiated node in pool my-pool with ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// Instantiated node in pool my-pool with ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// Instantiated node in pool my-pool with ID: QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
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

// ExamplePool_ExchangeDagBetweenPoolNodes starts up a pool with 2 nodes, stores a sample DAG in
// one node and fetches it via GraphSync from the other node.
func ExamplePool_ExchangeDagBetweenPoolNodes() {
	const poolName = "my-pool"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	n1, err := pool.New(pool.WithPoolName(poolName), pool.WithHost(h1))
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
	n2, err := pool.New(pool.WithPoolName(poolName), pool.WithHost(h2))
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

	// Generate a sample DAG and store it on node 1 (n1) in the pool
	leaf := fluent.MustBuildMap(basicnode.Prototype.Map, 1, func(na fluent.MapAssembler) {
		na.AssembleEntry("this").AssignBool(true)
	})
	leafLink, err := n1.Store(ctx, leaf)
	if err != nil {
		panic(err)
	}
	root := fluent.MustBuildMap(basicnode.Prototype.Map, 2, func(na fluent.MapAssembler) {
		na.AssembleEntry("that").AssignInt(42)
		na.AssembleEntry("leafLink").AssignLink(leafLink)
	})
	rootLink, err := n1.Store(ctx, root)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s stored IPLD data with links:\n    root: %s\n    leaf:%s\n", h1.ID(), rootLink, leafLink)

	// Fetch the sample DAG stored on node 1 from node 2 by only asking for the root link.
	// Because fetch implementation is recursive, it should fetch the leaf link too.
	if err := n2.Fetch(ctx, h1.ID(), rootLink); err != nil {
		panic(err)
	}

	// Assert that n2 now has both root and leaf links
	if exists, err := n2.Has(ctx, rootLink); err != nil {
		panic(err)
	} else if !exists {
		panic("expected n2 to have fetched the entire sample DAG")
	} else {
		fmt.Printf("%s successfully fetched:\n    link: %s\n    from %s\n", h2.ID(), rootLink, h1.ID())
		n, err := n2.Load(ctx, rootLink, basicnode.Prototype.Any)
		if err != nil {
			panic(err)
		}
		var buf bytes.Buffer
		if err := dagjson.Encode(n, &buf); err != nil {
			panic(err)
		}
		fmt.Printf("    content: %s\n", buf.String())
	}
	if exists, err := n2.Has(ctx, leafLink); err != nil {
		panic(err)
	} else if !exists {
		panic("expected n2 to have fetched the entire sample DAG")
	} else {
		fmt.Printf("%s successfully fetched:\n    link: %s\n    from %s\n", h2.ID(), leafLink, h1.ID())
		n, err := n2.Load(ctx, leafLink, basicnode.Prototype.Any)
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
	// Instantiated node in pool my-pool with ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// Instantiated node in pool my-pool with ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT stored IPLD data with links:
	//     root: bafyreihpspvqaii6nvjphmrerg4grecxcvkxaz7thfzhnu4drczbhvqpoq
	//     leaf:bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	// QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF successfully fetched:
	//     link: bafyreihpspvqaii6nvjphmrerg4grecxcvkxaz7thfzhnu4drczbhvqpoq
	//     from QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	//     content: {"leafLink":{"/":"bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34"},"that":42}
	// QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF successfully fetched:
	//     link: bafyreidulpo7on77a6pkq7c6da5mlj4n2p3av2zjomrpcpeht5zqgafc34
	//     from QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	//     content: {"this":true}
}
