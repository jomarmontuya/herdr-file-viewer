// Package herdr contains the small Herdr CLI bridge used by the file explorer.
package herdr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

const (
	pluginID         = "medianeth.file-viewer"
	fileTabStateFile = "file-tabs.json"
	fileTabLockFile  = ".file-tabs.lock"
)

type openedPane struct {
	PaneID string
	TabID  string
}

type fileTabState struct {
	Version int                          `json:"version"`
	Tabs    map[string]map[string]string `json:"tabs"`
	Roots   map[string]string            `json:"roots,omitempty"`
}

var fileTabStateMu sync.Mutex

// OpenFileTab focuses the existing tab for path in workspaceID, or opens a new
// read-only file tab and attaches a tree split to its right. State is guarded
// by a process-wide lock everywhere and an advisory cross-process lock on the
// macOS/Linux platforms supported by the plugin, so clicks from multiple tree
// panes cannot create duplicate tabs for the same absolute path.
func OpenFileTab(workspaceID, path, projectRoot string) error {
	if workspaceID == "" {
		return errors.New("Herdr workspace ID is unavailable")
	}
	if path == "" {
		return errors.New("file path is empty")
	}
	if projectRoot == "" {
		return errors.New("project root is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return err
	}

	bin := os.Getenv("HERDR_BIN_PATH")
	if bin == "" {
		bin = "herdr"
	}
	stateDir := os.Getenv("HERDR_PLUGIN_STATE_DIR")
	var openedTabID string
	openOrReuse := func(state *fileTabState) error {
		if state != nil {
			state.setRoot(workspaceID, root)
			if tabID := state.tabID(workspaceID, abs); tabID != "" && tabIsReusable(bin, tabID, workspaceID, abs) {
				return nil
			}
		}
		var openErr error
		openedTabID, openErr = openNewFileTab(bin, workspaceID, abs, root, state)
		return openErr
	}
	if stateDir == "" {
		err = openOrReuse(nil)
	} else {
		err = withFileTabState(stateDir, openOrReuse)
	}
	// Cleanup stays outside withFileTabState: a successful pane open followed
	// by an atomic state-save failure must roll the new tab back too.
	if err != nil && openedTabID != "" {
		closeTabBestEffort(bin, openedTabID)
	}
	return err
}

func openNewFileTab(bin, workspaceID, path, projectRoot string, state *fileTabState) (string, error) {
	out, err := exec.Command(bin, openFileTabArgs(workspaceID, path)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("open Herdr file tab: %w: %s", err, out)
	}
	opened, err := parseOpenedPane(out)
	if err != nil {
		return "", err
	}
	if out, err = exec.Command(bin, "tab", "rename", opened.TabID, filepath.Base(path)).CombinedOutput(); err != nil {
		return opened.TabID, fmt.Errorf("rename Herdr file tab: %w: %s", err, out)
	}
	if opened.PaneID == "" {
		return opened.TabID, errors.New("Herdr pane response did not include a pane ID")
	}
	if out, err = exec.Command(bin, openTreeArgs(opened.PaneID, projectRoot)...).CombinedOutput(); err != nil {
		return opened.TabID, fmt.Errorf("attach Herdr file tree: %w: %s", err, out)
	}
	if state != nil {
		state.set(workspaceID, path, opened.TabID)
	}
	return opened.TabID, nil
}

func closeTabBestEffort(bin, tabID string) {
	_ = exec.Command(bin, "tab", "close", tabID).Run()
}

func tabIsReusable(bin, tabID, workspaceID, path string) bool {
	out, err := exec.Command(bin, "tab", "get", tabID).CombinedOutput()
	if err != nil {
		return false
	}
	got, err := parseTabContext(out)
	if err != nil || got.TabID != tabID || got.WorkspaceID != workspaceID ||
		got.Label != filepath.Base(path) || got.PaneCount < 2 {
		return false
	}
	if !tabHasOwnedFilePane(bin, workspaceID, tabID, path) {
		return false
	}
	return exec.Command(bin, "tab", "focus", tabID).Run() == nil
}

func openFileTabArgs(workspaceID, path string) []string {
	return []string{
		"plugin", "pane", "open",
		"--plugin", pluginID,
		"--entrypoint", "file",
		"--placement", "tab",
		"--workspace", workspaceID,
		"--cwd", filepath.Dir(path),
		"--env", "HERDR_FILE_PATH=" + path,
		"--focus",
	}
}

func openTreeArgs(targetPaneID, projectRoot string) []string {
	return []string{
		"plugin", "pane", "open",
		"--plugin", pluginID,
		"--entrypoint", "viewer",
		"--placement", "split",
		"--target-pane", targetPaneID,
		"--env", "HERDR_TREE_ROOT=" + projectRoot,
		"--direction", "right",
		"--no-focus",
	}
}

func parseOpenedPane(raw []byte) (openedPane, error) {
	var response struct {
		Result struct {
			PluginPane struct {
				Pane struct {
					PaneID string `json:"pane_id"`
					TabID  string `json:"tab_id"`
				} `json:"pane"`
			} `json:"plugin_pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return openedPane{}, fmt.Errorf("decode Herdr pane response: %w", err)
	}
	opened := openedPane{
		PaneID: response.Result.PluginPane.Pane.PaneID,
		TabID:  response.Result.PluginPane.Pane.TabID,
	}
	if opened.TabID == "" {
		return openedPane{}, errors.New("Herdr pane response did not include a tab ID")
	}
	return opened, nil
}

func parseOpenedTabID(raw []byte) (string, error) {
	opened, err := parseOpenedPane(raw)
	return opened.TabID, err
}

type tabContext struct {
	TabID       string
	WorkspaceID string
	Label       string
	PaneCount   int
}

func parseTabContext(raw []byte) (tabContext, error) {
	var response struct {
		Result struct {
			Tab struct {
				TabID       string `json:"tab_id"`
				WorkspaceID string `json:"workspace_id"`
				Label       string `json:"label"`
				PaneCount   int    `json:"pane_count"`
			} `json:"tab"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return tabContext{}, err
	}
	got := tabContext{
		TabID:       response.Result.Tab.TabID,
		WorkspaceID: response.Result.Tab.WorkspaceID,
		Label:       response.Result.Tab.Label,
		PaneCount:   response.Result.Tab.PaneCount,
	}
	if got.TabID == "" || got.WorkspaceID == "" {
		return tabContext{}, errors.New("Herdr tab response is incomplete")
	}
	return got, nil
}

type paneContext struct {
	PaneID      string
	TabID       string
	WorkspaceID string
	Label       string
	Cwd         string
}

func tabHasOwnedFilePane(bin, workspaceID, tabID, path string) bool {
	out, err := exec.Command(bin, "pane", "list", "--workspace", workspaceID).CombinedOutput()
	if err != nil {
		return false
	}
	panes, err := parsePaneList(out)
	if err != nil {
		return false
	}
	wantDir := filepath.Clean(filepath.Dir(path))
	for _, pane := range panes {
		if pane.TabID == tabID && pane.WorkspaceID == workspaceID && pane.Label == "File" &&
			filepath.Clean(pane.Cwd) == wantDir {
			return true
		}
	}
	return false
}

func parsePaneList(raw []byte) ([]paneContext, error) {
	var response struct {
		Result struct {
			Panes []struct {
				PaneID      string `json:"pane_id"`
				TabID       string `json:"tab_id"`
				WorkspaceID string `json:"workspace_id"`
				Label       string `json:"label"`
				Cwd         string `json:"cwd"`
			} `json:"panes"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, err
	}
	panes := make([]paneContext, 0, len(response.Result.Panes))
	for _, pane := range response.Result.Panes {
		panes = append(panes, paneContext{
			PaneID:      pane.PaneID,
			TabID:       pane.TabID,
			WorkspaceID: pane.WorkspaceID,
			Label:       pane.Label,
			Cwd:         pane.Cwd,
		})
	}
	return panes, nil
}

func newFileTabState() fileTabState {
	return fileTabState{
		Version: 1,
		Tabs:    make(map[string]map[string]string),
		Roots:   make(map[string]string),
	}
}

func (s *fileTabState) tabID(workspaceID, path string) string {
	if s.Tabs == nil || s.Tabs[workspaceID] == nil {
		return ""
	}
	return s.Tabs[workspaceID][path]
}

func (s *fileTabState) set(workspaceID, path, tabID string) {
	if s.Tabs == nil {
		s.Tabs = make(map[string]map[string]string)
	}
	if s.Tabs[workspaceID] == nil {
		s.Tabs[workspaceID] = make(map[string]string)
	}
	s.Version = 1
	s.Tabs[workspaceID][path] = tabID
}

func (s *fileTabState) pathForTab(workspaceID, tabID string) string {
	for path, savedTabID := range s.Tabs[workspaceID] {
		if savedTabID == tabID {
			return path
		}
	}
	return ""
}

func (s *fileTabState) setRoot(workspaceID, root string) {
	if s.Roots == nil {
		s.Roots = make(map[string]string)
	}
	s.Version = 1
	s.Roots[workspaceID] = root
}

func (s *fileTabState) root(workspaceID string) string {
	if s.Roots == nil {
		return ""
	}
	return s.Roots[workspaceID]
}

func withFileTabState(dir string, fn func(*fileTabState) error) error {
	fileTabStateMu.Lock()
	defer fileTabStateMu.Unlock()

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create Herdr file-tab state directory: %w", err)
	}
	lock, err := os.OpenFile(filepath.Join(dir, fileTabLockFile), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open Herdr file-tab state lock: %w", err)
	}
	defer lock.Close()
	if err := lockFileExclusive(lock); err != nil {
		return fmt.Errorf("lock Herdr file-tab state: %w", err)
	}
	defer unlockFile(lock) //nolint:errcheck

	state, err := loadFileTabState(filepath.Join(dir, fileTabStateFile))
	if err != nil {
		// Corrupt or partially-written state is only a stale cache. Rebuild it
		// from successful opens instead of blocking file navigation.
		state = newFileTabState()
	}
	if err := fn(&state); err != nil {
		return err
	}
	if err := saveFileTabState(filepath.Join(dir, fileTabStateFile), state); err != nil {
		return fmt.Errorf("save Herdr file-tab state: %w", err)
	}
	return nil
}

func loadFileTabState(path string) (fileTabState, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return newFileTabState(), nil
	}
	if err != nil {
		return fileTabState{}, err
	}
	state := newFileTabState()
	if err := json.Unmarshal(raw, &state); err != nil {
		return fileTabState{}, err
	}
	if state.Version != 1 || state.Tabs == nil {
		return fileTabState{}, errors.New("unsupported Herdr file-tab state")
	}
	if state.Roots == nil {
		state.Roots = make(map[string]string)
	}
	return state, nil
}

func saveFileTabState(path string, state fileTabState) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".file-tabs-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	enc := json.NewEncoder(tmp)
	if err := enc.Encode(state); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
