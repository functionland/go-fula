package blockchain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const pluginInternalDir = "/internal/plugins"
const activePluginsFile = pluginInternalDir + "/active-plugins.txt"
const updatePluginsFile = pluginInternalDir + "/update-plugins.txt"

type PluginInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Usage       struct {
		Storage   string `json:"storage"`
		Compute   string `json:"compute"`
		Bandwidth string `json:"bandwidth"`
		RAM       string `json:"ram"`
		GPU       string `json:"gpu"`
	} `json:"usage"`
	Rewards   []map[string]string `json:"rewards"`
	Socials   []map[string]string `json:"socials"`
	Approved  bool                `json:"approved"`
	Installed bool                `json:"installed"`
}

type PluginParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (bl *FxBlockchain) listPluginsImpl(ctx context.Context) ([]byte, error) {
	log.Debug("listPluginImpl started")

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return json.Marshal(map[string]interface{}{
			"msg":    "Operation cancelled",
			"status": false,
		})
	default:
	}
	// Fetch the list of plugins
	resp, err := http.Get("https://raw.githubusercontent.com/functionland/fula-ota/refs/heads/main/docker/fxsupport/linux/plugins/info.json")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plugin list: %w", err)
	}
	defer resp.Body.Close()

	var pluginList []struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pluginList); err != nil {
		return nil, fmt.Errorf("failed to decode plugin list: %w", err)
	}

	// Read active plugins
	activePlugins, err := bl.readActivePlugins()
	if err != nil {
		return nil, fmt.Errorf("failed to read active plugins: %w", err)
	}

	var detailedPlugins []PluginInfo
	for _, plugin := range pluginList {
		// Fetch detailed info for each plugin
		detailResp, err := http.Get(fmt.Sprintf("https://raw.githubusercontent.com/functionland/fula-ota/refs/heads/main/docker/fxsupport/linux/plugins/%s/info.json", plugin.Name))
		if err != nil {
			return nil, fmt.Errorf("failed to fetch details for plugin %s: %w", plugin.Name, err)
		}
		defer detailResp.Body.Close()

		var pluginInfo PluginInfo
		if err := json.NewDecoder(detailResp.Body).Decode(&pluginInfo); err != nil {
			return nil, fmt.Errorf("failed to decode details for plugin %s: %w", plugin.Name, err)
		}

		// Check if the plugin is installed
		pluginInfo.Installed = contains(activePlugins, plugin.Name)

		detailedPlugins = append(detailedPlugins, pluginInfo)
	}

	return json.Marshal(detailedPlugins)
}

func (bl *FxBlockchain) listActivePluginsImpl(ctx context.Context) ([]byte, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return json.Marshal(map[string]interface{}{
			"msg":    "Operation cancelled",
			"status": false,
		})
	default:
	}

	activePlugins, err := bl.readActivePlugins()
	if err != nil {
		return json.Marshal(map[string]interface{}{
			"msg":    fmt.Sprintf("Failed to read active plugins: %v", err),
			"status": false,
		})
	}

	// Create a slice of plugin names
	var pluginNames []string
	for _, plugin := range activePlugins {
		if plugin != "" {
			pluginNames = append(pluginNames, plugin)
		}
	}

	// Create the response
	response := map[string]interface{}{
		"msg":    pluginNames,
		"status": true,
	}

	// Marshal the response to JSON
	result, err := json.Marshal(response)
	if err != nil {
		return json.Marshal(map[string]interface{}{
			"msg":    fmt.Sprintf("Failed to marshal active plugins: %v", err),
			"status": false,
		})
	}

	return result, nil
}

