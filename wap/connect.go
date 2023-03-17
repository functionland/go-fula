package wap

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type Credentials struct {
	SSID     string
	Password string
}

func ConnectWifi(creds Credentials) error {
	switch runtime.GOOS {
	case "linux":
		return connectLinux(creds)
	default:
		return errors.New("unsupported platform")
	}
}

func connectLinux(creds Credentials) error {
	// Check if NetworkManager is installed
	_, err := exec.LookPath("nmcli")
	if err != nil {
		return errors.New("nmcli (NetworkManager) not found")
	}

	// Create a connection
	connectionName := strings.ReplaceAll(creds.SSID, " ", "_")
	cmdCreate := exec.Command("nmcli", "con", "add", "type", "wifi", "ifname", "*", "con-name", connectionName, "ssid", creds.SSID)
	output, err := cmdCreate.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create connection: %s", string(output))
	}

	// Set the Wi-Fi password
	cmdSetPass := exec.Command("nmcli", "con", "modify", connectionName, "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", creds.Password)
	output, err = cmdSetPass.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set password: %s", string(output))
	}

	// Connect to the Wi-Fi network
	cmdConnect := exec.Command("nmcli", "con", "up", connectionName)
	output, err = cmdConnect.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect: %s", string(output))
	}

	return nil
}
