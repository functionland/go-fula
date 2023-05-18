package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	blox "github.com/functionland/go-fula/wap/cmd/blox"
	"github.com/functionland/go-fula/wap/pkg/server"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/main")

func main() {
	logging.SetLogLevel("*", os.Getenv("LOG_LEVEL"))
	ctx := context.Background()

	isConnected := false

	// Check if "/internal/config.yaml" file exists
	configExists := true
	if _, err := os.Stat("/internal/config.yaml"); os.IsNotExist(err) {
		log.Info("File /internal/config.yaml does not exist")
		configExists = false
	} else {
		log.Info("File /internal/config.yaml exists")
	}

	log.Info("Waiting for the system to connect to Wi-Fi")
	if !isConnected {
		log.Info("Wi-Fi is still not connected and system is activating the hotspot mode")
		if err := wifi.StartHotspot(ctx, true); err != nil {
			log.Errorw("start hotspot on startup", "err", err)
		}
		log.Info("Access point enabled on startup")
	} else {
		log.Info("Wi-Fi already connected")
	}

	// Start the server in a separate goroutine
	serverCloser := make(chan io.Closer, 1)
	stopServer := make(chan struct{}, 1)
	go func() {
		closer := server.Serve(blox.BloxCommandInitOnly, "", "")
		serverCloser <- closer
		<-stopServer
		closer.Close()
	}()

	if !isConnected && configExists {
		timeout2 := time.After(30 * time.Second)
		ticker2 := time.NewTicker(3 * time.Second)
		log.Info("Wi-Fi is not connected")
		err := wifi.ConnectToSavedWifi(ctx)
		if err != nil {
			log.Errorw("Connecting to saved wifi failed with error", "err", err)
		}
	loop2:
		for {
			select {
			case <-timeout2:
				log.Info("Waiting for the system to connect to saved Wi-Fi timeout passed")
				break loop2
			case <-ticker2.C:
				log.Info("Waiting for the system to connect to saved Wi-Fi periodic check")
				if wifi.CheckIfIsConnected(ctx) == nil {
					isConnected = true
					//stopServer <- struct{}{} //Stopping server
					break loop2
				}
			}
		}
	} else if isConnected {
		log.Info("Wi-Fi is already connected")
		//stopServer <- struct{}{} //Stopping server
	}

	ticker3 := time.NewTicker(600 * time.Second) // Check the connection every 300 seconds

	for range ticker3.C {
		err := wifi.CheckConnection(5 * time.Second)
		if err == nil {
			log.Info("Connected to a wifi network")
		} else {
			log.Info("Not connected to a wifi network")
			if err := wifi.StartHotspot(ctx, true); err != nil {
				log.Errorw("start hotspot on startup", "err", err)
			}
		}
	}

	// Wait for the signal to terminate
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("Shutting down wap")

	// Close the server
	select {
	case closer := <-serverCloser:
		closer.Close()
	default:
		log.Info("Server not started, nothing to close")
	}
}
