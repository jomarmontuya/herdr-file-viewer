package herdr

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const (
	workspacePaneAttempts   = 40
	workspacePaneRetryDelay = 50 * time.Millisecond
	workspaceTreeLockFile   = ".workspace-tree.lock"
)

var workspaceTreeMu sync.Mutex

// EnsureWorkspaceTree attaches one unfocused tree to the first pane created in
// a workspace. workspace.created can arrive before its initial pane exists, so
// the hook waits briefly for that pane instead of racing workspace setup.
func EnsureWorkspaceTree(workspaceID string) error {
	if workspaceID == "" {
		return errors.New("Herdr workspace ID is unavailable")
	}
	workspaceTreeMu.Lock()
	defer workspaceTreeMu.Unlock()

	if stateDir := os.Getenv("HERDR_PLUGIN_STATE_DIR"); stateDir != "" {
		if err := os.MkdirAll(stateDir, 0o700); err != nil {
			return fmt.Errorf("create Herdr workspace-tree state directory: %w", err)
		}
		lock, err := os.OpenFile(filepath.Join(stateDir, workspaceTreeLockFile), os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return fmt.Errorf("open Herdr workspace-tree lock: %w", err)
		}
		defer lock.Close()
		if err := lockFileExclusive(lock); err != nil {
			return fmt.Errorf("lock Herdr workspace-tree hook: %w", err)
		}
		defer unlockFile(lock) //nolint:errcheck
	}

	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		bin = "herdr"
	}

	for attempt := 0; attempt < workspacePaneAttempts; attempt++ {
		out, err := exec.Command(bin, "pane", "list", "--workspace", workspaceID).CombinedOutput()
		if err != nil {
			return fmt.Errorf("list new Herdr workspace panes: %w: %s", err, out)
		}
		panes, err := parsePaneList(out)
		if err != nil {
			return fmt.Errorf("decode new Herdr workspace panes: %w", err)
		}

		targetPaneID := ""
		for _, pane := range panes {
			if pane.WorkspaceID != workspaceID {
				continue
			}
			if pane.Label == "File Tree" {
				return nil
			}
			if targetPaneID == "" && pane.PaneID != "" {
				targetPaneID = pane.PaneID
			}
		}
		if targetPaneID != "" {
			out, err = exec.Command(bin, openWorkspaceTreeArgs(targetPaneID)...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("attach default Herdr file tree: %w: %s", err, out)
			}
			return nil
		}
		if attempt+1 < workspacePaneAttempts {
			time.Sleep(workspacePaneRetryDelay)
		}
	}
	return errors.New("new Herdr workspace did not create an initial pane")
}

func openWorkspaceTreeArgs(targetPaneID string) []string {
	return []string{
		"plugin", "pane", "open",
		"--plugin", pluginID,
		"--entrypoint", "viewer",
		"--placement", "split",
		"--target-pane", targetPaneID,
		"--direction", "right",
		"--no-focus",
	}
}