func (bl *FxBlockchain) installPluginImpl(ctx context.Context, pluginName string, paramsString string) ([]byte, error) {
	log.Debug("installPluginImpl started")
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return json.Marshal(map[string]interface{}{
			"msg":    "Operation cancelled",
			"status": false,
		})
	default:
	}

	// Read existing plugins
	plugins, err := bl.readActivePlugins()
	if err != nil {
		return json.Marshal(map[string]interface{}{
			"msg":    fmt.Sprintf("Failed to read active plugins: %v", err),
			"status": false,
		})
	}
	log.Debugw("installPluginImpl plugins:", "plugins", plugins, "pluginName", pluginName)

	// Check if plugin already exists
	for _, p := range plugins {
		log.Debugw("installPluginImpl plugins for loop:", "p", p, "pluginName", pluginName)
		if p == pluginName {
			bl.setPluginStatus(pluginName, "install plugin request: Processed")
			return json.Marshal(map[string]interface{}{
				"msg":    "Plugin already installed",
				"status": true,
			})
		}
	}

	// Process parameters param1====value1,,,,param2====value2
	if paramsString != "" {
		params := strings.Split(paramsString, ",,,,")
		for _, param := range params {
			parts := strings.SplitN(param, "====", 2)
			if len(parts) != 2 {
				return json.Marshal(map[string]interface{}{
					"msg":    fmt.Sprintf("Invalid parameter format: %s", param),
					"status": false,
				})
			}
			name := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			filePath := fmt.Sprintf("/%s/%s/%s.txt", pluginInternalDir, pluginName, name)
			dirPath := fmt.Sprintf("/%s/%s", pluginInternalDir, pluginName)

			// Create directory if it doesn't exist
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return json.Marshal(map[string]interface{}{
					"msg":    fmt.Sprintf("Failed to create directory for plugin %s: %v", pluginName, err),
					"status": false,
				})
			}

			// Write parameter value to file
			if err := os.WriteFile(filePath, []byte(value), 0644); err != nil {
				return json.Marshal(map[string]interface{}{
					"msg":    fmt.Sprintf("Failed to write parameter file for plugin %s: %v", pluginName, err),
					"status": false,
				})
			}
		}
	}

	// Append new plugin
	plugins = append(plugins, pluginName)

	// Write updated list back to file
	if err := bl.writeActivePlugins(plugins); err != nil {
		return json.Marshal(map[string]interface{}{
			"msg":    fmt.Sprintf("Failed to write active plugins: %v", err),
			"status": false,
		})
	}

	bl.setPluginStatus(pluginName, "install plugin request: Processed")
	return json.Marshal(map[string]interface{}{
		"msg":    "Plugin installed successfully",
		"status": true,
	})
}

func (bl *FxBlockchain) uninstallPluginImpl(ctx context.Context, pluginName string) ([]byte, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return json.Marshal(map[string]interface{}{
			"msg":    "Operation cancelled",
			"status": false,
		})
	default:
	}

	// Read existing plugins
	plugins, err := bl.readActivePlugins()
	if err != nil {
		return json.Marshal(map[string]interface{}{
			"msg":    fmt.Sprintf("Failed to read active plugins: %v", err),
			"status": false,
		})
	}

	// Remove the plugin if it exists
	var newPlugins []string
	found := false
	for _, p := range plugins {
		if p != pluginName {
			newPlugins = append(newPlugins, p)
		} else {
			found = true
		}
	}

	if !found {
		return json.Marshal(map[string]interface{}{
			"msg":    "Plugin not found",
			"status": false,
		})
	}

	// Write updated list back to file
	if err := bl.writeActivePlugins(newPlugins); err != nil {
		return json.Marshal(map[string]interface{}{
			"msg":    fmt.Sprintf("Failed to write active plugins: %v", err),
			"status": false,
		})
	}

	return json.Marshal(map[string]interface{}{
		"msg":    "Plugin uninstalled successfully",
		"status": true,
	})
}

func (bl *FxBlockchain) ListPlugins(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionListPlugins, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (bl *FxBlockchain) InstallPlugin(ctx context.Context, to peer.ID, pluginName string, paramsString string) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(map[string]string{"plugin_name": pluginName, "params": paramsString}); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionInstallPlugin, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (bl *FxBlockchain) UninstallPlugin(ctx context.Context, to peer.ID, pluginName string) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(map[string]string{"plugin_name": pluginName}); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionUninstallPlugin, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (bl *FxBlockchain) ListActivePlugins(ctx context.Context, to peer.ID) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+to.String()+".invalid/"+actionListActivePlugins, nil)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (bl *FxBlockchain) HandleListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins, err := bl.listPluginsImpl(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plugins)
}

func (bl *FxBlockchain) HandleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PluginName string `json:"plugin_name"`
		Params     string `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	result, err := bl.installPluginImpl(r.Context(), req.PluginName, req.Params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func (bl *FxBlockchain) HandleUninstallPlugin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PluginName string `json:"plugin_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	result, err := bl.uninstallPluginImpl(r.Context(), req.PluginName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func (bl *FxBlockchain) HandleListActivePlugins(w http.ResponseWriter, r *http.Request) {
	activePlugins, err := bl.listActivePluginsImpl(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activePlugins)
}

func (bl *FxBlockchain) readActivePlugins() ([]string, error) {
	log.Debug("readActivePlugins started")
	content, err := os.ReadFile(activePluginsFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Debug("readActivePlugins file does not exist")
			return []string{}, nil
		}
		log.Debugw("readActivePlugins error", "error", err)
		return nil, fmt.Errorf("failed to read active plugins file: %w", err)
	}
	log.Debugw("readActivePlugins finished", "content", string(content))
	return strings.Split(strings.TrimSpace(string(content)), "\n"), nil
}

