package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTreeOnlyRequestsNarrowRightSidebar(t *testing.T) {
	got := treePaneResizeArgs("w1:p2")
	want := []string{
		"pane", "resize",
		"--direction", "right",
		"--amount", "0.2",
		"--pane", "w1:p2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wrong tree-pane resize args\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestTreeOnlyRendersOnlyTheExplorer(t *testing.T) {
	m, err := NewTree(fixtureRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	var model tea.Model = m
	model = send(model, tea.WindowSizeMsg{Width: 60, Height: 24})
	view := model.View()

	for _, want := range []string{"File Tree", "main.go", "docs/", "pkg/"} {
		if !strings.Contains(view, want) {
			t.Fatalf("tree-only pane missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"no file", "FIND FILE", "CHANGES", "Go to file", "git log"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("tree-only pane should remove %q:\n%s", unwanted, view)
		}
	}
}

func TestTreeOnlyClickOpensFileTabAcrossFullPaneWidth(t *testing.T) {
	root := fixtureRoot(t)
	logPath := filepath.Join(t.TempDir(), "herdr-args.log")
	fakeHerdr := filepath.Join(t.TempDir(), "herdr")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
if [ "$1 $2 $3" = "plugin pane open" ]; then
  printf '%s\n' '{"result":{"plugin_pane":{"pane":{"tab_id":"w-tree:t2"}}}}'
else
  printf '%s\n' '{"result":{"type":"tab_renamed"}}'
fi
`
	if err := os.WriteFile(fakeHerdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_BIN_PATH", fakeHerdr)
	t.Setenv("HERDR_WORKSPACE_ID", "w-tree")
	t.Setenv("HERDR_TEST_LOG", logPath)

	m, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	var model tea.Model = m
	model = send(model, tea.WindowSizeMsg{Width: 60, Height: 24})

	// Visible rows are root, docs/, pkg/, main.go. X=50 proves the whole slim
	// pane is clickable, not only the old 34-column explorer region.
	next, cmd := model.Update(tea.MouseMsg{
		X:      50,
		Y:      4,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	})
	if cmd == nil {
		t.Fatal("tree-only file click should open a Herdr tab")
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
	if !strings.Contains(string(logged), "HERDR_FILE_PATH="+filepath.Join(root, "main.go")) {
		t.Fatalf("tree-only click opened wrong file:\n%s", logged)
	}
}
