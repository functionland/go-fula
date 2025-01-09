package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/multiformats/go-multiaddr"
	"github.com/libp2p/go-libp2p/core/network"
)

// ReadIdentity reads a private key from the given path and returns it.
func ReadIdentity(path string) (crypto.PrivKey, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPrivateKey(bytes)
}

func main() {
	// Define a command-line flag for the identity key path
	idPath := flag.String("identity", "", "Path to the identity.key file")
	flag.Parse()

	if *idPath == "" {
		log.Fatalf("You must specify a path to the identity.key file using the -identity flag")
	}

	// Read the private key from the file
	privKey, err := ReadIdentity(*idPath)
	if err != nil {
		log.Fatalf("Failed to read identity key: %v", err)
	}

	// Generate the Peer ID from the private key
	peerID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		log.Fatalf("Failed to generate peer ID: %v", err)
	}

	fmt.Printf("Using Peer ID: %s\n", peerID.String())

	// Define listen addresses
	listenAddrs := []multiaddr.Multiaddr{
		multiaddr.StringCast("/ip4/0.0.0.0/tcp/4001"),
		multiaddr.StringCast("/ip4/0.0.0.0/udp/4001/quic-v1"),
		multiaddr.StringCast("/ip4/0.0.0.0/udp/4001/quic-v1/webtransport"),
	}

	// Define relay service options with resources and limits
	relayOpts := []relayv2.Option{
		relayv2.WithResources(relayv2.Resources{
			Limit: &relayv2.RelayLimit{
				Duration: 480000000 * time.Second, // Duration in seconds
				Data:     171798691840,       // Data limit in bytes (160 GiB)
			},
			MaxReservations:        2048,
			MaxCircuits:            2048,
			MaxReservationsPerPeer: 2048,
			MaxReservationsPerIP:   2048,
			MaxReservationsPerASN:  2048,
			ReservationTTL:         360000 * time.Hour, // TTL in hours
			BufferSize:             81920,
		}),
	}

	h, err := libp2p.New(
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.Identity(privKey), // Use the private key for identity
		libp2p.EnableRelayService(relayOpts...),
		libp2p.ForceReachabilityPublic(),
	)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}

	for _, addr := range h.Addrs() {
		fmt.Printf("Listening on %s/p2p/%s\n", addr, h.ID().String())
	}

	h.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			fmt.Printf("Peer connected: %s\n", conn.RemotePeer().String())
		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			fmt.Printf("Peer disconnected: %s\n", conn.RemotePeer().String())
		},
	})

	select {}
}
