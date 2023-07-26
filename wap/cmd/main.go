package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	blox "github.com/functionland/go-fula/wap/cmd/blox"
	mdns "github.com/functionland/go-fula/wap/cmd/mdns"
	"github.com/functionland/go-fula/wap/pkg/config"
	"github.com/functionland/go-fula/wap/pkg/server"
	"github.com/functionland/go-fula/wap/pkg/wifi"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap/main")

// The state of the application.
var currentIsConnected int32
var isHotspotStarted = false
var currentServer io.Closer = nil
var serverMutex sync.Mutex

type VersionInfo struct {
	Version int       `json:"version"`
	Date    time.Time `json:"date"`
}

func versionStringToInt(version string) (int, error) {
	versionSlice := strings.Split(version, ".")
	versionInt := 0

	for i := 0; i < len(versionSlice); i++ {
		num, err := strconv.Atoi(versionSlice[i])
		if err != nil {
			return 0, err
		}
		versionInt = versionInt*1000 + num // 1000 is chosen as a multiplier assuming version components do not exceed 999
	}

	return versionInt, nil
}

// GetLastRebootTime reads the last boot time from /proc/stat
func GetLastRebootTime() (time.Time, error) {
	var stat syscall.Sysinfo_t
	err := syscall.Sysinfo(&stat)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot get system info: %v", err)
	}

	bootTime := time.Unix(int64(stat.Uptime), 0)

	return bootTime, nil
}

func checkAndSetVersionInfo() error {
	versionFilePath := "/internal/go_fula_version.info"
	restartNeededPath := "/internal/.restart_needed"

	// Replace "1.2.3" with strings from map
	OTA_VERSION, err := versionStringToInt(config.OTA_VERSION)
	if err != nil {
		return fmt.Errorf("error converting OTA_VERSION to int: %v", err)
	}

	RESTART_NEEDED_AFTER, err := versionStringToInt(config.RESTART_NEEDED_AFTER)
	if err != nil {
		return fmt.Errorf("error converting RESTART_NEEDED_AFTER to int: %v", err)
	}

	_, err = os.Stat(versionFilePath)

	if os.IsNotExist(err) {
		// if the version file does not exist, create it
		versionInfo := VersionInfo{
			Version: OTA_VERSION,
			Date:    time.Now(),
		}

		file, _ := json.MarshalIndent(versionInfo, "", " ")

		err = os.WriteFile(versionFilePath, file, 0644)
		if err != nil {
			return fmt.Errorf("error writing version file: %v", err)
		}

		// also create a file named /internal/.restart_neded
		_, err = os.Create(restartNeededPath)
		if err != nil {
			return fmt.Errorf("error creating restart needed file: %v", err)
		}

	} else {
		// if the version file exists
		versionFileContent, err := os.ReadFile(versionFilePath)
		if err != nil {
			return fmt.Errorf("error reading version file: %v", err)
		}

		var versionInfo VersionInfo
		err = json.Unmarshal(versionFileContent, &versionInfo)
		if err != nil {
			return fmt.Errorf("error parsing version file: %v", err)
		}

		// check if OTA_VERSION is different than version in the file
		if versionInfo.Version != OTA_VERSION {
			// if different, update the file with new OTA_VERSION and current date/time
			versionInfo.Version = OTA_VERSION
			versionInfo.Date = time.Now()

			file, _ := json.MarshalIndent(versionInfo, "", " ")

			err = os.WriteFile(versionFilePath, file, 0644)
			if err != nil {
				return fmt.Errorf("error updating version file: %v", err)
			}
		}

		// retrieve the system restart time
		restartTime, err := GetLastRebootTime()
		if err != nil {
			return fmt.Errorf("error getting last reboot time: %v", err)
		}
		log.Infof("last reboot time: ", restartTime)
		log.Infof("versionInfo.Date: ", versionInfo.Date)
		log.Infof("RESTART_NEEDED_AFTER: ", RESTART_NEEDED_AFTER)
		log.Infof("OTA_VERSION: ", OTA_VERSION)

		// compare the dates and version
		if versionInfo.Date.After(restartTime) && OTA_VERSION <= RESTART_NEEDED_AFTER {
			// create a file named /internal/.restart_needed
			_, err = os.Create(restartNeededPath)
			if err != nil {
				return fmt.Errorf("error creating restart needed file: %v", err)
			}
			log.Info("creating restart needed file")
		} else {
			// delete a file named /internal/.restart_needed
			err = os.Remove(restartNeededPath)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("error removing restart needed file: %v", err)
			}
			log.Info("removing restart needed file")
		}
	}

	return nil
}

func checkConfigExists() bool {
	// Check if "/internal/config.yaml" file exists
	if _, err := os.Stat("/internal/config.yaml"); os.IsNotExist(err) {
		log.Info("File /internal/config.yaml does not exist")
		return false
	} else {
		log.Info("File /internal/config.yaml exists")
		return true
	}
}

