package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
