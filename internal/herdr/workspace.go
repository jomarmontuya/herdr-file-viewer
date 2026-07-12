package herdr

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	workspacePaneAttempts   = 40
	workspacePaneRetryDelay = 50 * time.Millisecond
)

// EnsureWorkspaceTree attaches one unfocused tree to the first pane created in
// a workspace. workspace.created can arrive before its initial pane exists, so
// the hook waits briefly for that pane instead of racing workspace setup.
func EnsureWorkspaceTree(workspaceID string) error {
	if workspaceID == "" {
		return errors.New("Herdr workspace ID is unavailable")
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
			out, err = exec.Command(bin, openWorkspaceTreeArgs(workspaceID, targetPaneID)...).CombinedOutput()
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

func openWorkspaceTreeArgs(workspaceID, targetPaneID string) []string {
	return []string{
		"plugin", "pane", "open",
		"--plugin", pluginID,
		"--entrypoint", "viewer",
		"--placement", "split",
		"--workspace", workspaceID,
		"--target-pane", targetPaneID,
		"--direction", "right",
		"--no-focus",
	}
}
