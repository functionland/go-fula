package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/functionland/go-fula/wap/pkg/server"
)

func main() {
	connectedCh := make(chan bool, 1)

	mockPeerFn := func(clientPeerId string, bloxSeed string) (string, error) {
		return "test-peer-id", nil
	}

	closer := server.Serve(mockPeerFn, "", "", connectedCh)
	defer closer.Close()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Server is running. Press Ctrl+C to stop.")
	<-sigChan
	fmt.Println("\nShutting down server...")
}