func (bl *FxBlockchain) writeActivePlugins(plugins []string) error {
	content := strings.Join(plugins, "\n")
	if err := os.WriteFile(activePluginsFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write active plugins file: %w", err)
	}
	return nil
}

func (bl *FxBlockchain) ShowPluginStatus(ctx context.Context, pluginName string, lines int) ([]byte, error) {
	status, err := bl.showPluginStatusImpl(ctx, pluginName, lines)
	if err != nil {
		return nil, err
	}
	return json.Marshal(status)
}

func (bl *FxBlockchain) showPluginStatusImpl(ctx context.Context, pluginName string, lines int) ([]string, error) {
	args := []string{"docker", "logs"}

	if lines > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", lines))
	}

	args = append(args, pluginName)

	cmd := exec.CommandContext(ctx, "sudo", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to get logs for plugin %s: %w\nStderr: %s", pluginName, err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to execute docker logs for plugin %s: %w", pluginName, err)
	}

	rawLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var formattedLines []string
	for _, line := range rawLines {
		select {
		case <-ctx.Done():
			return formattedLines, ctx.Err()
		default:
			formattedLines = append(formattedLines, line)
		}
	}

	if len(formattedLines) == 0 {
		return nil, fmt.Errorf("no log output for plugin %s", pluginName)
	}

	return formattedLines, nil
}

func (bl *FxBlockchain) GetInstallOutput(ctx context.Context, to peer.ID, pluginName string, paramsString string) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(map[string]string{"plugin_name": pluginName, "params": paramsString}); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionGetInstallOutput, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (bl *FxBlockchain) GetInstallStatus(ctx context.Context, to peer.ID, pluginName string) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(map[string]string{"plugin_name": pluginName}); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionGetInstallStatus, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (bl *FxBlockchain) getInstallOutputImpl(ctx context.Context, pluginName string, paramsString string) ([]byte, error) {
	log.Debug("getInstallOutputImpl started")

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return json.Marshal(map[string]interface{}{
			"msg":    "Operation cancelled",
			"status": false,
		})
	default:
	}

	output := make(map[string]string)
	params := strings.Split(paramsString, ",,,,")

	for _, name := range params {
		filePath := fmt.Sprintf("/%s/%s/%s.txt", pluginInternalDir, pluginName, name)
		content, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				output[name] = ""
			} else {
				return json.Marshal(map[string]interface{}{
					"msg":    fmt.Errorf("error reading file for %s: %w", name, err),
					"status": false,
				})
			}
		} else {
			output[name] = strings.TrimSpace(string(content))
		}
	}

	return json.Marshal(map[string]interface{}{
		"msg":    output,
		"status": true,
	})
}

func (bl *FxBlockchain) getInstallStatusImpl(ctx context.Context, pluginName string) ([]byte, error) {
	log.Debug("getInstallStatusImpl started")

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return json.Marshal(map[string]interface{}{
			"msg":    "Operation cancelled",
			"status": false,
		})
	default:
	}

	filePath := fmt.Sprintf("/%s/%s/status.txt", pluginInternalDir, pluginName)
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return json.Marshal(map[string]interface{}{
				"status": true,
				"msg":    "No Status",
			})
		}
		return json.Marshal(map[string]interface{}{
			"status": false,
			"msg":    fmt.Errorf("error reading status file for %s: %w", pluginName, err),
		})
	}

	return json.Marshal(map[string]interface{}{
		"status": true,
		"msg":    strings.TrimSpace(string(content)),
	})
}

func (bl *FxBlockchain) HandleGetInstallOutput(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PluginName string `json:"plugin_name"`
		Params     string `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	result, err := bl.getInstallOutputImpl(r.Context(), req.PluginName, req.Params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func (bl *FxBlockchain) HandleGetInstallStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PluginName string `json:"plugin_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	result, err := bl.getInstallStatusImpl(r.Context(), req.PluginName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func (bl *FxBlockchain) setPluginStatus(pluginName, status string) error {
	// Trim any leading or trailing whitespace from the status
	status = strings.TrimSpace(status)

	// Construct the file path
	filePath := fmt.Sprintf("/%s/%s/status.txt", pluginInternalDir, pluginName)

	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for plugin status: %w", err)
	}

	// Write the status to the file
	if err := os.WriteFile(filePath, []byte(status), 0644); err != nil {
		return fmt.Errorf("failed to write status file for plugin %s: %w", pluginName, err)
	}

	return nil
}

