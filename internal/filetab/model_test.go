package filetab

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestFileTabRendersSelectedFileAndCloses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hello.go")
	if err := os.WriteFile(path, []byte("package hello\n\nconst Answer = 42\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	var model tea.Model = m
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := model.View()
	if !strings.Contains(view, "hello.go") || !strings.Contains(view, "const Answer = 42") {
		t.Fatalf("file tab should render the selected file; got:\n%s", view)
	}

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should close the file tab")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q command should return tea.QuitMsg")
	}
}

func TestFileTabRejectsDirectory(t *testing.T) {
	if _, err := New(t.TempDir()); err == nil {
		t.Fatal("file tab should reject a directory path")
	}
}

