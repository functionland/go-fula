package wap

import (
	"bufio"
	"fmt"
	"runtime"
	"strings"
	"time"
)

func parseHotspot(output, os string) (supported bool, err error) {
	switch os {
	case "windows":
		supported, err = parseHotspotWindows(output)
	case "darwin":
		supported, err = parseHotspotDarwin(output)
	case "linux":
		supported, err = parseHotspotLinux(output)
	default:
		err = fmt.Errorf("%s is not a recognized OS", os)
	}
	return
}

func parseHotspotWindows(output string) (supported bool, err error) {
	supported = false
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Hosted network supported") {
			fs := strings.Fields(line)
			if len(fs) == 5 {
				if fs[4] == "Yes" {
					supported = true
				}
			}
		} else {
			continue
		}
	}
	return supported, nil
}

func parseHotspotDarwin(output string) (supported bool, err error) {
	supported = false
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Wi-Fi") {
			supported = true
		} else {
			continue
		}
	}
	return supported, nil
}

func parseHotspotLinux(output string) (supported bool, err error) {
	supported = false
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "AP") {
			supported = true
		} else {
			continue
		}
	}
	return supported, nil
}

func CheckHotspotSupported() (supported bool, err error) {
	command := ""
	os := ""
	switch runtime.GOOS {
	case "windows":
		os = "windows"
		command = "netsh wlan show drivers"
	case "darwin":
		os = "darwin"
		command = "networksetup -listallhardwareports"
	default:
		os = "linux"
		command = "iw list | grep -i \"AP\""
	}
	stdout, _, err := runCommand(TimeLimit, command)
	if err != nil {
		log.Errorw("failed to check hotspot support", "command", command, "err", err)
		return
	}
	return parseHotspot(stdout, os)
}

// startHotspot can be used to get the list of available wifis and their strength
// If forceReload is set to true it resets the network adapter to make sure it fetches the latest list, otherwise it reads from cache
// wifiInterface is the name of interface that it should look for in Linux. Default is wlan0
func startHotspot(forceReload bool) error {
	command := ""
	supported, err := CheckHotspotSupported()
	if err != nil {
		log.Errorw("failed to check hotspot support", "err", err)
		return err
	} else if !supported {
		log.Errorw("hotspot not supported")
		return fmt.Errorf("hotspot not supported")
	}
	switch runtime.GOOS {
	case "windows":
		command = "netsh wlan start hostednetwork"
		_, _, errRun := runCommand(TimeLimit, "netsh wlan set hostednetwork mode=allow ssid=FxBlox")
		if errRun != nil {
			log.Errorw("failed to set hostednetwork", "errRun", errRun)
		}
		if forceReload {
			_, _, errRun := runCommand(TimeLimit, "netsh interface set interface name=Wi-Fi admin=disabled")
			if errRun != nil {
				log.Errorw("failed to disable wifi interface", "errRun", errRun)
			}
			_, _, errRun = runCommand(TimeLimit, "netsh interface set interface name=Wi-Fi admin=enabled")
			if errRun != nil {
				log.Errorw("failed to enabled wifi interface", "errRun", errRun)
			}
			time.Sleep(3 * time.Second)
		}
	case "darwin":
		command = "/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/createbssid -n FxBlox"
	default:
		command = "nmcli dev wifi hotspot ifname wlan0 ssid FxBlox password"
	}
	_, _, err = runCommand(TimeLimit, command)
	if err != nil {
		log.Errorw("failed to start wifi hotspot", "command", command, "err", err)
		return err
	}
	return nil
}
