package blockchain

import (
	"context"
	"encoding/json"

	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/sys/unix"
)

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
)

func (bl *FxBlockchain) BloxFreeSpace(ctx context.Context, to peer.ID) ([]byte, error) {
	var stat unix.Statfs_t
	unix.Statfs("/storage", &stat)

	var Size float32 = float32(stat.Blocks * uint64(stat.Bsize))
	var Avail float32 = float32(stat.Bfree * uint64(stat.Bsize))
	var Used float32 = float32(Size - Avail)
	out := BloxFreeSpaceResponse{
		Size:          Size / float32(GB),
		Avail:         Avail / float32(GB),
		Used:          Used / float32(GB),
		UsePercentage: Used / Size * 100.0,
	}
	return json.Marshal(out)

}
