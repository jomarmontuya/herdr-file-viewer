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

func TestTreeOnlyResizeCommandUsesHerdrPaneContext(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "resize.log")
	fakeHerdr := filepath.Join(t.TempDir(), "herdr")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > \"$HERDR_TEST_LOG\"\n"
	if err := os.WriteFile(fakeHerdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_BIN_PATH", fakeHerdr)
	t.Setenv("HERDR_PANE_ID", "w9:p4")
	t.Setenv("HERDR_TEST_LOG", logPath)

	cmd := resizeTreePaneCmd()
	if cmd == nil {
		t.Fatal("tree pane with Herdr context should request a resize")
	}
	_ = cmd()
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(logged)), "pane resize --direction right --amount 0.2 --pane w9:p4"; got != want {
		t.Fatalf("wrong resize command: got %q, want %q", got, want)
	}

	t.Setenv("HERDR_PANE_ID", "")
	if cmd := resizeTreePaneCmd(); cmd != nil {
		t.Fatal("tree pane outside Herdr should not request a resize")
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

func TestTreeOnlyKeyboardNavigationAndOpen(t *testing.T) {
	m, err := NewTree(fixtureRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	m.ready = true
	m.width, m.height = 60, 24

	// Root starts expanded. Collapse and re-expand it, then move to main.go.
	next, _ := m.handleTreeKey(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	if m.tree.Selected() == nil || m.tree.Selected().Expanded {
		t.Fatal("left should collapse the selected root")
	}
	next, _ = m.handleTreeKey(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if !m.tree.Selected().Expanded {
		t.Fatal("right should expand the selected root")
	}
	for range 3 {
		next, _ = m.handleTreeKey(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	if n := m.tree.Selected(); n == nil || n.Name != "main.go" {
		t.Fatalf("keyboard should select main.go, got %+v", n)
	}
	_, cmd := m.handleTreeKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on a file should open a Herdr tab")
	}

	next, _ = m.handleTreeKey(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(Model)
	next, _ = m.handleTreeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = next.(Model)
	m.statusNote = "file tab failed: test"
	if !strings.Contains(m.View(), m.statusNote) {
		t.Fatal("tree-only view should surface file-tab errors")
	}
	_, quit := m.handleTreeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if quit == nil {
		t.Fatal("q should close the tree pane")
	}
}
