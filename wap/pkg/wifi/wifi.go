package wifi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
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
	connectionName := strings.ReplaceAll(creds.SSID, " ", "_")

	// Delete existing connection with the same name
	DeleteConnection(ctx, connectionName)

	// Create a new connection
	if err := createConnection(ctx, connectionName, creds.SSID, creds.Password); err != nil {
		log.Errorf("failed to create Wi-Fi connection: %v", err)
		activateHotspot(ctx)
		return err
	}

	// Try connecting to the Wi-Fi network
	if err := connectToNetwork(ctx, connectionName); err != nil {
		log.Errorf("failed to connect to Wi-Fi: %v", err)
		DeleteConnection(ctx, connectionName)
		activateHotspot(ctx)
		return err
	}

	// If connected successfully, delete the hotspot
	DeleteConnection(ctx, "FxBlox")

	// Save connection properties
	if err := config.WriteProperties(map[string]interface{}{
		"ssid":         creds.SSID,
		"password":     creds.Password,
		"connection":   connectionName,
		"country_code": creds.CountryCode,
	}); err != nil {
		log.Warnf("Couldn't write the properties file: %v", err)
	}

	return nil
}

func DeleteConnection(ctx context.Context, connectionName string) {
	command := fmt.Sprintf("nmcli con delete %s", connectionName)
	_, _, err := runCommand(ctx, command)
	if err != nil {
		log.Warnf("failed to delete connection %s: %v", connectionName, err)
	}
}

func createConnection(ctx context.Context, connectionName, ssid, password string) error {
	// Create a connection
	c1 := fmt.Sprintf("nmcli con add type wifi ifname * con-name %s ssid %s", connectionName, ssid)
	c2 := fmt.Sprintf("nmcli con modify %s wifi-sec.key-mgmt wpa-psk wifi-sec.psk %s", connectionName, password)

	commands := []string{c1, c2}
	for _, command := range commands {
		_, _, err := runCommand(ctx, command)
		if err != nil {
			return err
		}
	}

	return nil
}

func getWifiConnections(ctx context.Context) ([]string, error) {
	stdout, _, err := runCommand(ctx, "nmcli --terse --fields TYPE,NAME connection")
	if err != nil {
		return nil, err
	}

	lines := bufio.NewScanner(strings.NewReader(stdout))
	var connections []string
	for lines.Scan() {
		line := lines.Text()
		parts := strings.Split(line, ":")
		if len(parts) == 2 && parts[0] == "802-11-wireless" && parts[1] != "FxBlox" {
			connections = append(connections, parts[1])
		}
	}

	return connections, nil
}

func getWiFiPassword(connectionName string) (string, error) {
	ctx := context.Background()
	command := fmt.Sprintf("nmcli -s -g 802-11-wireless-security.psk connection show %s | tr -d '\n'", connectionName)
	stdout, stderr, err := runCommand(ctx, command)
	if err != nil {
		return "", fmt.Errorf("error running command: %w; stderr: %s", err, stderr)
	}
	return strings.TrimSpace(stdout), nil
}

func connectToFirstWifi(ctx context.Context, connections []string) error {
	if len(connections) == 0 {
		return fmt.Errorf("no Wi-Fi connections available")
	}

	connection := connections[0]
	passwd, err := getWiFiPassword(connection)
	if err != nil {
		return err
	}

	// Create a temporary file
	tempFile, err := ioutil.TempFile("", "passwd-")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Write the password to the temporary file
	_, err = tempFile.WriteString(fmt.Sprintf("802-11-wireless-security.psk:%s", passwd))
	if err != nil {
		return fmt.Errorf("failed to write password to temporary file: %w", err)
	}
	err = tempFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	_, _, err = runCommand(ctx, fmt.Sprintf("nmcli connection up %s passwd-file %s", connection, tempFile.Name()))

	if err != nil {
		DeleteConnection(ctx, connection)
		createConnection(ctx, connection, connection, passwd)
		err = connectToNetwork(ctx, connection)
		return err
	}
	return nil
}

func ConnectToSavedWifi(ctx context.Context) error {
	connections, err := getWifiConnections(ctx)
	if err != nil {
		fmt.Println("Error getting Wi-Fi connections:", err)
		return err
	}

	return connectToFirstWifi(ctx, connections)
}

func connectToNetwork(ctx context.Context, connectionName string) error {
	c3 := fmt.Sprintf("nmcli con up %s", connectionName)
	_, _, err := runCommand(ctx, c3)
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Second)
	if err := CheckIfIsConnected(ctx); err != nil {
		return err

	} else {
		setAutoconnect := fmt.Sprintf("nmcli connection modify %s connection.autoconnect yes", connectionName)
		setPriority := fmt.Sprintf("nmcli connection modify %s connection.autoconnect-priority 20", connectionName)
		_, _, err := runCommand(ctx, setAutoconnect)
		if err != nil {
			return err
		}
		_, _, err = runCommand(ctx, setPriority)
		if err != nil {
			return err
		}
		return nil
	}
}

func activateHotspot(ctx context.Context) {
	c5 := "nmcli connection up FxBlox"
	_, _, err := runCommand(ctx, c5)
	if err != nil {
		log.Warnf("failed to activate hotspot: %v", err)
	}
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
