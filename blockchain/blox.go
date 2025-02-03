package blockchain

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/functionland/go-fula/wap/pkg/wifi"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
)

type StreamBuffer struct {
	Chunks    chan string
	closed    bool
	mu        sync.Mutex
	closeOnce sync.Once
	err       error
}

func NewStreamBuffer() *StreamBuffer {
	return &StreamBuffer{
		Chunks: make(chan string, 100), // Buffered channel to prevent blocking
	}
}

func (b *StreamBuffer) GetChunk() (string, error) {
	chunk, ok := <-b.Chunks
	if !ok {
		return "", b.err
	}
	return chunk, nil
}

func (b *StreamBuffer) AddChunk(chunk string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.closed {
		b.Chunks <- chunk
	}
}

func (b *StreamBuffer) Close(err error) {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.closed = true
		close(b.Chunks)
	})
}

func (b *StreamBuffer) IsClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

func (bl *FxBlockchain) BloxFreeSpace(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionBloxFreeSpace, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}

}

func (bl *FxBlockchain) EraseBlData(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionEraseBlData, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}

}

func (bl *FxBlockchain) WifiRemoveall(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionWifiRemoveall, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}

}

func (bl *FxBlockchain) Reboot(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionReboot, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}

}

func (bl *FxBlockchain) Partition(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionPartition, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}

}

func (bl *FxBlockchain) DeleteFulaConfig(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionDeleteFulaConfig, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}

}
func (bl *FxBlockchain) GetAccount(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionGetAccount, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)

	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
	default:
		return b, nil
	}

}

func (bl *FxBlockchain) FetchContainerLogs(ctx context.Context, to peer.ID, r wifi.FetchContainerLogsRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionFetchContainerLogs, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) ChatWithAI(ctx context.Context, to peer.ID, r wifi.ChatWithAIRequest) (*StreamBuffer, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionChatWithAI, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected response: %d; body: %s", resp.StatusCode, string(bodyBytes))
	}

	buffer := NewStreamBuffer()

	go func() {
		defer func() {
			resp.Body.Close()
			if !buffer.IsClosed() {
				buffer.Close(nil)
			}
		}()

		reader := bufio.NewReader(resp.Body)
		var accum []byte

		for {
			select {
			case <-ctx.Done():
				buffer.Close(fmt.Errorf("request canceled: %w", ctx.Err()))
				return
			default:
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err == io.EOF {
						if len(accum) > 0 {
							buffer.AddChunk(string(accum))
						}
						buffer.Close(nil)
					} else {
						buffer.Close(fmt.Errorf("error reading response: %w", err))
					}
					return
				}

				// Check for completion marker
				if bytes.HasPrefix(line, []byte("!COMPLETION!")) {
					buffer.Close(nil)
					return
				}

				accum = append(accum, line...)

				// Try to parse as JSON to verify chunk completeness
				var temp interface{}
				if json.Unmarshal(accum, &temp) == nil {
					buffer.AddChunk(string(accum))
					accum = accum[:0] // Reset accumulator
				}
			}
		}
	}()

	return buffer, nil
}

func (bl *FxBlockchain) FindBestAndTargetInLogs(ctx context.Context, to peer.ID, r wifi.FindBestAndTargetInLogsRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionFindBestAndTargetInLogs, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) GetFolderSize(ctx context.Context, to peer.ID, r wifi.GetFolderSizeRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionGetFolderSize, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) GetDatastoreSize(ctx context.Context, to peer.ID, r wifi.GetDatastoreSizeRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionGetDatastoreSize, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) DeleteWifi(ctx context.Context, to peer.ID, r wifi.DeleteWifiRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionDeleteWifi, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) DisconnectWifi(ctx context.Context, to peer.ID, r wifi.DeleteWifiRequest) ([]byte, error) {

	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionDisconnectWifi, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	switch {
	case err != nil:
		return nil, err
	case resp.StatusCode != http.StatusAccepted:
		// Attempt to parse the body as JSON.
		if jsonErr := json.Unmarshal(b, &apiError); jsonErr != nil {
			// If we can't parse the JSON, return the original body in the error.
			return nil, fmt.Errorf("unexpected response: %d %s", resp.StatusCode, string(b))
		}
		// Return the parsed error message and description.
		return nil, fmt.Errorf("unexpected response: %d %s - %s", resp.StatusCode, apiError.Message, apiError.Description)
	default:
		return b, nil
	}
}

