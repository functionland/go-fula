//go:build linux
// +build linux

package wifi

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func GetBloxFreeSpaceWindows() (BloxFreeSpaceResponse, error) {
	return BloxFreeSpaceResponse{}, fmt.Errorf("GetBloxFreeSpaceWindows not supported on this platform")
}
func GetBloxFreeSpaceMac() (BloxFreeSpaceResponse, error) {
	return BloxFreeSpaceResponse{}, fmt.Errorf("GetBloxFreeSpaceWindows not supported on this platform")
}

func GetBloxFreeSpaceLinux() (BloxFreeSpaceResponse, error) {
	cmd := `df -B1 2>/dev/null | grep -nE '/storage/(usb|sd[a-z]|nvme)' | awk '{sum2+=$2; sum3+=$3; sum4+=$4; sum5+=$5} END { print NR "," sum2 "," sum3 "," sum4 "," sum5}'`
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error executing shell command: %v", err)
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ",")

	if len(parts) != 5 {
		return BloxFreeSpaceResponse{}, fmt.Errorf("unexpected output format")
	}

	deviceCount, errCount := strconv.Atoi(parts[0])
	size, errSize := strconv.ParseFloat(parts[1], 32)
	used, errUsed := strconv.ParseFloat(parts[2], 32)
	avail, errAvail := strconv.ParseFloat(parts[3], 32)
	usedPercentage, errUsedPercentage := strconv.ParseFloat(parts[4], 32)

	var errors []string
	if errCount != nil {
		errors = append(errors, fmt.Sprintf("error parsing count: %v", errCount))
	}
	if errSize != nil {
		errors = append(errors, fmt.Sprintf("error parsing size: %v", errSize))
	}
	if errUsed != nil {
		errors = append(errors, fmt.Sprintf("error parsing used: %v", errUsed))
	}
	if errAvail != nil {
		errors = append(errors, fmt.Sprintf("error parsing avail: %v", errAvail))
	}
	if errUsedPercentage != nil {
		errors = append(errors, fmt.Sprintf("error parsing used_percentage: %v", errUsedPercentage))
	}

	if len(errors) > 0 {
		return BloxFreeSpaceResponse{}, fmt.Errorf(strings.Join(errors, "; "))
	}

	return BloxFreeSpaceResponse{
		DeviceCount:    deviceCount,
		Size:           float32(size),
		Used:           float32(used),
		Avail:          float32(avail),
		UsedPercentage: float32(usedPercentage),
	}, nil
}
