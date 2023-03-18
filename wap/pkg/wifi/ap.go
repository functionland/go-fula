package wifi

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"text/template"

	"github.com/functionland/go-fula/wap/pkg/config"
)

func EnableAccessPoint(ctx context.Context) error {
	switch runtime.GOOS {
	case "linux":
		return enableAccessPoint(ctx)
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func DisableAccessPoint(ctx context.Context) error {
	switch runtime.GOOS {
	case "linux":
		return disableAccessPoint(ctx)
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func enableAccessPoint(ctx context.Context) error {
	var err error
	log.Info("Disabling access point")
	err = writeAccessPointFiles("client")
	if err != nil {
		return fmt.Errorf("write client access point files: %v", err)
	}
	return runJournalCtlCommands(ctx, enableCommands)
}

func disableAccessPoint(ctx context.Context) error {
	var err error
	log.Info("Disabling access point")
	err = writeAccessPointFiles("client")
	if err != nil {
		return fmt.Errorf("write client access point files: %v", err)
	}
	return runJournalCtlCommands(ctx, disableCommands)
}

func writeAccessPointFiles(tYpe string) error {
	f, err := os.OpenFile("/etc/dhcpcd.conf", os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("open file for write: %v", err)
	}

	tmpl := template.Must(template.ParseFiles(fmt.Sprintf("%s/templates/dhcpcd/dhcpcd.%s.tpl", workingDirectory, tYpe)))
	tmpl.Execute(f, struct {
		WifiInterface string
		IpAddr        string
	}{
		WifiInterface: config.IFFACE,
		IpAddr:        config.IPADDRESS,
	})

	f, err = os.OpenFile("/etc/dnsmasq.conf", os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("open file for write: %v", err)
	}

	tmpl = template.Must(template.ParseFiles(fmt.Sprintf("%s/templates/dnsmasq/dnsmasq.%s.tpl", workingDirectory, tYpe)))
	tmpl.Execute(f, struct {
		WifiInterface    string
		SubnetRangeStart string
		SubnetRangeEnd   string
		IpAddr           string
	}{
		WifiInterface:    config.IFFACE,
		SubnetRangeStart: config.SUBNET_RANGE_START,
		SubnetRangeEnd:   config.SUBNET_RANGE_END,
		IpAddr:           config.IPADDRESS,
	})

	f, err = os.OpenFile("/etc/hostapd/hostapd.conf", os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("open file for write: %v", err)
	}

	tmpl = template.Must(template.ParseFiles(fmt.Sprintf("%s/templates/hostapd/hostapd.%s.tpl", workingDirectory, tYpe)))
	tmpl.Execute(f, struct {
		WifiInterface string
		SSID          string
	}{
		WifiInterface: config.IFFACE,
		SSID:          config.SSID,
	})
	return nil
}
