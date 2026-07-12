package herdr

import (
	"path/filepath"
	"reflect"
	"testing"
)

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

