package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jomarmontuya/herdr-file-viewer/internal/gitstatus"
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

func TestTreeOnlyManualRefreshLoadsGitDecorations(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := fixtureRoot(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("add", ".")
	run("commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() { changed() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	m.ready = true
	m.width, m.height = 60, 24
	next, cmd := m.handleTreeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("tree-only refresh should reload git status decorations")
	}
	msg := cmd()
	next, _ = next.Update(msg)
	view := ansiRe.ReplaceAllString(next.(Model).View(), "")
	if !strings.Contains(view, "main.go") || !strings.Contains(view, "M") {
		t.Fatalf("tree-only refresh did not render modified-file badge:\n%s", view)
	}
}

func TestTreeOnlyFollowsSourcePaneLiveCWD(t *testing.T) {
	oldRoot := fixtureRoot(t)
	newRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(newRoot, "project-grey.txt"), []byte("current project\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	fakeHerdr := filepath.Join(t.TempDir(), "herdr")
	script := `#!/bin/sh
if [ "$1 $2" = "pane list" ]; then
  printf '{"result":{"panes":[{"pane_id":"w-follow:p1","tab_id":"w-follow:t1","workspace_id":"w-follow","cwd":"%s","foreground_cwd":"%s"}]}}\n' "$HERDR_TEST_OLD_ROOT" "$HERDR_TEST_NEW_ROOT"
fi
`
	if err := os.WriteFile(fakeHerdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_BIN_PATH", fakeHerdr)
	t.Setenv("HERDR_WORKSPACE_ID", "w-follow")
	t.Setenv("HERDR_TREE_FOLLOW_PANE_ID", "w-follow:p1")
	t.Setenv("HERDR_TEST_OLD_ROOT", oldRoot)
	t.Setenv("HERDR_TEST_NEW_ROOT", newRoot)

	m, err := NewTree(oldRoot)
	if err != nil {
		t.Fatal(err)
	}
	m = send(m, tea.WindowSizeMsg{Width: 60, Height: 24}).(Model)
	cmd := followTreeCWDCommand()
	if cmd == nil {
		t.Fatal("default workspace tree should poll its source pane cwd")
	}
	next, _ := m.Update(cmd())
	got := next.(Model)
	if got.root != newRoot || got.tree.Root.Path != newRoot {
		t.Fatalf("tree did not re-root to live cwd: model=%q tree=%q want=%q", got.root, got.tree.Root.Path, newRoot)
	}
	if view := got.View(); !strings.Contains(view, "project-grey.txt") || strings.Contains(view, "main.go") {
		t.Fatalf("tree still renders the stale root:\n%s", view)
	}
}

func TestTreeOnlyWithoutFollowPaneStaysPinned(t *testing.T) {
	t.Setenv("HERDR_TREE_FOLLOW_PANE_ID", "")
	if cmd := followTreeCWDCommand(); cmd != nil {
		t.Fatal("file-tab and manually pinned trees must not poll another pane cwd")
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

func TestTreeOnlyMouseWheelScrollsFilesWithoutMovingCursor(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 24; i++ {
		name := filepath.Join(root, fmt.Sprintf("file-%02d.txt", i))
		if err := os.WriteFile(name, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	m = send(m, tea.WindowSizeMsg{Width: 44, Height: 10}).(Model)
	initialCursor := m.tree.Cursor()

	next, _ := m.Update(tea.MouseMsg{
		X: 2, Y: 4, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
	})
	m = next.(Model)
	if m.treeScroll == 0 {
		t.Fatal("mouse wheel down should advance the Files viewport")
	}
	if got := m.tree.Cursor(); got != initialCursor {
		t.Fatalf("wheel scrolling moved keyboard cursor: got %d, want %d", got, initialCursor)
	}
	view := ansiRe.ReplaceAllString(m.View(), "")
	if strings.Contains(view, "file-00.txt") || !strings.Contains(view, "file-02.txt") {
		t.Fatalf("wheel did not reveal later file rows:\n%s", view)
	}

	// Click mapping must use the scrolled viewport, not the hidden cursor row.
	next, cmd := m.Update(tea.MouseMsg{
		X: 2, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	m = next.(Model)
	if cmd == nil || m.tree.Selected() == nil || m.tree.Selected().Name != "file-02.txt" {
		t.Fatalf("click after wheel scroll selected %+v, want file-02.txt", m.tree.Selected())
	}

	for i := 0; i < 20; i++ {
		next, _ = m.Update(tea.MouseMsg{X: 2, Y: 4, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
		m = next.(Model)
	}
	if m.treeScroll != 0 {
		t.Fatalf("wheel up should clamp at the first row, got offset %d", m.treeScroll)
	}
}

func TestTreeOnlyMouseWheelScrollsSourceControlWithoutMovingSelection(t *testing.T) {
	m, err := NewTree(fixtureRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	changes := make([]gitstatus.Change, 20)
	for i := range changes {
		changes[i] = gitstatus.Change{Path: fmt.Sprintf("change-%02d.ts", i), Index: ' ', Work: 'M'}
	}
	m = send(m, tea.WindowSizeMsg{Width: 44, Height: 10}).(Model)
	m = send(m, gitChangesMsg{changes: changes}).(Model)
	m.treeView = treeViewSourceControl
	initialCursor := m.sourceControlCursor

	next, _ := m.Update(tea.MouseMsg{
		X: 2, Y: 4, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
	})
	m = next.(Model)
	if m.sourceControlScroll == 0 {
		t.Fatal("mouse wheel down should advance the Source Control viewport")
	}
	if m.sourceControlCursor != initialCursor {
		t.Fatalf("wheel scrolling moved Source Control selection: got %d, want %d", m.sourceControlCursor, initialCursor)
	}
	view := ansiRe.ReplaceAllString(m.View(), "")
	if strings.Contains(view, "change-00.ts") || !strings.Contains(view, "change-01.ts") {
		t.Fatalf("wheel did not reveal later change rows:\n%s", view)
	}

	next, cmd := m.Update(tea.MouseMsg{
		X: 2, Y: 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	m = next.(Model)
	selected, ok := m.selectedSourceControlLine()
	if cmd == nil || !ok || selected.change.Path != "change-01.ts" {
		t.Fatalf("click after wheel selected %+v, want change-01.ts", selected)
	}
}

func TestTreeOnlyPersistsExpandedFoldersAndSelectionAcrossRestart(t *testing.T) {
	root := fixtureRoot(t)
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	t.Setenv("HERDR_WORKSPACE_ID", "w-state")
	t.Setenv("HERDR_TAB_ID", "w-state:t7")

	m, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	m.ready = true
	m.width, m.height = 60, 24

	// Root children sort as docs/, pkg/, main.go. Expand docs/ and select its file.
	next, _ := m.handleTreeKey(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, _ = m.handleTreeKey(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.handleTreeKey(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	if n := m.tree.Selected(); n == nil || n.Name != "readme.md" {
		t.Fatalf("test setup should select docs/readme.md, got %+v", n)
	}

	restored, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	n := restored.tree.Selected()
	if n == nil || n.Path != filepath.Join(root, "docs", "readme.md") {
		t.Fatalf("tree selection was not restored: %+v", n)
	}
	docs := restored.tree.Visible()[1]
	if docs.Name != "docs" || !docs.Expanded {
		t.Fatalf("expanded folder state was not restored: %+v", docs)
	}
}

func TestTreeOnlyClonesSourceTabStateForNewFileTabTree(t *testing.T) {
	root := fixtureRoot(t)
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())
	t.Setenv("HERDR_WORKSPACE_ID", "w-clone")
	t.Setenv("HERDR_TAB_ID", "w-clone:t-source")

	source, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	source.ready = true
	source.width, source.height = 60, 24
	next, _ := source.handleTreeKey(tea.KeyMsg{Type: tea.KeyDown})
	source = next.(Model)
	next, _ = source.handleTreeKey(tea.KeyMsg{Type: tea.KeyRight})
	source = next.(Model)
	next, _ = source.handleTreeKey(tea.KeyMsg{Type: tea.KeyDown})
	source = next.(Model)
	if n := source.tree.Selected(); n == nil || n.Path != filepath.Join(root, "docs", "readme.md") {
		t.Fatalf("test setup should select expanded docs/readme.md, got %+v", n)
	}

	t.Setenv("HERDR_TAB_ID", "w-clone:t-new-file")
	t.Setenv("HERDR_TREE_STATE_SOURCE_TAB_ID", "w-clone:t-source")
	cloned, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	n := cloned.tree.Selected()
	if n == nil || n.Path != filepath.Join(root, "docs", "readme.md") {
		t.Fatalf("new file-tab tree did not clone selected row: %+v", n)
	}
	docs := cloned.tree.Visible()[1]
	if docs.Name != "docs" || !docs.Expanded {
		t.Fatalf("new file-tab tree did not clone expanded folders: %+v", docs)
	}
}

func TestTreeOnlyIgnoresCorruptOrDifferentRootState(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("HERDR_PLUGIN_STATE_DIR", stateDir)
	t.Setenv("HERDR_WORKSPACE_ID", "w-state")
	t.Setenv("HERDR_TAB_ID", "w-state:t-corrupt")
	statePath := treeStatePathFromEnv()
	if err := os.WriteFile(statePath, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := fixtureRoot(t)
	m, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.tree.Selected() != m.tree.Root {
		t.Fatal("corrupt state should fall back to the default root selection")
	}

	otherRoot := fixtureRoot(t)
	m.persistTreeState()
	restored, err := NewTree(otherRoot)
	if err != nil {
		t.Fatal(err)
	}
	if restored.tree.Selected() != restored.tree.Root {
		t.Fatal("state saved for a different root must be ignored")
	}
}
