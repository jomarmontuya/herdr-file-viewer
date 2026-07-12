package herdr

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fakeHerdr(t *testing.T, scriptBody string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "herdr")
	logPath := filepath.Join(dir, "args.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$HERDR_TEST_LOG\"\n" + scriptBody + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_BIN_PATH", bin)
	t.Setenv("HERDR_TEST_LOG", logPath)
	return bin, logPath
}

func TestOpenFileTabArgsStayInWorkspace(t *testing.T) {
	path := filepath.Join("/tmp", "project", "docs", "read me.md")
	got := openFileTabArgs("w7", path)
	want := []string{
		"plugin", "pane", "open",
		"--plugin", "medianeth.file-viewer",
		"--entrypoint", "file",
		"--placement", "tab",
		"--workspace", "w7",
		"--cwd", filepath.Dir(path),
		"--env", "HERDR_FILE_PATH=" + path,
		"--focus",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrong Herdr args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestParseOpenedTabID(t *testing.T) {
	raw := []byte(`{"result":{"plugin_pane":{"pane":{"pane_id":"w7:p3","tab_id":"w7:t9"}}}}`)
	got, err := parseOpenedTabID(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != "w7:t9" {
		t.Fatalf("want w7:t9, got %q", got)
	}
}

func TestParseOpenedTabIDRejectsMalformedResponse(t *testing.T) {
	for _, raw := range [][]byte{[]byte("not-json"), []byte(`{"result":{}}`)} {
		if _, err := parseOpenedTabID(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestOpenFileTabOpensAndRenamesTab(t *testing.T) {
	_, logPath := fakeHerdr(t, `
if [ "$1 $2 $3" = "plugin pane open" ]; then
  printf '%s\n' '{"result":{"plugin_pane":{"pane":{"tab_id":"w2:t8"}}}}'
else
  printf '%s\n' '{"result":{"type":"tab_renamed"}}'
fi`)
	path := filepath.Join(t.TempDir(), "file with spaces.go")
	if err := os.WriteFile(path, []byte("package example\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := OpenFileTab("w2", path); err != nil {
		t.Fatal(err)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logged), "tab rename w2:t8 file with spaces.go") {
		t.Fatalf("expected tab rename in log, got:\n%s", logged)
	}
}

func TestOpenFileTabValidatesContextAndPath(t *testing.T) {
	if err := OpenFileTab("", "/tmp/file"); err == nil || !strings.Contains(err.Error(), "workspace ID") {
		t.Fatalf("expected missing workspace error, got %v", err)
	}
	if err := OpenFileTab("w1", ""); err == nil || !strings.Contains(err.Error(), "file path") {
		t.Fatalf("expected missing path error, got %v", err)
	}
}

func TestOpenFileTabReportsOpenAndRenameFailures(t *testing.T) {
	t.Run("open", func(t *testing.T) {
		fakeHerdr(t, `printf '%s\n' 'open failed' >&2; exit 7`)
		err := OpenFileTab("w1", "/tmp/example.go")
		if err == nil || !strings.Contains(err.Error(), "open failed") {
			t.Fatalf("expected open stderr, got %v", err)
		}
	})

	t.Run("rename", func(t *testing.T) {
		fakeHerdr(t, `
if [ "$1 $2 $3" = "plugin pane open" ]; then
  printf '%s\n' '{"result":{"plugin_pane":{"pane":{"tab_id":"w1:t2"}}}}'
else
  printf '%s\n' 'rename failed' >&2
  exit 9
fi`)
		err := OpenFileTab("w1", "/tmp/example.go")
		if err == nil || !strings.Contains(err.Error(), "rename failed") {
			t.Fatalf("expected rename stderr, got %v", err)
		}
	})
}
