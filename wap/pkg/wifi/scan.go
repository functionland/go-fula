package wifi

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"runtime"
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
	// Check which command is available
	useIw := false
	if _, err := exec.LookPath("iwlist"); err != nil {
		if _, err := exec.LookPath("iw"); err != nil {
			return nil, fmt.Errorf("neither iwlist nor iw commands found")
		}
		useIw = true
	}

	if output == "" {
		return nil, fmt.Errorf("empty output received")
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	w := Wifi{}
	wifis = []Wifi{}

	if useIw {
		// Parse iw output
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if strings.Contains(line, "BSS") {
				// If we have a complete wifi entry, append it
				if w.SSID != "" && w.RSSI != 0 {
					// Copy SSID to ESSID if ESSID is empty
					if w.ESSID == "" {
						w.ESSID = w.SSID
					}
					wifis = append(wifis, w)
					w = Wifi{} // Reset for next entry
				}
			} else if strings.Contains(line, "SSID:") {
				fs := strings.Fields(line)
				if len(fs) > 1 {
					ssid := strings.Join(fs[1:], " ")
					if ssid != "" {
						w.SSID = ssid
						w.ESSID = ssid // In iw, SSID is what we want for both fields
					}
				}
			} else if strings.Contains(line, "signal:") {
				fs := strings.Fields(line)
				if len(fs) > 1 {
					levelStr := strings.TrimSpace(strings.TrimSuffix(fs[1], " dBm"))
					level, errParse := strconv.ParseFloat(levelStr, 64)
					if errParse == nil && level < 0 { // Signal strength should be negative
						w.RSSI = int(level) // Already in dBm
					}
				}
			}
		}
		// Don't forget the last entry
		if w.SSID != "" && w.RSSI != 0 {
			if w.ESSID == "" {
				w.ESSID = w.SSID
			}
			wifis = append(wifis, w)
		}
	} else {
		// Original iwlist parsing logic
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			if w.SSID == "" {
				if strings.Contains(line, "Address") {
					fs := strings.Fields(line)
					if len(fs) >= 5 { // Changed from == 5 to >= 5 for more flexibility
						w.SSID = strings.ToLower(fs[4])
					}
				} else {
					continue
				}
			} else {
				if strings.Contains(line, "ESSID") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						essid := strings.Trim(parts[1], "\"")
						if essid != "" {
							w.ESSID = essid
						}
					}
				} else if strings.Contains(line, "Signal level=") {
					parts := strings.Split(line, "level=")
					if len(parts) > 1 {
						levelParts := strings.Split(parts[1], "/")
						if len(levelParts) > 0 {
							levelStr := strings.Split(levelParts[0], " dB")[0]
							level, errParse := strconv.Atoi(strings.TrimSpace(levelStr))
							if errParse == nil {
								if level > 0 {
									level = (level / 2) - 100
								}
								w.RSSI = level
							}
						}
					}
				}
			}
			if w.SSID != "" && w.RSSI != 0 && w.ESSID != "" {
				wifis = append(wifis, w)
				w = Wifi{}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return wifis, fmt.Errorf("error scanning output: %v", err)
	}

	return wifis, nil
}

// Scan can be used to get the list of available wifis and their strength
// If forceReload is set to true it resets the network adapter to make sure it fetches the latest list, otherwise it reads from cache
// wifiInterface is the name of interface that it should look for in Linux.
func Scan(forceReload bool, wifiInterface ...string) (wifilist []Wifi, err error) {
	var command, stdout, stderr string
	switch runtime.GOOS {
	case "windows":
		// Your windows related code
	case "darwin":
		// Your darwin related code
	default:
		// Get all available wireless network interfaces
		ctx, cl := context.WithTimeout(context.Background(), TimeLimit)
		defer cl()

		// Check which command is available
		useIw := false
		if _, err := exec.LookPath("iwlist"); err != nil {
			if _, err := exec.LookPath("iw"); err != nil {
				return nil, fmt.Errorf("neither iwlist nor iw commands found")
			}
			useIw = true
		}

		var interfaces []string
		if useIw {
			// Get interfaces using iw
			stdout, stderr, err = runCommand(ctx, "iw dev")
			if err != nil {
				log.Errorw("failed to run iw dev", "err", err, "stderr", stderr)
				return nil, err
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
			// Use existing logic for iwconfig/iwlist
			stdout, stderr, err = runCommand(ctx, "iwconfig")
			if err != nil {
				log.Errorw("failed to run iwconfig", "err", err, "stderr", stderr)
				return nil, err
			}

			// Filter output with grep-like functionality
			var filteredLines []string
			scanner := bufio.NewScanner(strings.NewReader(stdout))
			for scanner.Scan() {
				line := scanner.Text()
				if len(line) > 0 && (line[0] >= 'a' && line[0] <= 'z' || line[0] >= 'A' && line[0] <= 'Z') {
					filteredLines = append(filteredLines, line)
				}
			}

			// Run awk-like functionality to print the first field of each line
			for _, line := range filteredLines {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					interfaces = append(interfaces, fields[0])
				}
			}
		}

		// Loop over interfaces and perform scan
		for _, iface := range interfaces {
			if useIw {
				command = fmt.Sprintf("iw dev %s scan", iface)
			} else {
				command = fmt.Sprintf("iwlist %s scan", iface)
			}

			stdout, _, err = runCommand(ctx, command)
			if err == nil {
				// Break the loop when the scan command is successful
				wifilist, err = parse(stdout, "linux")
				if err == nil {
					break
				}
			}
		}
	}
	return
}