func (bl *FxBlockchain) handleFetchContainerLogs(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionFetchContainerLogs, "from", from)

	// Parse the JSON body of the request into the DeleteWifiRequest struct
	var req wifi.FetchContainerLogsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("failed to decode request: %v", err)
		http.Error(w, "failed to decode request", http.StatusBadRequest)
		return
	}
	log.Debugw("handleFetchContainerLogs received", "req", req)

	out := wifi.FetchContainerLogsResponse{
		Status: true,
		Msg:    "",
	}
	res, err := wifi.FetchContainerLogs(ctx, req)
	if err != nil {
		out = wifi.FetchContainerLogsResponse{
			Status: false,
			Msg:    err.Error(),
		}
	} else {
		out = wifi.FetchContainerLogsResponse{
			Status: true,
			Msg:    res,
		}
	}
	log.Debugw("handleFetchContainerLogs response", "out", out)
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) handleChatWithAI(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionChatWithAI, "from", from)

	// Decode the incoming request
	var req wifi.ChatWithAIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("failed to decode request: %v", err)
		http.Error(w, "failed to decode request", http.StatusBadRequest)
		return
	}

	// Set up headers for streaming response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Fetch AI response using FetchAIResponse
	chunks, err := wifi.FetchAIResponse(ctx, req.AIModel, req.UserMessage)
	if err != nil {
		log.Error("error in fetchAIResponse: %v", err)
		http.Error(w, fmt.Sprintf("Error fetching AI response: %v", err), http.StatusInternalServerError)
		return
	}

	log.Debugw("Streaming AI response started", "ai_model", req.AIModel, "user_message", req.UserMessage)
	defer log.Debugw("Streaming AI response ended", "ai_model", req.AIModel, "user_message", req.UserMessage)

	var buffer string // Buffer to store incomplete chunks

	for {
		select {
		case <-ctx.Done(): // Handle client disconnect or cancellation
			log.Warn("client disconnected")
			return
		case chunk, ok := <-chunks:
			if !ok {
				return // Channel closed
			}

			chunk = strings.TrimSpace(chunk) // Remove leading/trailing whitespace

			if chunk == "" { // Skip empty chunks
				continue
			}

			buffer += chunk // Append chunk to buffer

			var parsedChunk struct {
				ID      string `json:"id"`
				Object  string `json:"object"`
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(buffer), &parsedChunk); err != nil {
				log.Error("failed to parse chunk: %v", err)
				continue // Wait for more data to complete the JSON object
			}

			buffer = "" // Clear buffer after successful parsing

			var newContent string
			for _, choice := range parsedChunk.Choices {
				newContent += choice.Delta.Content
			}

			newContent = strings.TrimSpace(newContent) // Remove whitespace

			response := wifi.ChatWithAIResponse{
				Status: true,
				Msg:    newContent,
			}
			log.Debugw("Streaming AI response chunk", "chunk", newContent)

			if err := json.NewEncoder(w).Encode(response); err != nil {
				log.Error("failed to write response: %v", err)
				http.Error(w, fmt.Sprintf("Error writing response: %v", err), http.StatusInternalServerError)
				return
			}
			flusher.Flush() // Flush each chunk to ensure real-time streaming
		}
	}
}

func (bl *FxBlockchain) handleFindBestAndTargetInLogs(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionFindBestAndTargetInLogs, "from", from)

	// Parse the JSON body of the request into the DeleteWifiRequest struct
	var req wifi.FindBestAndTargetInLogsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("failed to decode request: %v", err)
		http.Error(w, "failed to decode request", http.StatusBadRequest)
		return
	}
	log.Debugw("handleFindBestAndTargetInLogs received", "req", req)

	out := wifi.FindBestAndTargetInLogsResponse{
		Best:   "0",
		Target: "0",
		Err:    "",
	}
	best, target, err := wifi.FindBestAndTargetInLogs(ctx, req)
	if err != nil {
		out = wifi.FindBestAndTargetInLogsResponse{
			Best:   "0",
			Target: "0",
			Err:    err.Error(),
		}
	} else {
		out = wifi.FindBestAndTargetInLogsResponse{
			Best:   best,
			Target: target,
			Err:    "",
		}
	}
	log.Debugw("handleFindBestAndTargetInLogs response", "out", out)
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) handleGetFolderSize(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionGetFolderSize, "from", from)

	// Parse the JSON body of the request into the DeleteWifiRequest struct
	var req wifi.GetFolderSizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("failed to decode request: %v", err)
		http.Error(w, "failed to decode request", http.StatusBadRequest)
		return
	}
	log.Debugw("handleGetFolderSize received", "req", req)

	out := wifi.GetFolderSizeResponse{
		FolderPath:  "",
		SizeInBytes: "",
	}
	res, err := wifi.GetFolderSize(ctx, req)
	if err != nil {
		out = wifi.GetFolderSizeResponse{
			FolderPath:  "",
			SizeInBytes: "",
		}
	} else {
		out = res
	}
	log.Debugw("handleGetFolderSize response", "out", out)
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}

func (bl *FxBlockchain) handleGetDatastoreSize(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request) {
	log := log.With("action", actionGetDatastoreSize, "from", from)

	// Parse the JSON body of the request into the DeleteWifiRequest struct
	var req wifi.GetDatastoreSizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error("failed to decode request: %v", err)
		http.Error(w, "failed to decode request", http.StatusBadRequest)
		return
	}
	log.Debugw("handleGetDatastoreSize received", "req", req)

	out := wifi.GetDatastoreSizeResponse{}
	res, err := wifi.GetDatastoreSize(ctx, req)
	if err != nil {
		out = wifi.GetDatastoreSizeResponse{}
	} else {
		out = res
	}
	log.Debugw("handleGetDatastoreSize response", "out", out)
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.Error("failed to write response: %v", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}

}
