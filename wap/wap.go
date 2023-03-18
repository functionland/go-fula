package wap

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/functionland/go-fula/wap/pkg/server"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/main")

func Run(globalCtx context.Context, peerFn func(clientPeerId string) string) io.Closer {
	logging.SetLogLevel("*", os.Getenv("LOG_LEVEL"))
	ctx, cl := context.WithTimeout(globalCtx, time.Second*10)
	defer cl()
	if wifi.CheckIfIsConnected(ctx) != nil {
		if err := wifi.StartHotspot(ctx, true); err != nil {
			log.Errorw("start hotspot on startup", "err", err)
		}
		log.Info("Access point enabled on startup")
	} else {
		// TODO: this code seems unused while using nmcli
		// log.Info("Wifi already connected")
		// if err := wifi.StopHotspot(ctx); err != nil {
		// 	log.Errorw("stop hotspot on startup", "err", err)
		// }
		// log.Info("Access point disabled on startup")
	}

	return server.Serve(peerFn, "", "")
}