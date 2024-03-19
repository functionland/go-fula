package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

var (
	PROJECT_NAME           string
	PROJECT_ROOT           string
	CUSTOM_PROPERTIES_FILE string
	API_URL                string
	API_PORT               string
	IFFACE                 string
	IFFACE_CLIENT          string
	IPADDRESS              string
	SUBNET_RANGE_START     string
	SUBNET_RANGE_END       string
	NETMASK                string
	FORCE_ACCESSPOINT      string
	COUNTRY                string
	BLOX_COMMAND           string
	OTA_VERSION            string
	HOTSPOT_SSID           string
	RESTART_NEEDED_AFTER   string
	VERSION_FILE_PATH      string
	RESTART_NEEDED_PATH    string
	PARTITION_NEEDED_PATH  string
	FULA_CONFIG_PATH       string
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func init() {
	err := godotenv.Load()
	if err != nil {
		fmt.Printf("Error loading .env file %s\n", err)
	}

	PROJECT_NAME = getEnv("PROJECT_NAME", "Box Firmware")
	CUSTOM_PROPERTIES_FILE =
		getEnv("CUSTOM_PROPERTIES_FILE", "/internal/box_props.json")

	API_URL = getEnv("API_URL", "http://localhost:3500")
	API_PORT = getEnv("API_PORT", "3500")

	IFFACE = getEnv("IFFACE", "uap0")
	IFFACE_CLIENT = getEnv("IFFACE_CLIENT", "wlan0")

	IPADDRESS = getEnv("IPADDRESS", "10.42.0.1")
	SUBNET_RANGE_START = getEnv("SUBNET_RANGE_START", "192.168.88.100")
	SUBNET_RANGE_END =
		getEnv("SUBNET_RANGE_END", "192.168.88.200")
	NETMASK = getEnv("NETMASK", "255.255.255.0")
	FORCE_ACCESSPOINT = getEnv("FORCE_ACCESSPOINT", "1")
	COUNTRY = getEnv("COUNTRY", "GB")
	PROJECT_ROOT = getEnv("PROJECT_ROOT", "../..")
	FULA_CONFIG_PATH = getEnv("FULA_CONFIG_PATH", "/internal/config.yaml")
	BLOX_COMMAND = fmt.Sprintf(getEnv("BLOX_COMMAND", "/app --authorizer %%s --identity %%s --initOnly --config %s --storeDir /uniondrive --disableResourceManger true --maxCIDPushRate 100 --logLevel info"), FULA_CONFIG_PATH)
	OTA_VERSION = getEnv("OTA_VERSION", "6")
	HOTSPOT_SSID = getEnv("HOTSPOT_SSID", "FxBlox")
	RESTART_NEEDED_AFTER = getEnv("RESTART_NEEDED_AFTER", "6")
	VERSION_FILE_PATH = getEnv("VERSION_FILE_PATH", "/home/go_fula_version.info")
	RESTART_NEEDED_PATH = getEnv("RESTART_NEEDED_PATH", "/home/commands/.command_reboot")
	PARTITION_NEEDED_PATH = getEnv("PARTITION_NEEDED_PATH", "/home/commands/.command_partition")

}
