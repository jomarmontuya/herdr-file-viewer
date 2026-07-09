package ui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Regression: the 2s auto-refresh reloads the file index; it must not yank the
// finder selection back to the top while the user is scrolling.
func TestFinderCursorSurvivesAutoRefresh(t *testing.T) {
	files := make([]string, 40)
	for i := range files {
		files[i] = fmt.Sprintf("dir/file_%02d.go", i)
	}
	m, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var model tea.Model = m
	model = send(model, tea.WindowSizeMsg{Width: 100, Height: 30})
	model = send(model, filesLoadedMsg{files: files})
	model = send(model, tea.KeyMsg{Type: tea.KeyCtrlP}) // focus finder

	for i := 0; i < 10; i++ {
		model = send(model, tea.KeyMsg{Type: tea.KeyDown})
	}
	before := model.(Model).finder.cursor
	if before != 10 {
		t.Fatalf("cursor should be 10 after 10 downs, got %d", before)
	}

	// An auto-refresh tick re-delivers the file list.
	model = send(model, filesLoadedMsg{files: files})
	if after := model.(Model).finder.cursor; after != before {
		t.Errorf("auto-refresh must not move the finder cursor: before=%d after=%d", before, after)
	}

	// Typing a query does reset the cursor to the top.
	model = send(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if got := model.(Model).finder.cursor; got != 0 {
		t.Errorf("typing a query should reset the cursor to 0, got %d", got)
	}
}
