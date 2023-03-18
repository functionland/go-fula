package wifi

import (
	"bufio"
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/functionland/go-fula/wap/pkg/config"
)

// Wifi is the data structure containing the basic
// elements
type Wifi struct {
	SSID string `json:"ssid"`
	RSSI int    `json:"rssi"`
}

func parse(output, os string) (wifis []Wifi, err error) {
	switch os {
	case "windows":
		wifis, err = parseWindows(output)
	case "darwin":
		wifis, err = parseDarwin(output)
	case "linux":
		wifis, err = parseLinux(output)
	default:
		err = fmt.Errorf("%s is not a recognized OS", os)
	}
	return
}

func parseWindows(output string) (wifis []Wifi, err error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	w := Wifi{}
	wifis = []Wifi{}
	for scanner.Scan() {
		line := scanner.Text()
		if w.SSID == "" {
			if strings.Contains(line, "SSID") && !strings.Contains(line, "BSSID") {
				fs := strings.Fields(line)
				if len(fs) == 4 {
					w.SSID = fs[3]
				}
			} else {
				continue
			}
		} else {
			if strings.Contains(line, "%") {
				fs := strings.Fields(line)
				if len(fs) == 3 {
					w.RSSI, err = strconv.Atoi(strings.Replace(fs[2], "%", "", 1))
					if err != nil {
						return
					}
					w.RSSI = (w.RSSI / 2) - 100
				}
			}
		}
		if w.SSID != "" && w.RSSI != 0 {
			wifis = append(wifis, w)
			w = Wifi{}
		}
	}
	return
}

func parseDarwin(output string) (wifis []Wifi, err error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	wifis = []Wifi{}
	for scanner.Scan() {
		line := scanner.Text()
		fs := strings.Fields(line)
		if len(fs) < 6 {
			continue
		}
		rssi, errParse := strconv.Atoi(fs[2])
		if errParse != nil {
			continue
		}
		if rssi > 0 {
			continue
		}
		wifis = append(wifis, Wifi{SSID: strings.ToLower(fs[1]), RSSI: rssi})
	}
	return
}

func parseLinux(output string) (wifis []Wifi, err error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	w := Wifi{}
	wifis = []Wifi{}
	for scanner.Scan() {
		line := scanner.Text()
		if w.SSID == "" {
			if strings.Contains(line, "Address") {
				fs := strings.Fields(line)
				if len(fs) == 5 {
					w.SSID = strings.ToLower(fs[4])
				}
			} else {
				continue
			}
		} else {
			if strings.Contains(line, "Signal level=") {
				level, errParse := strconv.Atoi(strings.Split(strings.Split(strings.Split(line, "level=")[1], "/")[0], " dB")[0])
				if errParse != nil {
					continue
				}
				if level > 0 {
					level = (level / 2) - 100
				}
				w.RSSI = level
			}
		}
		if w.SSID != "" && w.RSSI != 0 {
			wifis = append(wifis, w)
			w = Wifi{}
		}
	}
	return
}

// Scan can be used to get the list of available wifis and their strength
// If forceReload is set to true it resets the network adapter to make sure it fetches the latest list, otherwise it reads from cache
// wifiInterface is the name of interface that it should look for in Linux. Default is wlan0
func Scan(forceReload bool, wifiInterface ...string) (wifilist []Wifi, err error) {
	command := ""
	os := ""
	switch runtime.GOOS {
	case "windows":
		os = "windows"
		command = "netsh.exe wlan show networks mode=Bssid"
		if forceReload {
			ctx, cl1 := context.WithTimeout(context.Background(), TimeLimit)
			defer cl1()
			_, _, errRun := runCommand(ctx, "netsh interface set interface name=Wi-Fi admin=disabled")
			if errRun != nil {
				log.Errorw("failed to disable wifi interface", "errRun", errRun)
			}
			ctx, cl2 := context.WithTimeout(context.Background(), TimeLimit)
			defer cl2()
			_, _, errRun = runCommand(ctx, "netsh interface set interface name=Wi-Fi admin=enabled")
			if errRun != nil {
				log.Errorw("failed to enabled wifi interface", "errRun", errRun)
			}
			time.Sleep(3 * time.Second)
		}
	case "darwin":
		os = "darwin"
		command = "/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport -s"
	default:
		os = "linux"
		command = "sudo iwlist " + config.IFFACE_CLIENT + " scan"
		if len(wifiInterface) > 0 && len(wifiInterface[0]) > 0 {
			command = fmt.Sprintf("sudo iwlist %s scan", wifiInterface[0])
		}
	}
	ctx, cl := context.WithTimeout(context.Background(), TimeLimit)
	defer cl()
	stdout, _, err := runCommand(ctx, command)
	if err != nil {
		log.Errorw("failed to list interfaces", "command", command, "err", err)
		return
	}
	wifilist, err = parse(stdout, os)
	return
}
