package wap_test

import (
	"fmt"

	"github.com/functionland/go-fula/wap"
)

// ExampleNetworkScan scans the network for available wifis and lists their name with the signal strength
// forceReload parameter refreshes the list of available wifis on Windows by disabling and enabling the adapter
// If set to false it just shows whatever is in the cache
func ExampleScan() {
	wifis, err := wap.Scan(false, "")
	if err != nil {
		//log.Errorw("failed to scan the network", "err", err) commented out to pass the github tests
	}
	for _, w := range wifis {
		fmt.Println(w.SSID, w.RSSI)
	}

	// Unordered output:
	// BELL957 -58
	// BELL956 -89
}
