package wifi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/libp2p/go-libp2p/core/crypto"
	"golang.org/x/crypto/ed25519"
)

// Assuming config.ReadProperties() and config.PROJECT_NAME are defined in your code

type BloxFreeSpaceRequest struct {
}
type BloxFreeSpaceResponse struct {
	DeviceCount    int     `json:"device_count"`
	Size           float32 `json:"size"`
	Used           float32 `json:"used"`
	Avail          float32 `json:"avail"`
	UsedPercentage float32 `json:"used_percentage"`
}

type DockerInfo struct {
	Image       string            `json:"image"`
	Version     string            `json:"version"`
	ID          string            `json:"id"`
	Labels      map[string]string `json:"labels"`
	Created     string            `json:"created"`
	RepoDigests []string          `json:"repo_digests"`
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

	deviceCount, errCount := strconv.Atoi(parts[0])
	size, errSize := strconv.ParseFloat(parts[1], 32)
	used, errUsed := strconv.ParseFloat(parts[2], 32)
	avail, errAvail := strconv.ParseFloat(parts[3], 32)
	usedPercentage, errUsedPercentage := strconv.ParseFloat(parts[4], 32)

	var errors []string
	if errCount != nil {
		errors = append(errors, fmt.Sprintf("error parsing count: %v", errCount))
	}
	if errSize != nil {
		errors = append(errors, fmt.Sprintf("error parsing size: %v", errSize))
	}
	if errUsed != nil {
		errors = append(errors, fmt.Sprintf("error parsing used: %v", errUsed))
	}
	if errAvail != nil {
		errors = append(errors, fmt.Sprintf("error parsing avail: %v", errAvail))
	}
	if errUsedPercentage != nil {
		errors = append(errors, fmt.Sprintf("error parsing used_percentage: %v", errUsedPercentage))
	}

	if len(errors) > 0 {
		return BloxFreeSpaceResponse{}, fmt.Errorf(strings.Join(errors, "; "))
	}

	return BloxFreeSpaceResponse{
		DeviceCount:    deviceCount,
		Size:           float32(size),
		Used:           float32(used),
		Avail:          float32(avail),
		UsedPercentage: float32(usedPercentage),
	}, nil
}

func GetContainerInfo(containerName string) (DockerInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return DockerInfo{}, err
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return DockerInfo{}, err
	}

	var containerID string
	for _, container := range containers {
		for _, name := range container.Names {
			if name == "/"+containerName {
				containerID = container.ID
				break
			}
		}
	}

	if containerID == "" {
		return DockerInfo{}, fmt.Errorf("container not found: %s", containerName)
	}

	containerJSON, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return DockerInfo{}, err
	}

	imageJSON, _, err := cli.ImageInspectWithRaw(context.Background(), containerJSON.Image)
	if err != nil {
		return DockerInfo{}, err
	}

	info := DockerInfo{
		Image:       containerJSON.Config.Image,
		Version:     containerJSON.Image,
		ID:          containerJSON.ID,
		Labels:      containerJSON.Config.Labels,
		Created:     containerJSON.Created,
		RepoDigests: imageJSON.RepoDigests,
	}

	return info, nil
}
