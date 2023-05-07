package wifi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"os/exec"
	"strings"

	"github.com/libp2p/go-libp2p/core/crypto"
	"golang.org/x/crypto/ed25519"
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
	storeDir := "/uniondrive" // Set the default value

	envStoreDir := os.Getenv("FULA_BLOX_STORE_DIR")
	if envStoreDir != "" {
		storeDir = envStoreDir
	} else {
		configFile := "/internal/config.yaml"
		if _, err := os.Stat(configFile); !os.IsNotExist(err) {
			data, err := os.ReadFile(configFile)
			if err != nil {
				log.Info("failed to read config file")
			} else {
				var config Config
				err = yaml.Unmarshal(data, &config)
				if err != nil {
					log.Info("failed to unmarshal config")
				} else if config.StoreDir != "" {
					storeDir = config.StoreDir
				}
			}
		}
	}

	fs, err := os.Open(storeDir)
	if err != nil {
		return BloxFreeSpaceResponse{}, err
	}
	defer fs.Close()

	fsInfo, err := fs.Stat()
	if err != nil {
		return BloxFreeSpaceResponse{}, err
	}

	// Assuming free space equals available space
	Size := float32(fsInfo.Size())
	Avail := Size
	Used := float32(0.0)
	UsedPercentage := float32(0.0)

	out := BloxFreeSpaceResponse{
		Size:           Size / GB,
		Avail:          Avail / GB,
		Used:           Used / GB,
		UsedPercentage: UsedPercentage,
	}
	return out, nil
}
