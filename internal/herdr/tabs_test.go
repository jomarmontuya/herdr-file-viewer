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
		"--env", "HERDR_FILE_PATH=" + path,
		"--focus",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrong Herdr args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestOpenFileTabArgsDoNotOverridePluginWorkingDirectory(t *testing.T) {
	got := openFileTabArgs("w7", "/tmp/project/internal/editor/editor.go")
	for _, arg := range got {
		if arg == "--cwd" {
			t.Fatalf("file tab must keep the plugin-root cwd so ./bin/file-viewer resolves: %#v", got)
		}
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

func TestOpenFileTabOpensRenamesAndAttachesTree(t *testing.T) {
	_, logPath := fakeHerdr(t, `
if [ "$1 $2 $3" = "plugin pane open" ]; then
  case "$*" in
    *"--entrypoint file"*)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w2:p8","tab_id":"w2:t8"}}}}'
      ;;
    *)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w2:p9","tab_id":"w2:t8"}}}}'
      ;;
  esac
else
  printf '%s\n' '{"result":{"type":"tab_renamed"}}'
fi`)
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
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
	wantTree := "plugin pane open --plugin medianeth.file-viewer --entrypoint viewer --placement split --target-pane w2:p8 --direction right --no-focus"
	if !strings.Contains(string(logged), wantTree) {
		t.Fatalf("new file tab should retain the right-side tree\nwant: %s\ngot:\n%s", wantTree, logged)
	}
}

func TestOpenFileTabReusesExistingTabForSamePath(t *testing.T) {
	_, logPath := fakeHerdr(t, `
if [ "$1 $2 $3" = "plugin pane open" ]; then
  case "$*" in
    *"--entrypoint file"*)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w2:p8","tab_id":"w2:t8"}}}}'
      ;;
    *)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w2:p9","tab_id":"w2:t8"}}}}'
      ;;
  esac
elif [ "$1 $2" = "tab get" ]; then
  printf '%s\n' '{"result":{"tab":{"tab_id":"w2:t8","workspace_id":"w2"}}}'
else
  printf '%s\n' '{"result":{"type":"ok"}}'
fi`)
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	path := filepath.Join(t.TempDir(), "same.go")
	if err := os.WriteFile(path, []byte("package same\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := OpenFileTab("w2", path); err != nil {
		t.Fatal(err)
	}
	if err := OpenFileTab("w2", path); err != nil {
		t.Fatal(err)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(logged)
	if count := strings.Count(got, "--entrypoint file"); count != 1 {
		t.Fatalf("same file should create one file tab, got %d opens:\n%s", count, got)
	}
	if !strings.Contains(got, "tab get w2:t8") || !strings.Contains(got, "tab focus w2:t8") {
		t.Fatalf("same file should validate and focus its existing tab:\n%s", got)
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
