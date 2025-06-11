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
var (
	currentIsConnected int32
	isHotspotStarted   bool
	currentServer      io.Closer
	serverMutex        sync.Mutex
	restartServer      = make(chan struct{}, 1)
)

var versionFilePath = config.VERSION_FILE_PATH
var restartNeededPath = config.RESTART_NEEDED_PATH

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
	uptimeBytes, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot read uptime: %v", err)
	}

	uptimeString := strings.Split(string(uptimeBytes), " ")[0]
	uptimeSeconds, err := strconv.ParseFloat(uptimeString, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot parse uptime: %v", err)
	}

	bootTime := time.Now().UTC().Add(-time.Second * time.Duration(uptimeSeconds))

	return bootTime, nil
}

func checkAndSetVersionInfo() error {

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

		// also create a file named /home/commands/.command_reboot
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
			// create a file named /home/commands/.command_reboot
			_, err = os.Create(restartNeededPath)
			if err != nil {
				return fmt.Errorf("error creating restart needed file: %v", err)
			}
			log.Info("creating restart needed file")
		} else {
			// delete a file named /home/commands/.command_reboot
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
	// Check if config.yaml file exists
	if _, err := os.Stat(config.FULA_CONFIG_PATH); os.IsNotExist(err) {
		log.Infof("File %s does not exist", config.FULA_CONFIG_PATH)
		return false
	} else {
		log.Infof("File %s exists", config.FULA_CONFIG_PATH)
		return true
	}
}

func handleServerLifecycle(ctx context.Context, serverControl chan bool) {
	var server *mdns.MDNSServer
	var serverMutex sync.Mutex

	for {
		select {
		case start := <-serverControl:
			serverMutex.Lock()
			if start {
				if server == nil {
					// Start the server
					server = mdns.StartServer(ctx, 8080) // Adjust port as necessary
					log.Debug("mDNS server started")
				}
			} else {
				if server != nil {
					// Stop the server
					server.Shutdown()
					server = nil
					log.Debug("mDNS server stopped")
				}
			}
			serverMutex.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

// handleAppState monitors the application state and starts/stops services as needed.
func handleAppState(ctx context.Context, isConnected bool, stopServer chan struct{}) {
	log.Info("handleAppState is called")
	// Load the config once at the start
	mdns.LoadConfig()

	currentState := atomic.LoadInt32(&currentIsConnected)
	newState := int32(0)
	if isConnected {
		newState = int32(1)
	}

	if currentState != newState {

		if isConnected {
			log.Info("Wi-Fi is connected")
			configExists := checkConfigExists()
			if configExists {

				req := wifi.DeleteWifiRequest{
					ConnectionName: "FxBlox",
				}
				// Execute the disconnect in the background
				disconnectWifiResponse := wifi.DisconnectNamedWifi(ctx, req)
				log.Infow("Disconnect Wifi with response", "res", disconnectWifiResponse)
				if wifi.CheckIfIsConnectedWifi(ctx, "") == nil {
					stopServer <- struct{}{} // stop the HTTP server
				}
			} else {
				log.Info("No config file found, activating the hotspot mode.")
				if !isHotspotStarted {
					//Disconnect from external Wi-Fi before starting server as it causes Android in some cases not being able to connect to hotspot

					// Check if /home/V6.info exists which means fula-ota update is completed before we disconnect wifi
					if _, err := os.Stat("/home/V6.info"); err == nil {
						// File exists
						if err := wifi.DisconnectFromExternalWifi(ctx); err != nil {
							log.Errorw("disconnect from wifi on startup", "err", err)
						}
					} else {
						// create a file named /home/commands/.command_reboot
						_, err = os.Create(restartNeededPath)
						if err != nil {
							log.Errorw("error creating restart needed file: %v", err)
						}
						log.Info("creating restart needed file")
					}

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
		/*if *mdnsServer != nil {
			// Shutdown existing mDNS server before state change
			(*mdnsServer).Shutdown()
			*mdnsServer = nil
		}
		log.Info("starting mDNS server.")
		*mdnsServer = mdns.StartServer(ctx, 8080) // start the mDNS server*/
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

	// Check if config.yaml file exists
	configExists := checkConfigExists()

	log.Info("Waiting for the system to connect to Wi-Fi")
	handleAppState(ctx, isConnected, stopServer)
	log.Infow("called handleAppState with ", isConnected)

	//Start a periodic mdns server
	serverControl := make(chan bool)
	go handleServerLifecycle(ctx, serverControl)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			// Toggle server state
			serverControl <- false      // Stop the server
			time.Sleep(3 * time.Second) // Wait a bit before restarting
			serverControl <- true       // Start the server
		}
	}()
	//end of mdns server handling

	// Start the http server in a separate goroutine
	go func() {
		for {
			connectedCh := make(chan bool, 1)
			serverMutex.Lock()
			if currentServer != nil {
				currentServer.Close()
				currentServer = nil
			}
			closer := server.Serve(blox.BloxCommandInitOnly, "", "", connectedCh)
			currentServer = closer
			serverMutex.Unlock()
			serverCloser <- closer

			// Signal that the server is ready (only first time)
			select {
			case serverReady <- struct{}{}:
				// Successfully sent ready signal
			default:
				// Channel already has a value, do nothing
			}

			// Handle events until server is stopped
			running := true
			for running {
				select {
				case <-stopServer:
					serverMutex.Lock()
					if currentServer != nil {
						currentServer.Close()
						currentServer = nil
					}
					isHotspotStarted = false
					serverMutex.Unlock()
					running = false
				case <-restartServer:
					// Break out of inner loop to restart server
					serverMutex.Lock()
					if currentServer != nil {
						currentServer.Close()
						currentServer = nil
					}
					serverMutex.Unlock()
					log.Info("Restarting HTTP server...")
					running = false // Set running to false to break out of the inner loop
				case isConnected = <-connectedCh:
					log.Infow("called handleAppState in go routine with ", isConnected)
					handleAppState(ctx, isConnected, stopServer)
				}
			}

			// If we're not supposed to restart, exit the goroutine
			select {
			case <-restartServer:
				// Continue the outer loop to restart the server
				log.Info("Restarting server after receiving restart signal")
			default:
				// No restart signal, exit the goroutine
				return
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

				// Ensure hotspot is active as fallback
				if err := wifi.EnsureHotspotActive(ctx); err != nil {
					log.Errorw("Failed to ensure hotspot is active", "err", err)
				} else {
					isHotspotStarted = true
					log.Info("Hotspot activated as fallback after Wi-Fi connection failure")

					// Give the hotspot some time to initialize
					log.Info("Waiting for hotspot to initialize...")
					time.Sleep(5 * time.Second)

					// Signal to restart the server
					restartServer <- struct{}{}
				}

				break loop2
			case <-ticker2.C:
				log.Info("Waiting for the system to connect to saved Wi-Fi periodic check")
				if wifi.CheckIfIsConnected(ctx, "") == nil {
					isConnected = true
					handleAppState(ctx, isConnected, stopServer)
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
			handleAppState(ctx, isConnected, stopServer)
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
	handleAppState(ctx, isConnected, stopServer)
	log.Info("Shutting down wap")

	// Close the server
	select {
	case closer := <-serverCloser:
		closer.Close()
	default:
		log.Info("Server not started, nothing to close")
	}
}
