// Package herdr contains the small Herdr CLI bridge used by the file explorer.
package herdr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const pluginID = "medianeth.file-viewer"

// OpenFileTab opens path in the current Herdr workspace and gives the new tab
// the file's base name. Herdr owns the tab; this command only starts its plugin
// pane and renames the tab returned by the CLI.
func OpenFileTab(workspaceID, path string) error {
	if workspaceID == "" {
		return errors.New("Herdr workspace ID is unavailable")
	}
	if path == "" {
		return errors.New("file path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		bin = "herdr"
	}
	out, err := exec.Command(bin, openFileTabArgs(workspaceID, abs)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("open Herdr file tab: %w: %s", err, out)
	}
	tabID, err := parseOpenedTabID(out)
	if err != nil {
		return err
	}
	if out, err = exec.Command(bin, "tab", "rename", tabID, filepath.Base(abs)).CombinedOutput(); err != nil {
		return fmt.Errorf("rename Herdr file tab: %w: %s", err, out)
	}
	return nil
}

func openFileTabArgs(workspaceID, path string) []string {
	return []string{
		"plugin", "pane", "open",
		"--plugin", pluginID,
		"--entrypoint", "file",
		"--placement", "tab",
		"--workspace", workspaceID,
		"--env", "HERDR_FILE_PATH=" + path,
		"--focus",
	}
}

func parseOpenedTabID(raw []byte) (string, error) {
	var response struct {
		Result struct {
			PluginPane struct {
				Pane struct {
					TabID string `json:"tab_id"`
				} `json:"pane"`
			} `json:"plugin_pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return "", fmt.Errorf("decode Herdr pane response: %w", err)
	}
	if response.Result.PluginPane.Pane.TabID == "" {
		return "", errors.New("Herdr pane response did not include a tab ID")
	}
	return response.Result.PluginPane.Pane.TabID, nil
}
