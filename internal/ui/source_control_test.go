package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jomarmontuya/herdr-file-viewer/internal/gitdiff"
	"github.com/jomarmontuya/herdr-file-viewer/internal/gitstatus"
)

func TestTreeSourceControlGroupsStagedAndWorktreeChanges(t *testing.T) {
	m, err := NewTree(fixtureRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	m = send(m, tea.WindowSizeMsg{Width: 60, Height: 24}).(Model)
	m = send(m, gitChangesMsg{changes: []gitstatus.Change{
		{Path: "partial.ts", Index: 'M', Work: 'M'},
		{Path: "staged.ts", Index: 'A', Work: ' '},
		{Path: "worktree.ts", Index: ' ', Work: 'M'},
		{Path: "fresh.ts", Index: '?', Work: '?'},
	}}).(Model)

	next, _ := m.handleTreeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = next.(Model)
	if m.treeView != treeViewSourceControl {
		t.Fatalf("g should open Source Control, got %v", m.treeView)
	}

	view := ansiRe.ReplaceAllString(m.View(), "")
	for _, want := range []string{
		"Source Control", "Staged Changes (2)", "Changes (3)",
		"partial.ts", "staged.ts", "worktree.ts", "fresh.ts",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("Source Control missing %q:\n%s", want, view)
		}
	}
	if got := strings.Count(view, "partial.ts"); got != 2 {
		t.Fatalf("partially staged file must appear in both groups, got %d rows:\n%s", got, view)
	}
	if got := m.sourceControlCount(); got != 4 {
		t.Fatalf("activity badge should count unique changed files, got %d", got)
	}

	var partialModes []gitdiff.Mode
	for _, line := range m.sourceControlLines {
		if line.selectable && line.change.Path == "partial.ts" {
			partialModes = append(partialModes, line.mode)
		}
	}
	if len(partialModes) != 2 || partialModes[0] != gitdiff.ModeStaged || partialModes[1] != gitdiff.ModeWorktree {
		t.Fatalf("partial.ts modes = %v, want staged then worktree", partialModes)
	}
}

func TestTreeActivityRailSwitchesWithMouseAndKeyboard(t *testing.T) {
	m, err := NewTree(fixtureRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	m = send(m, tea.WindowSizeMsg{Width: 50, Height: 20}).(Model)
	m = send(m, gitChangesMsg{changes: []gitstatus.Change{{Path: "main.go", Index: ' ', Work: 'M'}}}).(Model)

	next, _ := m.Update(tea.MouseMsg{
		X: 49, Y: activityGitRow + 1,
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	m = next.(Model)
	if m.treeView != treeViewSourceControl {
		t.Fatal("clicking the Git rail button should open Source Control")
	}
	if view := ansiRe.ReplaceAllString(m.View(), ""); !strings.Contains(view, "[G]") || !strings.Contains(view, "1") {
		t.Fatalf("activity rail should show Git and its dirty count:\n%s", view)
	}

	next, _ = m.handleTreeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = next.(Model)
	if m.treeView != treeViewFiles {
		t.Fatal("f should return to Files")
	}

	next, _ = m.Update(tea.MouseMsg{
		X: 49, Y: activityGitRow + 1,
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	m = next.(Model)
	next, _ = m.Update(tea.MouseMsg{
		X: 49, Y: activityFilesRow + 1,
		Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
	})
	if next.(Model).treeView != treeViewFiles {
		t.Fatal("clicking the Files rail button should return to the file tree")
	}
}

func TestTreeSourceControlEnterOpensModeSpecificDiffTab(t *testing.T) {
	root := fixtureRoot(t)
	logPath := filepath.Join(t.TempDir(), "herdr.log")
	fakeHerdr := filepath.Join(t.TempDir(), "herdr")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$HERDR_TEST_LOG"
if [ "$1 $2 $3" = "plugin pane open" ]; then
  printf '%s\n' '{"result":{"plugin_pane":{"pane":{"pane_id":"w-sc:p2","tab_id":"w-sc:t2"}}}}'
else
  printf '%s\n' '{"result":{"type":"ok"}}'
fi
`
	if err := os.WriteFile(fakeHerdr, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_BIN_PATH", fakeHerdr)
	t.Setenv("HERDR_TEST_LOG", logPath)
	t.Setenv("HERDR_WORKSPACE_ID", "w-sc")
	t.Setenv("HERDR_PLUGIN_STATE_DIR", t.TempDir())

	m, err := NewTree(root)
	if err != nil {
		t.Fatal(err)
	}
	m.ready, m.width, m.height = true, 60, 24
	m = send(m, gitChangesMsg{changes: []gitstatus.Change{{Path: "main.go", Index: 'M', Work: 'M'}}}).(Model)
	next, _ := m.handleTreeKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = next.(Model)

	// The first selectable line is the staged half of the partially staged file.
	next, cmd := m.handleTreeKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on a Source Control file should open its diff tab")
	}
	_ = next
	_ = cmd()

	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(logged)
	for _, want := range []string{
		"--entrypoint diff", "HERDR_DIFF_PATH=" + filepath.Join(root, "main.go"),
		"HERDR_DIFF_MODE=staged", "--entrypoint viewer", "HERDR_TREE_ROOT=" + root,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("diff-tab command missing %q:\n%s", want, got)
		}
	}
}

func TestDiffTabRendersOnlyRequestedSideOfPartialChange(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	path := filepath.Join(root, "demo.txt")
	runGit("init", "-q")
	if err := os.WriteFile(path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "demo.txt")
	runGit("commit", "-qm", "base")
	if err := os.WriteFile(path, []byte("base\nstaged line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "demo.txt")
	if err := os.WriteFile(path, []byte("base\nstaged line\nworktree line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := NewDiffTab(root, path, gitdiff.ModeStaged)
	if err != nil {
		t.Fatal(err)
	}
	m = send(m, tea.WindowSizeMsg{Width: 100, Height: 30}).(DiffTabModel)
	if cmd := m.Init(); cmd != nil {
		m = send(m, cmd()).(DiffTabModel)
	}
	view := ansiRe.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, "staged line") || strings.Contains(view, "worktree line") {
		t.Fatalf("staged diff tab leaked worktree content:\n%s", view)
	}
}
