package blox

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/functionland/go-fula/wap/pkg/config"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/blox")

func BloxCommandInitOnly(clientPeerId string, bloxSeed string) (string, error) {
	command := fmt.Sprintf(config.BLOX_COMMAND, clientPeerId, bloxSeed)
	log.Infof("trying to run the blox command: %s", command)
	ctxCommand, cl := context.WithTimeout(context.Background(), time.Second*10)
	defer cl()
	stdout, _, err := wifi.RunCommand(ctxCommand, command)
	if err != nil {
		return "", err
	}
	bloxPeerId := strings.TrimSpace(strings.Split(stdout, "blox peer ID:")[1])
	if len(bloxPeerId) == 0 {
		return "", fmt.Errorf("empty blox peer id for client peer id: %v", clientPeerId)
	}
	return bloxPeerId, nil
}
