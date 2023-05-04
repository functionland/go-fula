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
	log.Info("Waiting for the system to connect to Wi-Fi")
	wifi.ConnectToSavedWifi(ctx)

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(2 * time.Second)

	var isConnected bool
loop:
	for {
		select {
		case <-timeout:
			break loop
		case <-ticker.C:
			if wifi.CheckIfIsConnected(ctx) == nil {
				isConnected = true
				break loop
			}
		}
	}

	if !isConnected {
		log.Info("Wi-Fi is not connected")
		if err := wifi.StartHotspot(ctx, true); err != nil {
			log.Errorw("start hotspot on startup", "err", err)
		}
		log.Info("Access point enabled on startup")
	} else {
		log.Info("Wi-Fi already connected")
	}

	closer := server.Serve(blox.BloxCommandInitOnly, "", "")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("Shutting down wap")
	closer.Close()
}
