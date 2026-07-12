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

func TestParseTabContextRejectsMalformedOrIncompleteResponse(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte("not-json"),
		[]byte(`{"result":{"tab":{"tab_id":"w2:t8"}}}`),
		[]byte(`{"result":{"tab":{"workspace_id":"w2"}}}`),
	} {
		if _, err := parseTabContext(raw); err == nil {
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
  printf '%s\n' '{"result":{"tab":{"tab_id":"w2:t8","workspace_id":"w2","label":"same.go","pane_count":2}}}'
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

func TestOpenFileTabReplacesMismatchedLiveTabRecord(t *testing.T) {
	_, logPath := fakeHerdr(t, `
if [ "$1 $2" = "tab get" ]; then
  printf '%s\n' '{"result":{"tab":{"tab_id":"w2:t-wrong","workspace_id":"w2","label":"other.go","pane_count":2}}}'
elif [ "$1 $2 $3" = "plugin pane open" ]; then
  case "$*" in
    *"--entrypoint file"*)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w2:p-new","tab_id":"w2:t-new"}}}}'
      ;;
    *)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w2:p-tree","tab_id":"w2:t-new"}}}}'
      ;;
  esac
else
  printf '%s\n' '{"result":{"type":"ok"}}'
fi`)
	stateDir := t.TempDir()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", stateDir)
	path := filepath.Join(t.TempDir(), "wanted.go")
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	state := newFileTabState()
	state.set("w2", abs, "w2:t-wrong")
	if err := saveFileTabState(filepath.Join(stateDir, fileTabStateFile), state); err != nil {
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
	if strings.Contains(got, "tab focus w2:t-wrong") {
		t.Fatalf("mismatched record must not focus unrelated live tab:\n%s", got)
	}
	if strings.Count(got, "--entrypoint file") != 1 {
		t.Fatalf("mismatched record should be replaced with a fresh file tab:\n%s", got)
	}
}

func TestOpenFileTabReplacesClosedTabRecord(t *testing.T) {
	_, logPath := fakeHerdr(t, `
if [ "$1 $2" = "tab get" ]; then
  printf '%s\n' 'tab closed' >&2
  exit 4
elif [ "$1 $2 $3" = "plugin pane open" ]; then
  case "$*" in
    *"--entrypoint file"*)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w2:p-new","tab_id":"w2:t-new"}}}}'
      ;;
    *)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w2:p-tree","tab_id":"w2:t-new"}}}}'
      ;;
  esac
else
  printf '%s\n' '{"result":{"type":"ok"}}'
fi`)
	stateDir := t.TempDir()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", stateDir)
	path := filepath.Join(t.TempDir(), "reopened.go")
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	state := newFileTabState()
	state.set("w2", abs, "w2:t-closed")
	if err := saveFileTabState(filepath.Join(stateDir, fileTabStateFile), state); err != nil {
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
	if !strings.Contains(got, "tab get w2:t-closed") || strings.Count(got, "--entrypoint file") != 1 {
		t.Fatalf("closed record should be checked then replaced:\n%s", got)
	}
	saved, err := loadFileTabState(filepath.Join(stateDir, fileTabStateFile))
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.tabID("w2", abs); got != "w2:t-new" {
		t.Fatalf("closed record not replaced: got %q", got)
	}
}

func TestOpenFileTabRecoversFromMalformedState(t *testing.T) {
	fakeHerdr(t, `
if [ "$1 $2 $3" = "plugin pane open" ]; then
  case "$*" in
    *"--entrypoint file"*)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w3:p3","tab_id":"w3:t3"}}}}'
      ;;
    *)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w3:p4","tab_id":"w3:t3"}}}}'
      ;;
  esac
else
  printf '%s\n' '{"result":{"type":"ok"}}'
fi`)
	stateDir := t.TempDir()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", stateDir)
	statePath := filepath.Join(stateDir, fileTabStateFile)
	if err := os.WriteFile(statePath, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "valid.go")

	if err := OpenFileTab("w3", path); err != nil {
		t.Fatal(err)
	}
	saved, err := loadFileTabState(statePath)
	if err != nil {
		t.Fatalf("malformed state should be atomically replaced: %v", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.tabID("w3", abs); got != "w3:t3" {
		t.Fatalf("recovered state missing file record: got %q", got)
	}
}

func TestFileTabStateDistinguishesSameBasenamePaths(t *testing.T) {
	state := newFileTabState()
	first := filepath.Join(t.TempDir(), "main.go")
	second := filepath.Join(t.TempDir(), "main.go")
	state.set("w4", first, "w4:t1")
	state.set("w4", second, "w4:t2")

	if got := state.tabID("w4", first); got != "w4:t1" {
		t.Fatalf("first path got %q", got)
	}
	if got := state.tabID("w4", second); got != "w4:t2" {
		t.Fatalf("second path got %q", got)
	}
}

func TestFileTabStateInitializesNilMaps(t *testing.T) {
	state := fileTabState{}
	state.set("w4", "/tmp/main.go", "w4:t1")
	if got := state.tabID("w4", "/tmp/main.go"); got != "w4:t1" {
		t.Fatalf("initialized state got %q", got)
	}
	if state.Version != 1 {
		t.Fatalf("initialized state version got %d", state.Version)
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

func TestOpenFileTabRollsBackWhenTreeAttachmentFails(t *testing.T) {
	_, logPath := fakeHerdr(t, `
if [ "$1 $2 $3" = "plugin pane open" ]; then
  case "$*" in
    *"--entrypoint file"*)
      printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w2:p8","tab_id":"w2:t8"}}}}'
      ;;
    *)
      printf '%s\n' 'tree failed' >&2
      exit 8
      ;;
  esac
else
  printf '%s\n' '{"result":{"type":"ok"}}'
fi`)
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())

	err := OpenFileTab("w2", "/tmp/example.go")
	if err == nil || !strings.Contains(err.Error(), "tree failed") {
		t.Fatalf("expected tree attachment failure, got %v", err)
	}
	logged, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(logged), "tab close w2:t8") {
		t.Fatalf("failed file tab must be rolled back:\n%s", logged)
	}
}
