package wap_test

import (
	"fmt"

	"github.com/functionland/go-fula/wap"
	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/wap")

// ExampleNetworkScan scans the network for available wifis and lists their name with the signal strength
// forceReload parameter refreshes the list of available wifis on Windows by disabling and enabling the adapter
// If set to false it just shows whatever is in the cache
func ExampleScan() {
	wifis, err := wap.Scan(false, "")
	if err != nil {
		log.Errorw("failed to scan the network", "err", err)
	}
	for _, w := range wifis {
		fmt.Println(w.SSID, w.RSSI)
	}

	// Unordered output:
	// Instantiated node in pool my-pool with ID: QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// Instantiated node in pool my-pool with ID: QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// Instantiated node in pool my-pool with ID: QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT peerstore contains 3 nodes:
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF peerstore contains 3 nodes:
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
	// QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA peerstore contains 3 nodes:
	// - QmPNZMi2LAhczsN2FoXXQng6YFYbSHApuP6RpKuHbBH9eF
	// - QmaUMRTBMoANXqpUbfARnXkw9esfz9LP2AjXRRr7YknDAT
	// - QmYMEnv3GUKPNr34gePX2qQmBH4YEQcuGhQHafuKuujvMA
}
