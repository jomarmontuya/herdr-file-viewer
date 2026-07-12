package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMouseClickDirectoryTogglesExactTreeRow(t *testing.T) {
	m, err := New(fixtureRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	var model tea.Model = m
	model = send(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Row 0 is the root at terminal y=1 (below the header); docs/ is row 1.
	model = send(model, tea.MouseMsg{
		X:      2,
		Y:      2,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})

	got := model.(Model)
	n := got.tree.Selected()
	if n == nil || n.Name != "docs" || !n.IsDir {
		t.Fatalf("click should select docs directory, got %+v", n)
	}
	if !n.Expanded {
		t.Fatal("click should expand the selected directory")
	}
}

func TestMouseClickFileOpensHerdrTabInSameWorkspace(t *testing.T) {
	root := fixtureRoot(t)
	logPath := filepath.Join(t.TempDir(), "herdr-args.log")
	fakeHerdr := filepath.Join(t.TempDir(), "herdr")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
if [ "$1 $2 $3" = "plugin pane open" ]; then
  printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w-test:p9","tab_id":"w-test:t2"}}}}'
else
  printf '%s\n' '{"result":{"type":"tab_renamed"}}'
fi
`
	if err := os.WriteFile(fakeHerdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_BIN_PATH", fakeHerdr)
	t.Setenv("HERDR_WORKSPACE_ID", "w-test")
	t.Setenv("HERDR_TEST_LOG", logPath)

	m, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	var model tea.Model = m
	model = send(model, tea.WindowSizeMsg{Width: 120, Height: 40})

	// Visible rows are root, docs/, pkg/, main.go. Header occupies terminal y=0.
	next, cmd := model.Update(tea.MouseMsg{
		X:      2,
		Y:      4,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd == nil {
		t.Fatal("file click should return a command that opens a Herdr tab")
	}
	selected := next.(Model).tree.Selected()
	if selected == nil || selected.Path != filepath.Join(root, "main.go") {
		t.Fatalf("click should target main.go, got %+v", selected)
	}
	_ = cmd()

	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(logged)
	wantOpen := "plugin pane open --plugin medianeth.file-viewer --entrypoint file --placement tab --workspace w-test --cwd " + root + " --env HERDR_FILE_PATH=" + filepath.Join(root, "main.go") + " --focus"
	if !strings.Contains(got, wantOpen) {
		t.Fatalf("file click opened wrong tab\nwant args containing: %s\ngot:\n%s", wantOpen, got)
	}
	if !strings.Contains(got, "tab rename w-test:t2 main.go") {
		t.Fatalf("new Herdr tab should be named after the file; got:\n%s", got)
	}
}
