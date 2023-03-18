package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

var (
	PROJECT_NAME           string
	CUSTOM_PROPERTIES_FILE string
	API_URL                string
	API_PORT               string
	IFFACE                 string
	IFFACE_CLIENT          string
	SSID                   string
	IPADDRESS              string
	SUBNET_RANGE_START     string
	SUBNET_RANGE_END       string
	NETMASK                string
	FORCE_ACCESSPOINT      string
	COUNTRY                string
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
		fmt.Printf("Error loading .env file\n")
	}

	PROJECT_NAME = getEnv("PROJECT_NAME", "Box Firmware")
	CUSTOM_PROPERTIES_FILE =
		getEnv("CUSTOM_PROPERTIES_FILE", "/var/box_props.json")

	API_URL = getEnv("API_URL", "http://localhost:3500")
	API_PORT = getEnv("API_PORT", "3500")

	IFFACE = getEnv("IFFACE", "uap0")
	IFFACE_CLIENT = getEnv("IFFACE_CLIENT", "wlan0")

	SSID = getEnv("SSID", "Box")
	IPADDRESS = getEnv("IPADDRESS", "192.168.88.1")
	SUBNET_RANGE_START = getEnv("SUBNET_RANGE_START", "192.168.88.100")
	SUBNET_RANGE_END =
		getEnv("SUBNET_RANGE_END", "192.168.88.200")
	NETMASK = getEnv("NETMASK", "255.255.255.0")
	FORCE_ACCESSPOINT = getEnv("FORCE_ACCESSPOINT", "1")
	COUNTRY = getEnv("COUNTRY", "GB")
}
