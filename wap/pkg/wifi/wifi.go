package wifi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/functionland/go-fula/wap/pkg/config"
	"gopkg.in/ini.v1"
)

type WifiRemoveallRequest struct {
}
type WifiRemoveallResponse struct {
	Msg    string `json:"msg"`
	Status bool   `json:"status"`
}

type DeleteWifiRequest struct {
	ConnectionName string `json:"name"`
}
type DeleteWifiResponse struct {
	Msg    string `json:"msg"`
	Status bool   `json:"status"`
}

type DeleteFulaConfigRequest struct {
}
type DeleteFulaConfigResponse struct {
	Msg    string `json:"msg"`
	Status bool   `json:"status"`
}

type EraseBlDataRequest struct {
}
type EraseBlDataResponse struct {
	Status bool   `json:"status"`
	Msg    string `json:"msg"`
}

type RebootRequest struct {
}
type RebootResponse struct {
	Msg    string `json:"msg"`
	Status bool   `json:"status"`
}

type PartitionRequest struct {
}
type PartitionResponse struct {
	Msg    string `json:"msg"`
	Status bool   `json:"status"`
}

type Credentials struct {
	SSID        string
	Password    string
	CountryCode string
}

func CheckIfIsConnected(ctx context.Context, interfaceName string) error {
	switch runtime.GOOS {
	case "linux":
		err := CheckIfIsConnectedWifi(ctx, interfaceName)
		if err != nil {
			// If not connected via WiFi, try to ping a well-known website
			pingCmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "5", "google.com")
			if err := pingCmd.Run(); err != nil {
				return fmt.Errorf("wifi not connected and unable to reach internet: %v", err)
			}
			log.Info("Not connected to WiFi, but can access internet via other means")
		}
		return nil
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func CheckIfIsConnectedWifi(ctx context.Context, interfaceName string) error {
	switch runtime.GOOS {
	case "linux":
		return checkIfIsConnectedLinux(ctx, interfaceName)
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
	log.Info("Deleting FxBlox as connectToNetwork was successful")
	DeleteConnection(ctx, "FxBlox")

	return nil
}

func DeleteConnection(ctx context.Context, connectionName string) {
	command1 := fmt.Sprintf("nmcli con down %s", connectionName)
	stdout, stderr, err1 := runCommand(ctx, command1)
	if err1 != nil {
		log.Warnf("failed to down connection %s: %v", connectionName, err1)
	}
	log.Infof("command '%s' ran with %v , %v", command1, stdout, stderr)

	command := fmt.Sprintf("nmcli con delete %s", connectionName)
	_, _, err := runCommand(ctx, command)
	if err != nil {
		log.Warnf("failed to delete connection %s: %v", connectionName, err)
	}
}

func Reboot(ctx context.Context) RebootResponse {
	file, err := os.OpenFile(config.RESTART_NEEDED_PATH, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	res := ""
	status := true
	if err != nil {
		if os.IsExist(err) {
			res = "File already exists"
			log.Warnf(res)
		} else {
			// Other error
			res = fmt.Sprintf("Failed to open file: %s", err)
			log.Error(res)
			status = false
		}
	} else {
		res = "File created"
		log.Info(res)
		// Don't forget to close the file when you're done
		defer file.Close()
	}
	return RebootResponse{
		Msg:    res,
		Status: status,
	}
}

func Partition(ctx context.Context) PartitionResponse {
	file, err := os.OpenFile(config.PARTITION_NEEDED_PATH, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	res := ""
	status := true
	if err != nil {
		if os.IsExist(err) {
			res = "File already exists"
			log.Warnf(res)
		} else {
			// Other error
			res = fmt.Sprintf("Failed to open file: %s", err)
			log.Error(res)
			status = false
		}
	} else {
		res = "File created"
		log.Info(res)
		// Don't forget to close the file when you're done
		defer file.Close()
	}
	return PartitionResponse{
		Msg:    res,
		Status: status,
	}
}

func EraseBlData(ctx context.Context) EraseBlDataResponse {
	folderPath := "/uniondrive/chain/chains/functionyard/db/full"

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			err := os.Remove(path)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		msg := fmt.Sprintf("Error deleting files: %v", err)
		return EraseBlDataResponse{
			Msg:    msg,
			Status: false,
		}
	}

	return EraseBlDataResponse{
		Msg:    "All files deleted successfully",
		Status: true,
	}
}

func DeleteFulaConfig(ctx context.Context) DeleteFulaConfigResponse {
	configFilePath := config.FULA_CONFIG_PATH
	msg := ""
	status := true

	if _, err := os.Stat(configFilePath); err == nil {
		// The file exists, delete it
		if err := os.Remove(configFilePath); err != nil {
			msg = fmt.Sprintf("failed to delete config file: %v", err)
			status = false
		} else {
			msg = "Config file deleted successfully."
		}
	} else if os.IsNotExist(err) {
		// The file does not exist
		msg = "Config file does not exist."
	} else {
		// An error other than IsNotExist occurred
		msg = fmt.Sprintf("error checking config file: %v", err)
		status = false
	}
	return DeleteFulaConfigResponse{
		Msg:    msg,
		Status: status,
	}
}

func DeleteWifi(ctx context.Context, req DeleteWifiRequest) DeleteWifiResponse {
	errorString := ""
	command := fmt.Sprintf("nmcli con delete '%s'", strings.TrimSpace(req.ConnectionName))
	_, _, err := runCommand(ctx, command)
	if err != nil {
		log.Warnf("failed to delete connection %s: %v", req.ConnectionName, err)
		errorString = fmt.Sprintf("Failed to delete connection %s: %v", req.ConnectionName, err)
	}

	if errorString != "" {
		return DeleteWifiResponse{
			Msg:    errorString,
			Status: false,
		}
	}

	msg := "wifi connection removed successfully. "
	status := true

	return DeleteWifiResponse{
		Msg:    msg,
		Status: status,
	}
}

func DisconnectNamedWifi(ctx context.Context, req DeleteWifiRequest) DeleteWifiResponse {
	errorString := ""
	command := fmt.Sprintf("nmcli con down %s", strings.TrimSpace(req.ConnectionName))
	_, _, err := runCommand(ctx, command)
	if err != nil {
		log.Warnf("failed to disconnect connection %s: %v", req.ConnectionName, err)
		errorString = fmt.Sprintf("Failed to disconnect connection %s: %v", req.ConnectionName, err)
	}

	if errorString != "" {
		return DeleteWifiResponse{
			Msg:    errorString,
			Status: false,
		}
	}

	msg := "wifi connection disconnected successfully. "
	status := true

	return DeleteWifiResponse{
		Msg:    msg,
		Status: status,
	}
}

func WifiRemoveall(ctx context.Context) WifiRemoveallResponse {
	connections, err := getWifiConnections(ctx)
	if err != nil {
		log.Warnf("failed to get connections: %v", err)
		return WifiRemoveallResponse{
			Msg:    fmt.Sprintf("Failed to get connections: %v", err),
			Status: false,
		}
	}

	var errors []string
	for _, connectionName := range connections {
		command := fmt.Sprintf("nmcli con delete '%s'", strings.TrimSpace(connectionName))
		_, _, err := runCommand(ctx, command)
		if err != nil {
			log.Warnf("failed to delete connection %s: %v", connectionName, err)
			errors = append(errors, fmt.Sprintf("Failed to delete connection %s: %v", connectionName, err))
		}
	}

	if len(errors) > 0 {
		return WifiRemoveallResponse{
			Msg:    strings.Join(errors, "; "),
			Status: false,
		}
	}

	msg := "All wifi connections removed successfully. "
	status := true

	return WifiRemoveallResponse{
		Msg:    msg,
		Status: status,
	}
}

func createConnection(ctx context.Context, connectionName, ssid, password string) error {
	// Create a connection
	cmd1 := exec.CommandContext(ctx, "nmcli", "con", "add", "type", "wifi", "ifname", "*", "con-name", connectionName, "ssid", ssid)
	if err := runCommandDirect(cmd1); err != nil {
		return err
	}

	// Modify the connection with security settings
	cmd2 := exec.CommandContext(ctx, "nmcli", "con", "modify", connectionName, "wifi-sec.key-mgmt", "wpa-psk", "wifi-sec.psk", password)
	if err := runCommandDirect(cmd2); err != nil {
		return err
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

func readWiFiPasswordFromFile(connectionName string) (string, error) {
	cfg, err := ini.Load(fmt.Sprintf("/etc/NetworkManager/system-connections/%s.nmconnection", connectionName))
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}

	psk := cfg.Section("wifi-security").Key("psk").String()
	return psk, nil
}

func getWiFiPassword(connectionName string) (string, error) {
	ctx := context.Background()
	command := fmt.Sprintf("nmcli -s -g 802-11-wireless-security.psk connection show %s | tr -d '\n'", connectionName)
	stdout, stderr, err := runCommand(ctx, command)
	if err != nil {
		// Try reading the password from the file
		stdout, err = readWiFiPasswordFromFile(connectionName)
		if err != nil {
			return "", fmt.Errorf("error running command: %w; stderr: %s", err, stderr)
		}
	}
	return strings.TrimSpace(stdout), nil
}

func connectToFirstWifi(ctx context.Context, connections []string) error {
	if len(connections) == 0 {
		return fmt.Errorf("no Wi-Fi connections available")
	}

	connection := connections[0]
	_, _, err := runCommand(ctx, fmt.Sprintf("nmcli connection up %s", connection))

	if err != nil {
		log.Warnf("failed to connect to Wi-Fi without specifying password: %v", err)
		passwd, err := getWiFiPassword(connection)
		if err != nil {
			return err
		}

		if passwd != "" {
			// Create a temporary file
			tempFile, err := os.CreateTemp("", "passwd-")
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
				log.Warnf("failed to connect to Wi-Fi: %v", err)
				log.Info("Trying to recreate the connection profile")
				DeleteConnection(ctx, connection)
				createConnection(ctx, connection, connection, passwd)
				err = connectToNetwork(ctx, connection)
				return err
			}
		} else {
			log.Warnf("failed to read saved wifi password")
		}
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

func getDeviceOfConnection(ctx context.Context, connectionName string) (string, error) {
	c3 := fmt.Sprintf(`nmcli -g GENERAL.DEVICES con show "%s"`, connectionName)
	stdout, _, err := runCommand(ctx, c3)
	if err != nil {
		return "", err
	}

	// The device name is the output of the command.
	deviceName := strings.TrimSpace(string(stdout))
	return deviceName, nil
}

func connectToNetwork(ctx context.Context, connectionName string) error {
	c3 := exec.CommandContext(ctx, "nmcli", "con", "up", connectionName, "--ask")
	err := runCommandDirect(c3)
	if err != nil {
		return err
	}

	time.Sleep(10 * time.Second)
	interfaceName, err := getDeviceOfConnection(ctx, connectionName)
	if err != nil {
		return err
	}

	if err := CheckIfIsConnected(ctx, interfaceName); err != nil {
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

func checkIfIsConnectedLinux(ctx context.Context, interfaceName string) error {
	var interfaces []string

	if interfaceName == "" {
		// Get all available wireless network interfaces
		ctx, cl := context.WithTimeout(context.Background(), TimeLimit)
		defer cl()

		var stdout string
		var err error

		// Get interfaces using iw
		stdout, _, err = runCommand(ctx, "iw dev")
		if err != nil {
			return err
		}

		// Parse iw output for interfaces
		scanner := bufio.NewScanner(strings.NewReader(stdout))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "Interface") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					interfaces = append(interfaces, fields[1])
				}
			}
		}
	} else {
		interfaces = []string{interfaceName}
	}

	// Iterate over interfaces and check the connection
	for _, iface := range interfaces {
		var stdout, stderr string
		var err error

		stdout, stderr, err = runCommand(ctx, fmt.Sprintf("iw dev %s link", iface))

		if err != nil {
			continue // Try next interface if this one fails
		}

		// Check connection status based on the tool being used
		// iw shows "connected" when connected
		if !strings.Contains(stdout, "FxBlox") &&
			strings.Contains(stdout, "connected") &&
			!strings.Contains(stderr, "Not connected") {
			return nil
		}
	}

	// If no connected interface is found, return error
	return errors.New("WiFi not connected on any interface")
}

// TODO: unused, complete the c1 command
func disconnectLinux(ctx context.Context) error {
	c1 := strings.Join([]string{"nmcli", "con", "down", "type",
		"wifi"}, "")

	err := runCommands(ctx, []string{c1})
	if err != nil {
		return err
	}
	if err := CheckIfIsConnected(ctx, ""); err != nil {
		return err

	}
	return nil
}

// EnsureHotspotActive makes sure the hotspot is active, even if it was already started
// This is used as a fallback when Wi-Fi connection fails
func EnsureHotspotActive(ctx context.Context) error {
	// Check if FxBlox connection exists and is active
	stdout, _, err := runCommand(ctx, "nmcli -t -f NAME,STATE connection show --active")
	if err == nil && strings.Contains(stdout, "FxBlox:activated") {
		log.Info("FxBlox hotspot is already active")
		return nil
	}

	// Try to activate existing FxBlox connection
	_, _, err = runCommand(ctx, "nmcli connection up FxBlox")
	if err != nil {
		log.Warnf("Failed to activate existing FxBlox hotspot: %v", err)

		// If activation fails, try to recreate the hotspot
		return StartHotspot(ctx, true)
	}

	log.Info("FxBlox hotspot activated successfully")
	return nil
}
