package editor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAndPreferred(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "editors"), []byte(
		"# comment\n\ncode = code\n* nvim = nvim -p\nzed = zed --wait\n"), 0o644)
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", dir)

	eds := Load()
	if len(eds) != 3 {
		t.Fatalf("expected 3 editors, got %d: %+v", len(eds), eds)
	}
	pref, ok := Preferred(eds)
	if !ok || pref.Name != "nvim" {
		t.Fatalf("default should be nvim, got ok=%v name=%q", ok, pref.Name)
	}
	// The starred editor's command keeps its extra args.
	if got := pref.Command("/x/file.go").Args; got[len(got)-1] != "/x/file.go" || got[1] != "-p" {
		t.Errorf("command args wrong: %v", got)
	}
}

func TestPreferredNoDefaultAsks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "editors"), []byte("code = code\nvim = vim\n"), 0o644)
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", dir)
	if _, ok := Preferred(Load()); ok {
		t.Error("two editors, no default → Preferred must be false (picker)")
	}
}

func TestSingleEditorIsPreferred(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "editors"), []byte("code = code\n"), 0o644)
	t.Setenv("HERDR_PLUGIN_CONFIG_DIR", dir)
	if _, ok := Preferred(Load()); !ok {
		t.Error("a single configured editor should be used without asking")
	}
}
