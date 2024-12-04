package wifi

import (
	"bufio"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Wifi is the data structure containing the basic
// elements
type Wifi struct {
	ESSID string `json:"essid"`
	SSID  string `json:"ssid"`
	RSSI  int    `json:"rssi"`
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

	// Skip header line
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty output")
	}

	// Use map to track strongest signal for each SSID
	uniqueWifi := make(map[string]Wifi)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 8 {
			continue
		}

		var startIndex int
		if fields[0] == "*" {
			startIndex = 1
		}

		// Skip networks with "--" as SSID
		if fields[startIndex+1] == "--" {
			continue
		}

		ssid := fields[startIndex+1]
		signal, err := strconv.Atoi(fields[startIndex+6])
		if err != nil {
			continue
		}

		rssi := (signal / 2) - 100

		// If SSID exists, only update if new signal is stronger
		if existing, exists := uniqueWifi[ssid]; exists {
			if rssi > existing.RSSI {
				uniqueWifi[ssid] = Wifi{
					SSID:  ssid,
					ESSID: ssid,
					RSSI:  rssi,
				}
			}
		} else {
			uniqueWifi[ssid] = Wifi{
				SSID:  ssid,
				ESSID: ssid,
				RSSI:  rssi,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning output: %v", err)
	}

	// Convert map to slice
	wifis = make([]Wifi, 0, len(uniqueWifi))
	for _, wifi := range uniqueWifi {
		wifis = append(wifis, wifi)
	}

	// Sort by signal strength (RSSI), strongest first
	sort.Slice(wifis, func(i, j int) bool {
		return wifis[i].RSSI > wifis[j].RSSI
	})

	return wifis, nil
}

// Scan can be used to get the list of available wifis and their strength
// If forceReload is set to true it resets the network adapter to make sure it fetches the latest list, otherwise it reads from cache
// wifiInterface is the name of interface that it should look for in Linux.
func Scan(forceReload bool, wifiInterface ...string) (wifilist []Wifi, err error) {
	ctx, cl := context.WithTimeout(context.Background(), TimeLimit)
	defer cl()

	stdout, stderr, err := runCommand(ctx, "nmcli dev wifi list --rescan yes")
	if err != nil {
		log.Errorw("failed to run nmcli scan", "err", err, "stderr", stderr)
		return nil, err
	}

	return parse(stdout, "linux")
}
