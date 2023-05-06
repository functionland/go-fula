package wifi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/libp2p/go-libp2p-core/crypto"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v2"
)

// Assuming config.ReadProperties() and config.PROJECT_NAME are defined in your code

type BloxFreeSpaceResponse struct {
	Size           float32 `json:"size"`
	Used           float32 `json:"used"`
	Avail          float32 `json:"avail"`
	UsedPercentage float32 `json:"used_percentage"`
}

type Config struct {
	StoreDir string `yaml:"storeDir"`
	// other fields
}

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
)

func GetHardwareID() (string, error) {
	cmd := "(cat /proc/cpuinfo | grep -i 'Serial' && cat /proc/cpuinfo | grep -i 'Hardware' && cat /proc/cpuinfo | grep -i 'Revision') | sha256sum | awk '{print $1}'"
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func GeneratePrivateKeyFromSeed(seed string) (string, error) {
	hash := sha256.New()
	hash.Write([]byte(seed))
	ed25519Seed := hash.Sum(nil)

	// Generate an ed25519.PrivateKey from the seed.
	edPrivKey := ed25519.NewKeyFromSeed(ed25519Seed)

	// Convert the ed25519.PrivateKey to a libp2p crypto.PrivateKey.
	privKey, err := crypto.UnmarshalEd25519PrivateKey(edPrivKey)
	if err != nil {
		panic(err)
	}

	// Marshal the private key to bytes
	privKeyBytes, err := crypto.MarshalPrivateKey(privKey)
	if err != nil {
		panic(err)
	}

	// Encode the byte slice into a base64 string
	privKeyString := base64.StdEncoding.EncodeToString(privKeyBytes)

	return privKeyString, nil
}

func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

func GetBloxFreeSpace() (BloxFreeSpaceResponse, error) {
	log := log.With("action", "GetBloxFreeSpace")
	stat := unix.Statfs_t{}

	storeDir := os.Getenv("FULA_BLOX_STORE_DIR")

	if storeDir == "" {
		configFile := "/internal/config.yaml"
		if _, err := os.Stat(configFile); !os.IsNotExist(err) {
			data, err := ioutil.ReadFile(configFile)
			if err != nil {
				log.Error("failed to read config file")
			} else {
				var config Config
				err = yaml.Unmarshal(data, &config)
				if err != nil {
					log.Error("failed to unmarshal config")
				} else {
					storeDir = config.StoreDir
				}
			}
		}
	}

	err := unix.Statfs(storeDir, &stat)
	if err != nil {
		log.Errorw("calling unix.Statfs", "storeDir", storeDir)

		return BloxFreeSpaceResponse{}, fmt.Errorf("Error calling unix.Statfs with storeDir: %s, error: %v", storeDir, err)
	}
	var Size float32 = float32(stat.Blocks * uint64(stat.Bsize))
	var Avail float32 = float32(stat.Bfree * uint64(stat.Bsize))
	var Used float32 = float32(Size - Avail)
	var UsedPercentage float32 = 0.0
	if Size > 0.0 {
		UsedPercentage = Used / Size * 100.0
	}
	out := BloxFreeSpaceResponse{
		Size:           Size / float32(GB),
		Avail:          Avail / float32(GB),
		Used:           Used / float32(GB),
		UsedPercentage: UsedPercentage,
	}
	return out, nil

}
