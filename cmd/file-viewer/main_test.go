package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
