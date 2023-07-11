//go:build windows
// +build windows

package wifi

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func GetBloxFreeSpaceWindows() (BloxFreeSpaceResponse, error) {
	wd, err := os.Getwd()
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error getting current directory: %v", err)
	}

	lpFreeBytesAvailable := uint64(0)
	lpTotalNumberOfBytes := uint64(0)
	lpTotalNumberOfFreeBytes := uint64(0)

	disk := windows.StringToUTF16Ptr(wd)

	err = windows.GetDiskFreeSpaceEx(disk, &lpFreeBytesAvailable, &lpTotalNumberOfBytes, &lpTotalNumberOfFreeBytes)
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error getting disk space details: %v", err)
	}

	total := lpTotalNumberOfBytes
	free := lpFreeBytesAvailable
	used := total - free
	usedPercentage := (float32(used) / float32(total)) * 100

	return BloxFreeSpaceResponse{
		DeviceCount:    1, // assuming that the current directory is on a single device
		Size:           float32(total),
		Used:           float32(used),
		Avail:          float32(free),
		UsedPercentage: usedPercentage,
	}, nil
}
