package wifi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/ipfs/kubo/client/rpc"
	"github.com/libp2p/go-libp2p/core/crypto"
	ma "github.com/multiformats/go-multiaddr"
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

type GetFolderSizeRequest struct {
	FolderPath string `json:"folder_path"`
}

type GetFolderSizeResponse struct {
	FolderPath  string `json:"folder_path"`
	SizeInBytes string `json:"size"`
}

type GetDatastoreSizeRequest struct {
}

type GetDatastoreSizeResponse struct {
	RepoSize   string `json:"size"`
	StorageMax string `json:"storage_max"`
	NumObjects string `json:"count"`
	RepoPath   string `json:"folder_path"`
	Version    string `json:"version"`
}

type getDatastoreSizeResponseLocal struct {
	RepoSize   int64
	StorageMax int64
	NumObjects int
	RepoPath   string
	Version    string
}

type FetchContainerLogsResponse struct {
	Status bool   `json:"status"`
	Msg    string `json:"msg"`
}
type FetchContainerLogsRequest struct {
	ContainerName string
	TailCount     string
}

type FindBestAndTargetInLogsRequest struct {
	NodeContainerName string
	TailCount         string
}

type FindBestAndTargetInLogsResponse struct {
	Best   string `json:"best"`
	Target string `json:"target"`
	Err    string `json:"err"`
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

type SyncInfo struct {
	Best      string
	Target    string
	Finalized string
	Speed     string
}

type ChatWithAIRequest struct {
	AIModel     string `json:"ai_model"`
	UserMessage string `json:"user_message"`
}

type ChatWithAIResponse struct {
	Status bool   `json:"status"`
	Msg    string `json:"msg"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Payload struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
)

// Docker image build dates
type GetDockerImageBuildDatesRequest struct{}

type DockerImageBuildDate struct {
	ContainerName string `json:"container_name"`
	ImageName     string `json:"image_name"`
	ImageCreated  string `json:"image_created"`
	ImageDigest   string `json:"image_digest"`
}

type GetDockerImageBuildDatesResponse struct {
	Images []DockerImageBuildDate `json:"images"`
}

// Cluster info
type GetClusterInfoRequest struct{}

type GetClusterInfoResponse struct {
	ClusterPeerID   string `json:"cluster_peer_id"`
	ClusterPeerName string `json:"cluster_peer_name"`
}

func GetDockerImageBuildDates() (GetDockerImageBuildDatesResponse, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return GetDockerImageBuildDatesResponse{}, fmt.Errorf("creating Docker client: %w", err)
	}

	containers, err := cli.ContainerList(context.Background(), container.ListOptions{})
	if err != nil {
		return GetDockerImageBuildDatesResponse{}, fmt.Errorf("listing containers: %w", err)
	}

	var images []DockerImageBuildDate
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		imageInspect, _, err := cli.ImageInspectWithRaw(context.Background(), c.ImageID)
		if err != nil {
			continue
		}

		digest := ""
		if len(imageInspect.RepoDigests) > 0 {
			digest = imageInspect.RepoDigests[0]
		}

		images = append(images, DockerImageBuildDate{
			ContainerName: name,
			ImageName:     c.Image,
			ImageCreated:  imageInspect.Created,
			ImageDigest:   digest,
		})
	}

	return GetDockerImageBuildDatesResponse{Images: images}, nil
}

func GetClusterInfo() (GetClusterInfoResponse, error) {
	// Read cluster peer ID from identity.json
	clusterIdentityData, err := os.ReadFile("/uniondrive/ipfs-cluster/identity.json")
	if err != nil {
		return GetClusterInfoResponse{}, fmt.Errorf("reading cluster identity: %w", err)
	}
	var clusterIdentity struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(clusterIdentityData, &clusterIdentity); err != nil {
		return GetClusterInfoResponse{}, fmt.Errorf("parsing cluster identity: %w", err)
	}

	// Read kubo peer ID (used as CLUSTER_PEERNAME) from kubo config
	kuboConfigData, err := os.ReadFile("/internal/ipfs_data/config")
	if err != nil {
		return GetClusterInfoResponse{}, fmt.Errorf("reading kubo config: %w", err)
	}
	var kuboConfig struct {
		Identity struct {
			PeerID string `json:"PeerID"`
		} `json:"Identity"`
	}
	if err := json.Unmarshal(kuboConfigData, &kuboConfig); err != nil {
		return GetClusterInfoResponse{}, fmt.Errorf("parsing kubo config: %w", err)
	}

	return GetClusterInfoResponse{
		ClusterPeerID:   clusterIdentity.ID,
		ClusterPeerName: kuboConfig.Identity.PeerID,
	}, nil
}

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

func GetFolderSize(ctx context.Context, req GetFolderSizeRequest) (GetFolderSizeResponse, error) {
	cmd := fmt.Sprintf(`du -sb %s | cut -f1`, req.FolderPath)
	out, err := exec.CommandContext(ctx, "sh", "-c", cmd).Output()
	if err != nil {
		return GetFolderSizeResponse{}, fmt.Errorf("error executing shell command: %v", err)
	}

	sizeInBytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return GetFolderSizeResponse{}, fmt.Errorf("error parsing folder size: %v", err)
	}

	return GetFolderSizeResponse{
		FolderPath:  req.FolderPath,
		SizeInBytes: fmt.Sprint(sizeInBytes),
	}, nil
}

func GetDatastoreSize(ctx context.Context, req GetDatastoreSizeRequest) (GetDatastoreSizeResponse, error) {
	nodeMultiAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/5001")
	if err != nil {
		return GetDatastoreSizeResponse{}, fmt.Errorf("invalid multiaddress: %w", err)
	}
	node, err := rpc.NewApi(nodeMultiAddr)
	if err != nil {
		return GetDatastoreSizeResponse{}, fmt.Errorf("error creating API client: %v", err)
	}

	stat, err := node.Request("repo/stat").Send(ctx)
	if err != nil {
		return GetDatastoreSizeResponse{}, fmt.Errorf("error executing api command: %v", err)
	}

	if stat.Output != nil {
		defer stat.Output.Close() // Ensure the output is closed after reading

		data, err := io.ReadAll(stat.Output)
		if err != nil {
			return GetDatastoreSizeResponse{}, fmt.Errorf("error reading response output: %v", err)
		}
		log.Debugw("GetDatastoreSize response received", "data", string(data))
		var response getDatastoreSizeResponseLocal
		err = json.Unmarshal(data, &response)
		if err != nil {
			return GetDatastoreSizeResponse{}, fmt.Errorf("error unmarshaling response data: %v", err)
		}
		responseString := GetDatastoreSizeResponse{
			RepoSize:   fmt.Sprint(response.RepoSize),
			StorageMax: fmt.Sprint(response.StorageMax),
			NumObjects: fmt.Sprint(response.NumObjects),
			RepoPath:   response.RepoPath,
			Version:    response.Version,
		}

		// Optionally, print the response data for debugging
		fmt.Println("Response data:", response)

		return responseString, nil
	} else {
		return GetDatastoreSizeResponse{}, fmt.Errorf("no output received")
	}
}

func GetBloxFreeSpace() (BloxFreeSpaceResponse, error) {
	cmd := `df -B1 2>/dev/null | grep -nE '/storage/(usb|sd[a-z]|nvme)' | awk '{sum2+=$2; sum3+=$3; sum4+=$4; sum5+=$5} END { print NR "," sum2 "," sum3 "," sum4 "," sum5}'`
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return BloxFreeSpaceResponse{}, fmt.Errorf("error executing shell command: %v", err)
	}

	parts := strings.Split(strings.TrimSpace(string(out)), ",")

	if len(parts) != 5 {
		return BloxFreeSpaceResponse{}, fmt.Errorf("unexpected output format")
	}

	deviceCount, errCount := strconv.Atoi(parts[0])
	sizeStr, usedStr, availStr, usedPercentageStr := parts[1], parts[2], parts[3], parts[4]

	if sizeStr == "" {
		sizeStr = "0"
	}
	if usedStr == "" {
		usedStr = "0"
	}
	if availStr == "" {
		availStr = "0"
	}
	if usedPercentageStr == "" {
		usedPercentageStr = "0"
	}

	size, errSize := strconv.ParseFloat(sizeStr, 32)
	used, errUsed := strconv.ParseFloat(usedStr, 32)
	avail, errAvail := strconv.ParseFloat(availStr, 32)
	usedPercentage, errUsedPercentage := strconv.ParseFloat(usedPercentageStr, 32)

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
		return BloxFreeSpaceResponse{}, fmt.Errorf("%s", strings.Join(errors, "; "))
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

	containers, err := cli.ContainerList(context.Background(), container.ListOptions{})
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

func FetchContainerLogs(ctx context.Context, req FetchContainerLogsRequest) (string, error) {
	switch req.ContainerName {
	case "MainService":
		return fetchLogsFromFile(ctx, req.ContainerName, req.TailCount)
	default:
		return fetchLogsFromDocker(ctx, req.ContainerName, req.TailCount)
	}
}

func FetchAIResponse(ctx context.Context, aiModel string, userMessage string) (<-chan string, error) {
	url := "http://127.0.0.1:8083/rkllm_chat"

	// Prepare payload
	payload := Payload{
		Model: aiModel,
		Messages: []Message{
			{Role: "user", Content: userMessage},
		},
		Stream: true,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("received non-OK status code: %d; body: %s", resp.StatusCode, bodyBytes)
	}

	responseChannel := make(chan string)

	go func() {
		defer close(responseChannel)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Printf("Error reading response stream: %v\n", err)
				break
			}
			line = line[:len(line)-1] // Remove newline character
			responseChannel <- line   // Send each chunk of data as it arrives
		}
	}()

	return responseChannel, nil
}

func fetchLogsFromDocker(ctx context.Context, containerName string, tailCount string) (string, error) {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation(), client.WithHost("unix:///var/run/docker.sock"))
	if err != nil {
		return "", fmt.Errorf("creating Docker client: %w", err)
	}

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tailCount, // Adjust the number of lines as needed
	}
	logs, err := cli.ContainerLogs(ctx, containerName, options)
	if err != nil {
		return "", fmt.Errorf("getting container logs: %w", err)
	}
	defer logs.Close()

	logBytes, err := io.ReadAll(logs)
	if err != nil {
		return "", fmt.Errorf("reading container logs: %w", err)
	}

	return string(logBytes), nil
}

// Usage example:
// best, target, err := findBestAndTargetInLogs(context.Background(), "fula_node")
//
//	if err != nil {
//	    log.Fatalf("Error: %s", err)
//	}
//
// log.Printf("Best: %s, Target: %s", best, target)
func FindBestAndTargetInLogs(ctx context.Context, req FindBestAndTargetInLogsRequest) (string, string, error) {
	logs, err := fetchLogsFromDocker(ctx, req.NodeContainerName, req.TailCount)
	if err != nil {
		return "0", "0", err
	}

	// Compile the regular expressions once before the loop
	bestRegex := regexp.MustCompile(`best: #(\d+)`)
	targetRegex := regexp.MustCompile(`target=#(\d+)`)

	// Split logs into lines and iterate in reverse
	lines := strings.Split(logs, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.Contains(line, "Syncing") && strings.Contains(line, "target") && strings.Contains(line, "best") {
			bestMatches := bestRegex.FindStringSubmatch(line)
			targetMatches := targetRegex.FindStringSubmatch(line)

			if len(bestMatches) > 1 && len(targetMatches) > 1 {
				// Return the values found for best and target from the first matching line
				return bestMatches[1], targetMatches[1], nil
			}
		}
	}

	// If we get here, it means no matching line was found
	return "0", "0", fmt.Errorf("no logs containing 'Syncing', 'target', and 'best' were found")
}

func fetchLogsFromFile(ctx context.Context, name string, tailCount string) (string, error) {
	fileName := ""
	switch name {
	case "MainService":
		fileName = "/home/fula.sh.log"
	default:
		fileName = ""
	}

	if fileName != "" {
		cmd := exec.Command("tail", "-n", tailCount, fileName)
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr // Capture stderr for logging errors.
		err := cmd.Run()
		if err != nil {
			return "", fmt.Errorf("reading system logs: %w | stderr: %s", err, stderr.String())
		}
		return out.String(), nil
	}
	return "", fmt.Errorf("name is undefined")
}
