package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jomarmontuya/herdr-file-viewer/internal/filetab"
	"github.com/jomarmontuya/herdr-file-viewer/internal/ui"
)

type mouseOptionProbe struct{}

func (mouseOptionProbe) Init() tea.Cmd { return nil }
func (mouseOptionProbe) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return mouseOptionProbe{}, tea.Quit
	}
	return mouseOptionProbe{}, nil
}
func (mouseOptionProbe) View() string { return "probe" }

func TestFileTabsLeaveMouseSelectionToHerdr(t *testing.T) {
	if shouldCaptureMouse(filetab.Model{}) {
		t.Fatal("standalone file tabs must leave mouse drag selection to Herdr core")
	}
}

func TestInteractiveTreePanesKeepMouseCapture(t *testing.T) {
	if !shouldCaptureMouse(ui.Model{}) {
		t.Fatal("tree/browser panes still need Bubble Tea mouse events for clicking files and folders")
	}
}

func TestProgramOptionsOnlyEmitMouseTrackingForInteractiveTreePanes(t *testing.T) {
	const enableCellMotion = "\x1b[?1002h"

	for _, tc := range []struct {
		name      string
		model     tea.Model
		wantMouse bool
	}{
		{name: "file tab", model: filetab.Model{}, wantMouse: false},
		{name: "tree pane", model: ui.Model{}, wantMouse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			options := append(programOptions(tc.model),
				tea.WithInput(bytes.NewBufferString("q")),
				tea.WithOutput(&output),
				tea.WithoutSignalHandler(),
			)
			if _, err := tea.NewProgram(mouseOptionProbe{}, options...).Run(); err != nil {
				t.Fatal(err)
			}
			gotMouse := strings.Contains(output.String(), enableCellMotion)
			if gotMouse != tc.wantMouse {
				t.Fatalf("mouse tracking emitted = %v, want %v; output %q", gotMouse, tc.wantMouse, output.String())
			}
		})
	}
}

func TestResolveRootSkipsTreeOnlyFlag(t *testing.T) {
	root := t.TempDir()
	contextJSON, err := json.Marshal(map[string]string{"workspace_cwd": root})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_PLUGIN_CONTEXT_JSON", string(contextJSON))

	oldArgs := os.Args
	os.Args = []string{filepath.Join(root, "file-viewer"), "--tree-only"}
	t.Cleanup(func() { os.Args = oldArgs })

	if got := resolveRoot(); got != root {
		t.Fatalf("--tree-only must not become the project root: got %q, want %q", got, root)
	}
}

func TestResolveRootPrefersExplicitTreeRootOverFocusedFileContext(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "scripts")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	contextJSON, err := json.Marshal(map[string]string{
		"workspace_cwd":    nested,
		"focused_pane_cwd": nested,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_TREE_ROOT", root)
	t.Setenv("HERDR_PLUGIN_CONTEXT_JSON", string(contextJSON))

	oldArgs := os.Args
	os.Args = []string{filepath.Join(root, "file-viewer"), "--tree-only"}
	t.Cleanup(func() { os.Args = oldArgs })

	if got := resolveRoot(); got != root {
		t.Fatalf("file-tab tree must stay at project root: got %q, want %q", got, root)
	}
}

func TestWorkspaceIDFromEventPrefersInjectedContext(t *testing.T) {
	t.Setenv("HERDR_WORKSPACE_ID", "w12")
	t.Setenv("HERDR_PLUGIN_EVENT_JSON", `{"data":{"workspace":{"workspace_id":"wrong"}}}`)
	if got := workspaceIDFromEvent(); got != "w12" {
		t.Fatalf("got %q, want w12", got)
	}
}

func TestWorkspaceIDFromEventSupportsEventEnvelope(t *testing.T) {
	t.Setenv("HERDR_WORKSPACE_ID", "")
	t.Setenv("HERDR_PLUGIN_EVENT_JSON", `{"event":"workspace.created","data":{"type":"workspace_created","workspace":{"workspace_id":"w13"}}}`)
	if got := workspaceIDFromEvent(); got != "w13" {
		t.Fatalf("got %q, want w13", got)
	}
}

func TestTabIDFromEventSupportsFocusedTabEnvelope(t *testing.T) {
	t.Setenv("HERDR_TAB_ID", "")
	t.Setenv("HERDR_PLUGIN_EVENT_JSON", `{"event":"tab.focused","data":{"type":"tab_focused","workspace_id":"w13","tab_id":"w13:t7"}}`)
	if got := tabIDFromEvent(); got != "w13:t7" {
		t.Fatalf("got %q, want w13:t7", got)
	}
}

func TestManifestRegistersWorkspaceCreatedHook(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "herdr-plugin.toml"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := string(raw)
	if !strings.Contains(manifest, `on = "workspace.created"`) ||
		!strings.Contains(manifest, `command = ["./bin/file-viewer", "--workspace-created"]`) {
		t.Fatalf("manifest must auto-attach the tree on workspace creation:\n%s", manifest)
	}
}

func TestManifestRegistersFocusedTabRestoreHook(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "herdr-plugin.toml"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := string(raw)
	if !strings.Contains(manifest, `on = "tab.focused"`) {
		t.Fatalf("manifest must restore plugin panes when a tab becomes focused:\n%s", manifest)
	}
	if !strings.Contains(manifest, `command = ["./bin/file-viewer", "--restore-focused-tab"]`) {
		t.Fatalf("restore hooks must invoke the focused-tab restorer:\n%s", manifest)
	}
}
