package wifi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/libp2p/go-libp2p/core/crypto"
	"golang.org/x/crypto/ed25519"
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
	cmd := `df -h 2>/dev/null | grep -n /storage/usb | awk '{sum2+=$2; sum3+=$3; sum4+=$4; sum5+=$5} END { print NR "," sum2 "," sum3 "," sum4 "," sum5}'`
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error executing shell command: %v", err)
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ",")

	if len(parts) != 5 {
		return BloxFreeSpaceResponse{}, fmt.Errorf("unexpected output format")
	}

	size, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error parsing size: %v", err)
	}

	used, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error parsing used: %v", err)
	}

	avail, err := strconv.ParseFloat(parts[3], 64)
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error parsing avail: %v", err)
	}

	usedPercentage, err := strconv.ParseFloat(parts[4], 64)
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error parsing used_percentage: %v", err)
	}

	return BloxFreeSpaceResponse{
		Size:           size,
		Used:           used,
		Avail:          avail,
		UsedPercentage: usedPercentage,
	}, nil
}
