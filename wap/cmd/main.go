package main

import (
	"context"
	"encoding/base64"
	"os"
	"os/signal"
	"syscall"

	"github.com/functionland/go-fula/wap/pkg/server"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("fula/wap/main")

func main() {
	logging.SetLogLevel("*", os.Getenv("LOG_LEVEL"))
	ctx := context.Background()
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

	closer := server.Serve(func(clientPeerId string) (string, error) {
		authorizer, err := peer.Decode(clientPeerId)
		if err != nil {
			return "", err
		}
		km, err := base64.StdEncoding.DecodeString("identitiy?!")
		if err != nil {
			return "", err
		}
		k, err := crypto.UnmarshalPrivateKey(km)
		if err != nil {
			return "", err
		}

		pid, err := peer.IDFromPrivateKey(k)
		if err != nil {
			return "", err
		}
		return pid.String(), nil

	}, "", "")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("Shutting down wap")
	closer.Close()
}
