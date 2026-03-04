package wifi

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
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
		return startHotspotLinux(ctx)
	}

	for _, command := range commands {
		_, stderr, errCmd := runCommand(ctx, command)
		time.Sleep(2 * time.Second)
		if errCmd != nil {
			err = errCmd
			log.Errorw("failed to start wifi hotspot", "command", command, "err", errCmd, "stderr", stderr)
		}
	}
	if err != nil {
		return err
	}
	return nil
}

// startHotspotLinux creates the FxBlox hotspot on Linux using nmcli,
// with a file-based fallback if nmcli connection add fails (e.g. due to
// nmcli/NM daemon version mismatch with mac-address-denylist property).
func startHotspotLinux(ctx context.Context) error {
	// Step 1: Delete existing FxBlox connection (cleanup, errors are non-fatal)
	_, stderr, delErr := runCommand(ctx, "nmcli connection delete FxBlox")
	if delErr != nil {
		log.Infow("no existing FxBlox connection to delete (this is normal on first run)", "stderr", stderr)
	}
	time.Sleep(2 * time.Second)

	// Step 2: Create FxBlox hotspot connection via nmcli
	addCmd := "nmcli connection add type wifi con-name FxBlox autoconnect no wifi.mode ap wifi.ssid FxBlox ipv4.method shared ipv6.method shared"
	_, stderr, err := runCommand(ctx, addCmd)
	if err != nil {
		log.Errorw("nmcli connection add failed, trying file-based fallback", "err", err, "stderr", stderr)

		// Fallback: write .nmconnection file directly
		if fallbackErr := writeHotspotConnectionFile(); fallbackErr != nil {
			return fmt.Errorf("nmcli add failed (%w) and file fallback also failed (%v)", err, fallbackErr)
		}
		_, stderr, reloadErr := runCommand(ctx, "nmcli connection reload")
		if reloadErr != nil {
			log.Errorw("failed to reload connections after file fallback", "err", reloadErr, "stderr", stderr)
			return fmt.Errorf("nmcli reload failed after file fallback: %w", reloadErr)
		}
		log.Infow("FxBlox connection created via file-based fallback")
	}
	time.Sleep(2 * time.Second)

	// Step 3: Activate FxBlox connection
	_, stderr, err = runCommand(ctx, "nmcli connection up FxBlox")
	if err != nil {
		log.Errorw("failed to activate FxBlox hotspot", "err", err, "stderr", stderr)
		return err
	}

	return nil
}

// writeHotspotConnectionFile writes a .nmconnection file for the FxBlox hotspot directly,
// bypassing nmcli. This avoids version-mismatch issues where a newer nmcli sends
// properties (like mac-address-denylist) that an older NM daemon doesn't recognize.
func writeHotspotConnectionFile() error {
	content := "[connection]\nid=FxBlox\ntype=wifi\nautoconnect=false\n\n[wifi]\nmode=ap\nssid=FxBlox\n\n[ipv4]\nmethod=shared\n\n[ipv6]\nmethod=shared\n"
	path := "/etc/NetworkManager/system-connections/FxBlox.nmconnection"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		log.Errorw("failed to write fallback .nmconnection file", "path", path, "err", err)
		return err
	}
	return nil
}

func DisconnectFromExternalWifi(ctx context.Context) error {
	var err error
	switch runtime.GOOS {
	case "windows":
		// TODO: Implement Windows-specific logic here.
	default:
		// Query NAME and TYPE together to avoid a per-connection type lookup,
		// which breaks when connection names contain spaces (e.g. "Wired connection 1")
		// because runCommand splits on whitespace.
		output, stderr, err := runCommand(ctx, "nmcli --terse --fields NAME,TYPE connection show --active")
		if err != nil {
			log.Errorw("failed to get a list of connected networks", "err", err, "stderr", stderr)
			return err
		}

		networks := strings.Split(output, "\n")

		for _, line := range networks {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Format is NAME:TYPE — TYPE (e.g. 802-11-wireless) never contains colons,
			// so split on the last colon to handle connection names that may contain colons.
			lastColon := strings.LastIndex(line, ":")
			if lastColon < 0 {
				continue
			}
			network := line[:lastColon]
			connType := line[lastColon+1:]

			if network != "FxBlox" && connType == "802-11-wireless" {
				// Use exec.CommandContext to properly handle connection names with spaces
				cmd := exec.CommandContext(ctx, "nmcli", "connection", "down", network)
				if cmdErr := cmd.Run(); cmdErr != nil {
					log.Errorw("failed to disconnect from network", "network", network, "err", cmdErr)
				}
			}
		}
	}
	return err
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
		_, stderr, errCmd := runCommand(ctx, command)
		if errCmd != nil {
			log.Errorw("failed to stop wifi hotspot", "command", command, "err", errCmd, "stderr", stderr)
			return errCmd
		}
	}
	return nil
}
