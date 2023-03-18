package main

import (
	"context"
	"time"

	"github.com/functionland/go-fula/wap/pkg/server"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/main")

func main() {
	ctx, cl := context.WithTimeout(context.Background(), time.Second*10)
	defer cl()
	if wifi.CheckIfIsConnected(ctx) != nil {
		log.Info("Wifi already connected")
		if err := wifi.DisableAccessPoint(ctx); err != nil {
			log.Info("Access point disabled on startup")
		}
	} else {
		if err := wifi.EnableAccessPoint(ctx); err != nil {
			log.Info("Access point enabled on startup")
		}
	}
	server.Serve("", "")
}
