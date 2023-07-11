//go:build darwin
// +build darwin

package wifi

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/disk"
)

func GetBloxFreeSpaceWindows() (BloxFreeSpaceResponse, error) {
	return BloxFreeSpaceResponse{}, fmt.Errorf("GetBloxFreeSpaceWindows not supported on this platform")
}
func GetBloxFreeSpaceLinux() (BloxFreeSpaceResponse, error) {
	return BloxFreeSpaceResponse{}, fmt.Errorf("GetBloxFreeSpaceWindows not supported on this platform")
}

func GetBloxFreeSpaceMac() (BloxFreeSpaceResponse, error) {
	usage, err := disk.Usage("/")
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error getting disk usage: %v", err)
	}

	usedPercentage := usage.UsedPercent

	return BloxFreeSpaceResponse{
		DeviceCount:    1, // assuming that the current directory is on a single device
		Size:           float32(usage.Total),
		Used:           float32(usage.Used),
		Avail:          float32(usage.Free),
		UsedPercentage: float32(usedPercentage),
	}, nil
}