func (bl *FxBlockchain) updatePluginImpl(ctx context.Context, pluginName string) ([]byte, error) {
	log.Debug("updatePluginImpl started")

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return json.Marshal(map[string]interface{}{
			"msg":    "Operation cancelled",
			"status": false,
		})
	default:
	}

	// Read existing update plugins
	updatePlugins, err := bl.readUpdatePlugins()
	if err != nil {
		return json.Marshal(map[string]interface{}{
			"msg":    fmt.Sprintf("Failed to read update plugins: %v", err),
			"status": false,
		})
	}
	log.Debugw("updatePluginImpl plugins:", "updatePlugins", updatePlugins, "pluginName", pluginName)

	// Check if plugin already exists in update list
	for _, p := range updatePlugins {
		if p == pluginName {
			return json.Marshal(map[string]interface{}{
				"msg":    "Plugin already in update queue",
				"status": true,
			})
		}
	}

	// Append new plugin to update list
	updatePlugins = append(updatePlugins, pluginName)

	// Write updated list back to file
	if err := bl.writeUpdatePlugins(updatePlugins); err != nil {
		return json.Marshal(map[string]interface{}{
			"msg":    fmt.Sprintf("Failed to write update plugins: %v", err),
			"status": false,
		})
	}

	return json.Marshal(map[string]interface{}{
		"msg":    "Plugin update queued successfully",
		"status": true,
	})
}

func (bl *FxBlockchain) HandleUpdatePlugin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PluginName string `json:"plugin_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	result, err := bl.updatePluginImpl(r.Context(), req.PluginName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}

func (bl *FxBlockchain) UpdatePlugin(ctx context.Context, to peer.ID, pluginName string) ([]byte, error) {
	if bl.allowTransientConnection {
		ctx = network.WithUseTransient(ctx, "fx.blockchain")
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(map[string]string{"plugin_name": pluginName}); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+to.String()+".invalid/"+actionUpdatePlugin, &buf)
	if err != nil {
		return nil, err
	}
	resp, err := bl.doP2PRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (bl *FxBlockchain) readUpdatePlugins() ([]string, error) {
	content, err := os.ReadFile(updatePluginsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read update plugins file: %w", err)
	}
	return strings.Split(strings.TrimSpace(string(content)), "\n"), nil
}

func (bl *FxBlockchain) writeUpdatePlugins(plugins []string) error {
	content := strings.Join(plugins, "\n")
	if err := os.WriteFile(updatePluginsFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write update plugins file: %w", err)
	}
	return nil
}

func (bl *FxBlockchain) handlePluginAction(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request, action string) {
	log := log.With("action", action, "from", from)
	log.Debug("started handlePluginAction")
	var req struct {
		PluginName string `json:"plugin_name,omitempty"`
		Lines      int    `json:"lines,omitempty"`
		Params     string `json:"params,omitempty"`
	}

	// Check if the request body is not empty before attempting to decode
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			if err != io.EOF {
				log.Errorw("An Error occurred while decoding request body", "error", err)
				http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
				return
			}
		}
	}

	if req.PluginName == "" && (action != actionListPlugins && action != actionListActivePlugins) {
		log.Error("Plugin name is required")
		http.Error(w, "Plugin name is required", http.StatusBadRequest)
		return
	}

	var result []byte
	var err error

	switch action {
	case actionListPlugins:
		log.Debug("handlePluginAction: calling method actionListPlugins")
		result, err = bl.listPluginsImpl(ctx)
	case actionListActivePlugins:
		log.Debug("handlePluginAction: calling method actionListActivePlugins")
		result, err = bl.listActivePluginsImpl(ctx)
	case actionInstallPlugin:
		log.Debugw("handlePluginAction: calling method actionInstallPlugin", "PluginName", req.PluginName, "params", req.Params)
		result, err = bl.installPluginImpl(ctx, req.PluginName, req.Params)
	case actionUninstallPlugin:
		log.Debug("handlePluginAction: calling method actionUninstallPlugin")
		result, err = bl.uninstallPluginImpl(ctx, req.PluginName)
	case actionShowPluginStatus:
		result, err = bl.ShowPluginStatus(ctx, req.PluginName, req.Lines)
	case actionGetInstallOutput:
		log.Debugw("handlePluginAction: calling method", "PluginName", req.PluginName, "params", req.Params)
		result, err = bl.getInstallOutputImpl(ctx, req.PluginName, req.Params)
	case actionGetInstallStatus:
		log.Debugw("handlePluginAction: calling method actionGetInstallStatus", "PluginName", req.PluginName)
		result, err = bl.getInstallStatusImpl(ctx, req.PluginName)
	case actionUpdatePlugin:
		log.Debug("handlePluginAction: calling method actionUpdatePlugin")
		result, err = bl.updatePluginImpl(ctx, req.PluginName)
	default:
		log.Error("Invalid action")
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Errorf("Error in plugin action %s for %s: %v", action, req.PluginName, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(result)
}
