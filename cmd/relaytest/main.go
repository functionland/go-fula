package main

import (
	"context"
	"fmt"
	"log"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func main() {
	// Define the multiaddress to connect to
	targetAddr := "/dns/relay.dev.fx.land/tcp/4001/p2p/12D3KooWDRrBaAfPwsGJivBoUw5fE7ZpDiyfUjqgiURq2DEcL835/p2p-circuit/p2p/12D3KooWPtXiYKheTJyrQeNyPwjjPjCPi6QFdAK6qHN9c2U7Qhfe"

	// Parse the multiaddress
	maddr, err := ma.NewMultiaddr(targetAddr)
	if err != nil {
		log.Fatalf("Invalid multiaddress: %v", err)
	}

	// Extract the peer ID and address info from the multiaddress
	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		log.Fatalf("Failed to extract peer info: %v", err)
	}

	// Create a new libp2p host
	host, err := libp2p.New()
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}
	defer host.Close()

	fmt.Printf("Libp2p host created. ID: %s\n", host.ID().String())

	// Connect to the target peer
	fmt.Printf("Attempting to connect to peer: %s\n", info.ID.String())
	if err := host.Connect(context.Background(), *info); err != nil {
		log.Fatalf("Failed to connect to peer: %v", err)
	}

	fmt.Println("Successfully connected to the peer!")
}
