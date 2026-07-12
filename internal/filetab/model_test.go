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

func TestFileTabInitializesHighlightingAndHandlesUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hello.go")
	if err := os.WriteFile(path, []byte("package hello\n\nfunc Hello() string { return \"hello\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("text file should request asynchronous highlighting")
	}
	model, _ := m.Update(cmd())
	model, _ = model.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if !strings.Contains(model.View(), "Hello") {
		t.Fatalf("updated tab lost file content:\n%s", model.View())
	}
}

func TestFileTabMarkdownToggleAndTinyWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte("# Heading\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Init() != nil {
		t.Fatal("rendered markdown should not request source highlighting")
	}
	model, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 1})
	if model.View() == "" {
		t.Fatal("tiny window should still render without panicking")
	}
	model, _ = model.Update(tea.WindowSizeMsg{Width: 40, Height: 8})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if !strings.Contains(model.View(), "Heading") {
		t.Fatalf("markdown source should remain visible after toggle:\n%s", model.View())
	}
}

func TestFileTabRejectsMissingPath(t *testing.T) {
	if _, err := New(filepath.Join(t.TempDir(), "missing.go")); err == nil {
		t.Fatal("file tab should reject a missing path")
	}
}
