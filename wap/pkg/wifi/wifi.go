package wifi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/template"

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
	f, err := os.OpenFile("/etc/wpa_supplicant/wpa_supplicant.conf", os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("open file for write: %v", err)
	}

	tmpl := template.Must(template.ParseFiles(workingDirectory + "/templates/wpa_supplicant.tpl"))
	tmpl.Execute(f, creds)

	err = runJournalCtlCommands(ctx, connectCommandsLinux)
	if err != nil {
		return err
	}
	if CheckIfIsConnected(ctx) != nil {
		// Retry
		err = runJournalCtlCommands(ctx, connectRetryCommandsLinux)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkIfIsConnectedLinux(ctx context.Context) error {
	// Check if iw is installed
	_, err := exec.LookPath("iw")
	if err != nil {
		log.Fatal("iw not found")
	}

	// Check the connection
	stdout, stderr, err := runCommand(ctx, fmt.Sprintf("iw %s link", config.IFFACE_CLIENT))
	if err != nil {
		return err
	}
	if strings.Contains(string(stdout), "Not connected") ||
		strings.Contains(string(stderr), "Not connected") {
		return fmt.Errorf("Wifi not connected!")
	}
	return nil
}

func disconnectLinux(ctx context.Context) error {
	// Check if wpa_cli is installed
	_, err := exec.LookPath("wpa_cli")
	if err != nil {
		return errors.New("wpa_cli not found")
	}

	// Check the connection
	_, stderr, err := runCommand(ctx, fmt.Sprintf("sudo wpa_cli -i %s DISCONNECT", config.IFFACE_CLIENT))
	if err != nil {
		return fmt.Errorf("failed to disconnect: %s", string(stderr))
	}
	return nil
}