// handleAppState monitors the application state and starts/stops services as needed.
func handleAppState(ctx context.Context, isConnected bool, stopServer chan struct{}, mdnsServer **mdns.MDNSServer) {
	log.Info("handleAppState is called")

	currentState := atomic.LoadInt32(&currentIsConnected)
	newState := int32(0)
	if isConnected {
		newState = int32(1)
	}

	if currentState != newState {
		if *mdnsServer != nil {
			// Shutdown existing mDNS server before state change
			(*mdnsServer).Shutdown()
			*mdnsServer = nil
		}
		log.Info("starting mDNS server.")
		*mdnsServer = mdns.StartServer(ctx, 8080) // start the mDNS server
		if isConnected {
			log.Info("Wi-Fi is connected")
			configExists := checkConfigExists()
			if configExists {
				stopServer <- struct{}{} // stop the HTTP server
			} else {
				log.Info("No config file found, activating the hotspot mode.")
				if !isHotspotStarted {
					//Disconnect from external Wi-Fi before starting server as it causes the hotspot server not get the proper IP address
					/*if err := wifi.DisconnectFromExternalWifi(ctx); err != nil {
						log.Errorw("disconnect from wifi on startup", "err", err)
					}*/
					if err := wifi.StartHotspot(ctx, true); err != nil {
						log.Errorw("start hotspot on startup", "err", err)
					} else {
						isHotspotStarted = true
					}
					log.Info("Access point enabled on startup")
				} else {
					log.Info("Access point already enabled on startup")
				}
			}
		} else {
			log.Info("Wi-Fi is disconnected, activating the hotspot mode.")
			if !isHotspotStarted {
				if err := wifi.StartHotspot(ctx, true); err != nil {
					log.Errorw("start hotspot on startup", "err", err)
				} else {
					isHotspotStarted = true
				}
				log.Info("Access point enabled on startup")
			} else {
				log.Info("Access point already enabled on startup")
			}
		}
		atomic.StoreInt32(&currentIsConnected, int32(newState))
	} else {
		log.Info("handleAppState is called but no action is needed")
	}
}

func main() {
	logging.SetLogLevel("*", os.Getenv("LOG_LEVEL"))
	ctx := context.Background()
	// Call checkAndSetVersionInfo function
	err := checkAndSetVersionInfo()
	if err != nil {
		log.Errorf("Error checking and setting version info: %v", err)
	} else {
		log.Info("Successfully checked and set version info")
	}
	var mdnsServer *mdns.MDNSServer = nil

	serverCloser := make(chan io.Closer, 1)
	stopServer := make(chan struct{}, 1)
	serverReady := make(chan struct{}, 1)

	atomic.StoreInt32(&currentIsConnected, int32(2))
	isConnected := false
	log.Info("initial assignment of isConnected made it false")
	if wifi.CheckIfIsConnected(ctx, "") == nil {
		log.Info("initial test of isConnected made it true")
		isConnected = true
	}

	// Check if "/internal/config.yaml" file exists
	configExists := checkConfigExists()

	log.Info("Waiting for the system to connect to Wi-Fi")
	handleAppState(ctx, isConnected, stopServer, &mdnsServer)
	log.Infow("called handleAppState with ", isConnected)

	// Start the server in a separate goroutine
	go func() {
		mdnsRestartCh := make(chan bool, 1)
		serverMutex.Lock()
		if currentServer != nil {
			currentServer.Close()
			currentServer = nil
		}
		closer := server.Serve(blox.BloxCommandInitOnly, "", "", mdnsRestartCh)
		currentServer = closer
		serverMutex.Unlock()
		serverCloser <- closer
		serverReady <- struct{}{} // Signal that the server is ready
		for {
			select {
			case <-stopServer:
				serverMutex.Lock()
				if currentServer != nil {
					currentServer.Close()
					currentServer = nil
				}
				isHotspotStarted = false
				serverMutex.Unlock()
				return // Exit the goroutine
			case isConnected = <-mdnsRestartCh:
				log.Infow("called handleAppState in go routine1 with ", isConnected)
				handleAppState(ctx, isConnected, stopServer, &mdnsServer)
			}
		}
	}()

	// Wait for server to be ready
	<-serverReady

	if !isConnected && configExists {
		timeout2 := time.After(30 * time.Second)
		ticker2 := time.NewTicker(3 * time.Second)
		log.Info("Wi-Fi is not connected")
		err := wifi.ConnectToSavedWifi(ctx)
		if err != nil {
			log.Errorw("Connecting to saved wifi failed with error", "err", err)
		}
	loop2:
		for {
			select {
			case <-timeout2:
				log.Info("Waiting for the system to connect to saved Wi-Fi timeout passed")
				ticker2.Stop()
				break loop2
			case <-ticker2.C:
				log.Info("Waiting for the system to connect to saved Wi-Fi periodic check")
				if wifi.CheckIfIsConnected(ctx, "") == nil {
					isConnected = true
					handleAppState(ctx, isConnected, stopServer, &mdnsServer)
					ticker2.Stop()
					break loop2
				}
			}
		}
	}

	ticker3 := time.NewTicker(600 * time.Second) // Check the connection every 600 seconds

	for range ticker3.C {
		err := wifi.CheckIfIsConnected(ctx, "")
		if err == nil {
			log.Info("Connected to a wifi network")
			ticker3.Stop()
		} else {
			log.Info("Not connected to a wifi network")
			isConnected = false
			handleAppState(ctx, isConnected, stopServer, &mdnsServer)
		}
		log.Info("Access point enabled on startup")
	}
	// TODO: this code seems unused while using nmcli
	// else {
	// log.Info("Wifi already connected")
	// if err := wifi.StopHotspot(ctx); err != nil {
	// 	log.Errorw("stop hotspot on startup", "err", err)
	// }
	// log.Info("Access point disabled on startup")
	//}

	// Wait for the signal to terminate
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	// Before shutting down, make sure to set the appropriate state
	handleAppState(ctx, isConnected, stopServer, &mdnsServer)
	log.Info("Shutting down wap")

	// Close the server
	select {
	case closer := <-serverCloser:
		closer.Close()
	default:
		log.Info("Server not started, nothing to close")
	}
}
