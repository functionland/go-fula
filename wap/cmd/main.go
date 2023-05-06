package main

import (
	"context"
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

	if !isConnected {
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
					break loop2
				}
			}
		}
	}

	ticker3 := time.NewTicker(5 * time.Second) // Check the connection every 5 seconds

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

	closer := server.Serve(blox.BloxCommandInitOnly, "", "")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("Shutting down wap")
	closer.Close()
}
