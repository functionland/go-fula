package blockchain

import (
	"fmt"
	"math/big"
	"os"
	"runtime"

	"github.com/shirou/gopsutil/disk"
)

type BigInt struct {
	big.Int
}

func (b BigInt) MarshalJSON() ([]byte, error) {
	return []byte(b.String()), nil
}

func (b *BigInt) UnmarshalJSON(p []byte) error {
	if string(p) == "null" {
		return nil
	}
	var z big.Int
	_, ok := z.SetString(string(p), 10)
	if !ok {
		return fmt.Errorf("not a valid big integer: %s", p)
	}
	b.Int = z
	return nil
}

func getBloxFreeSpace() (*BloxFreeSpaceResponse, error) {
	stat, err := disk.Usage(os.Getenv("FULA_BLOX_STORE_DIR"))
	if err != nil {
		return nil, fmt.Errorf("calling disk.Usage on %v platform: %v", runtime.GOOS, err)
	}
	var Size float32 = float32(stat.Total) / float32(GB)
	var Avail float32 = float32(stat.Free) / float32(GB)
	var Used float32 = float32(stat.Used) / float32(GB)
	var UsedPercentage float32 = float32(stat.UsedPercent)
	return &BloxFreeSpaceResponse{
		Size:           Size,
		Avail:          Avail,
		Used:           Used,
		UsedPercentage: UsedPercentage,
	}, nil
}
