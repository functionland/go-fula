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
	log.Info("waiting 10s for system to connect to wifi")
	time.Sleep(10 * time.Second)
	if wifi.CheckIfIsConnected(ctx) != nil {
		log.Info("Wifi is not connected")
		if err := wifi.StartHotspot(ctx, true); err != nil {
			log.Errorw("start hotspot on startup", "err", err)
		}
		log.Info("Access point enabled on startup")
	} else {
		// TODO: this code seems unused while using nmcli
		log.Info("Wifi already connected")
		// if err := wifi.StopHotspot(ctx); err != nil {
		// 	log.Errorw("stop hotspot on startup", "err", err)
		// }
		// log.Info("Access point disabled on startup")
	}

	closer := server.Serve(blox.BloxCommandInitOnly, "", "")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("Shutting down wap")
	closer.Close()
}
