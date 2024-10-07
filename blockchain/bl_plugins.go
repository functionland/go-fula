package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
)

const activePluginsFile = "/internal/active-plugins.txt"

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

func (bl *FxBlockchain) ListPlugins(ctx context.Context) ([]byte, error) {
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

func (bl *FxBlockchain) InstallPlugin(ctx context.Context, pluginName string, params []PluginParam) ([]byte, error) {
	// Read existing plugins
	plugins, err := bl.readActivePlugins()
	if err != nil {
		return nil, err
	}

	// Check if plugin already exists
	for _, p := range plugins {
		if p == pluginName {
			return []byte("Plugin already installed"), nil
		}
	}

	// Process parameters
	if len(params) > 0 {
		for _, param := range params {
			filePath := fmt.Sprintf("/internal/%s/%s.txt", pluginName, param.Name)
			dirPath := fmt.Sprintf("/internal/%s", pluginName)

			// Create directory if it doesn't exist
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return nil, fmt.Errorf("failed to create directory for plugin %s: %w", pluginName, err)
			}

			// Write parameter value to file
			content := strings.TrimSpace(param.Value)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return nil, fmt.Errorf("failed to write parameter file for plugin %s: %w", pluginName, err)
			}
		}
	}

	// Append new plugin
	plugins = append(plugins, pluginName)

	// Write updated list back to file
	if err := bl.writeActivePlugins(plugins); err != nil {
		return nil, err
	}

	return []byte("Plugin installed successfully"), nil
}

func (bl *FxBlockchain) UninstallPlugin(ctx context.Context, pluginName string) ([]byte, error) {
	// Read existing plugins
	plugins, err := bl.readActivePlugins()
	if err != nil {
		return nil, err
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
		return []byte("Plugin not found"), nil
	}

	// Write updated list back to file
	if err := bl.writeActivePlugins(newPlugins); err != nil {
		return nil, err
	}

	return []byte("Plugin uninstalled successfully"), nil
}

func (bl *FxBlockchain) readActivePlugins() ([]string, error) {
	content, err := os.ReadFile(activePluginsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read active plugins file: %w", err)
	}
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

func (bl *FxBlockchain) handlePluginAction(ctx context.Context, from peer.ID, w http.ResponseWriter, r *http.Request, action string) {
	var req struct {
		PluginName string        `json:"plugin_name"`
		Lines      int           `json:"lines,omitempty"`
		Params     []PluginParam `json:"params,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.PluginName == "" && action != "list" {
		http.Error(w, "Plugin name is required", http.StatusBadRequest)
		return
	}

	var result []byte
	var err error

	switch action {
	case "list":
		result, err = bl.ListPlugins(ctx)
	case "install":
		result, err = bl.InstallPlugin(ctx, req.PluginName, req.Params)
	case "uninstall":
		result, err = bl.UninstallPlugin(ctx, req.PluginName)
	case "status":
		result, err = bl.ShowPluginStatus(ctx, req.PluginName, req.Lines)
	default:
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
