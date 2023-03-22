package wifi

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/functionland/go-fula/wap/pkg/config"
)

type Credentials struct {
	SSID        string
	Password    string
	CountryCode string
}

func CheckIfIsConnected(ctx context.Context) error {
	switch runtime.GOOS {
	case "linux":
		return checkIfIsConnectedLinux(ctx)
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func ConnectWifi(ctx context.Context, creds Credentials) error {
	switch runtime.GOOS {
	case "linux":
		return connectLinux(ctx, creds)
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func DisconnectWifi(ctx context.Context) error {
	switch runtime.GOOS {
	case "linux":
		return disconnectLinux(ctx)
	default:
		return errors.New("unsupported platform")
	}
}

func connectLinux(ctx context.Context, creds Credentials) error {
	// Create a connection
	connectionName := strings.ReplaceAll(creds.SSID, " ", "_")
	c0 := strings.Join([]string{"nmcli", "con", "delete", connectionName}, " ")
	c1 := strings.Join([]string{"nmcli", "con", "add", "type",
		"wifi", "ifname", "*", "con-name", connectionName, "ssid", creds.SSID}, " ")
	// Set the Wi-Fi password
	c2 := strings.Join([]string{"nmcli", "con", "modify", connectionName,
		"wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", creds.Password}, " ")
	// Connect to the Wi-Fi network
	c3 := strings.Join([]string{"nmcli", "con", "up", connectionName}, " ")
	commands := []string{c0, c1, c2, c3}
	/*err := runCommands(ctx, []string{c1, c2, c3})
	if err != nil {
		runCommand(ctx, "nmcli connection up FxBlox")
		return err
	}*/
	for _, command := range commands {
		_, _, err := runCommand(ctx, command)
		time.Sleep(3 * time.Second)
		if err != nil {
			log.Errorw("failed to finish wifi setup", "command", command, "err", err)
		}
	}
	if err := CheckIfIsConnected(ctx); err != nil {
		c4 := strings.Join([]string{"nmcli", "con", "delete", connectionName}, " ")
		c5 := strings.Join([]string{"nmcli", "connection", "up", "FxBlox"}, " ")
		commands2 := []string{c4, c5}
		for _, command := range commands2 {
			_, _, err := runCommand(ctx, command)
			time.Sleep(3 * time.Second)
			if err != nil {
				log.Errorw("failed to finish wifi setup", "command", command, "err", err)
			}
		}
		return err
	}
	if err := config.WriteProperties(map[string]interface{}{
		"ssid":         creds.SSID,
		"password":     creds.Password,
		"connection":   connectionName,
		"country_code": creds.CountryCode,
	}); err != nil {
		log.Errorf("Couldn't write the properties file: %v", err)
	}
	return nil
}

func checkIfIsConnectedLinux(ctx context.Context) error {
	// Check the connection
	stdout, stderr, err := runCommand(ctx, fmt.Sprintf("iw %s link", config.IFFACE_CLIENT))
	if err != nil {
		return err
	}
	if strings.Contains(string(stdout), "Not connected") ||
		strings.Contains(string(stderr), "Not connected") {
		return errors.New("Wifi not connected")
	}
	return nil
}

// TODO: unused, complete the c1 command
func disconnectLinux(ctx context.Context) error {
	c1 := strings.Join([]string{"nmcli", "con", "down", "type",
		"wifi"}, "")

	err := runCommands(ctx, []string{c1})
	if err != nil {
		return err
	}
	if err := CheckIfIsConnected(ctx); err != nil {
		return err

	}
	return nil
}
