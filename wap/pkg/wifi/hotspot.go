package wifi

import (
	"bufio"
	"context"
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
		if strings.Contains(strings.ToLower(line), "device supports ap") {
			supported = true
		} else {
			continue
		}
	}
	return supported, nil
}

func CheckHotspotSupported(ctx context.Context) (supported bool, err error) {
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
		command = "iw list"
	}
	stdout, stderr, err := runCommand(ctx, command)
	if err != nil {
		log.Errorw("failed to check hotspot support", "command", command, "err", err, "stderr", stderr)
		return
	}

	return parseHotspot(stdout, os)
}

// startHotspot can be used to get the list of available wifis and their strength
// If forceReload is set to true it resets the network adapter to make sure it fetches the latest list, otherwise it reads from cache
// wifiInterface is the name of interface that it should look for in Linux.
func StartHotspot(ctx context.Context, forceReload bool) error {
	var commands []string
	var err error
	// supported, err := CheckHotspotSupported(ctx)
	// if err != nil {
	// 	log.Errorw("failed to check hotspot support", "err", err)
	// 	return err
	// } else if !supported {
	// 	log.Errorw("hotspot not supported")
	// 	return fmt.Errorf("hotspot not supported")
	// }
	switch runtime.GOOS {
	case "windows":
		commands = []string{"netsh wlan start hostednetwork"}
		_, _, errRun := runCommand(ctx, "netsh wlan set hostednetwork mode=allow ssid=FxBlox")
		if errRun != nil {
			log.Errorw("failed to set hostednetwork", "errRun", errRun)
		}
		if forceReload {
			_, _, errRun := runCommand(ctx, "netsh interface set interface name=Wi-Fi admin=disabled")
			if errRun != nil {
				log.Errorw("failed to disable wifi interface", "errRun", errRun)
			}
			_, _, errRun = runCommand(ctx, "netsh interface set interface name=Wi-Fi admin=enabled")
			if errRun != nil {
				log.Errorw("failed to enabled wifi interface", "errRun", errRun)
			}
			time.Sleep(3 * time.Second)
		}
	case "darwin":
		commands = []string{"/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/createbssid -n FxBlox"}
	default:
		commands = []string{"nmcli connection delete FxBlox", "nmcli connection add type wifi con-name FxBlox autoconnect no wifi.mode ap wifi.ssid FxBlox ipv4.method shared ipv6.method shared", "nmcli connection up FxBlox"}
	}
	for _, command := range commands {
		_, _, err = runCommand(ctx, command)
		time.Sleep(2 * time.Second)
		if err != nil {
			log.Errorw("failed to stop wifi hotspot", "command", command, "err", err)
		}
	}
	if err != nil {
		log.Errorw("failed to start wifi hotspot", "command", commands, "err", err)
		return err
	}
	return nil
}

func CheckConnection(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	stdout, _, err := runCommand(ctx, "nmcli -t -f DEVICE,STATE device status")
	if err != nil {
		return fmt.Errorf("failed to run nmcli command: %w", err)
	}

	if strings.Contains(stdout, "wlan0:connected") {
		return nil
	}

	return fmt.Errorf("Wi-Fi not connected")
}

func StopHotspot(ctx context.Context) error {
	var commands []string
	var err error
	// supported, err := CheckHotspotSupported(ctx)
	// if err != nil {
	// 	log.Errorw("failed to check hotspot support", "err", err)
	// 	return err
	// } else if !supported {
	// 	log.Errorw("hotspot not supported")
	// 	return fmt.Errorf("hotspot not supported")
	// }
	switch runtime.GOOS {
	case "windows":
		commands = []string{"netsh wlan stop hostednetwork"}
	case "darwin":
		// TODO: find the stop command
		commands = []string{"/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/createbssid -n FxBlox"}
	default:
		commands = []string{"nmcli r wifi off", "nmcli r wifi on"}
	}
	for _, command := range commands {
		_, _, err = runCommand(ctx, command)
		if err != nil {
			log.Errorw("failed to stop wifi hotspot", "command", command, "err", err)
			return err
		}
	}
	return nil
}
