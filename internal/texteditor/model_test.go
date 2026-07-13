package texteditor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func press(m Model, keys ...tea.KeyMsg) Model {
	for _, key := range keys {
		m, _ = m.Update(key)
	}
	return m
}

func TestShiftArrowsSelectAndTypingReplacesSelection(t *testing.T) {
	m := New("alpha beta")
	for range 5 {
		m = press(m, tea.KeyMsg{Type: tea.KeyShiftRight})
	}

	if got := m.SelectedText(); got != "alpha" {
		t.Fatalf("shift+right selected %q, want alpha", got)
	}
	if m.SelectionLen() != 5 {
		t.Fatalf("selection length = %d, want 5", m.SelectionLen())
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if got := m.Value(); got != "X beta" {
		t.Fatalf("typing should replace selection, got %q", got)
	}
	if got := m.SelectedText(); got != "" {
		t.Fatalf("typing should clear selection, got %q", got)
	}
}

func TestCtrlAndOptionArrowsMoveByWordAndShiftExtendsSelection(t *testing.T) {
	m := New("alpha beta.gap")
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlRight})
	if got := m.Cursor(); got != 6 {
		t.Fatalf("ctrl+right cursor = %d, want next word at 6", got)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlShiftRight})
	if got := m.SelectedText(); got != "beta." {
		t.Fatalf("ctrl+shift+right selected %q, want beta.", got)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyShiftRight, Alt: true})
	if got := m.SelectedText(); got != "beta.gap" {
		t.Fatalf("option+shift+right selected %q, want beta.gap", got)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyLeft, Alt: true})
	if got := m.Cursor(); got != 6 {
		t.Fatalf("option+left cursor = %d, want 6", got)
	}
	if m.SelectionLen() != 0 {
		t.Fatal("movement without shift should clear the selection")
	}
}

func TestVerticalSelectionKeepsColumnAcrossLines(t *testing.T) {
	m := New("abc\n12345\nxy")
	m = press(m,
		tea.KeyMsg{Type: tea.KeyRight},
		tea.KeyMsg{Type: tea.KeyRight},
		tea.KeyMsg{Type: tea.KeyShiftDown},
	)
	if got := m.SelectedText(); got != "c\n12" {
		t.Fatalf("shift+down selected %q, want c\\n12", got)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyShiftDown})
	if got := m.SelectedText(); got != "c\n12345\nxy" {
		t.Fatalf("second shift+down selected %q", got)
	}
}

func TestHomeEndDocumentMovementAndSelectAll(t *testing.T) {
	m := New("first line\nsecond line")
	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlEnd})
	if got := m.Cursor(); got != len([]rune(m.Value())) {
		t.Fatalf("ctrl+end cursor = %d, want document end", got)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyHome})
	if got := m.Cursor(); got != len([]rune("first line\n")) {
		t.Fatalf("home cursor = %d, want second-line start", got)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyShiftEnd})
	if got := m.SelectedText(); got != "second line" {
		t.Fatalf("shift+end selected %q", got)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyCtrlA})
	if got := m.SelectedText(); got != m.Value() {
		t.Fatalf("ctrl+a selected %q, want entire document", got)
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyBackspace})
	if got := m.Value(); got != "" {
		t.Fatalf("backspace should delete selection, got %q", got)
	}
}

func TestSelectionIsRuneSafeAndVisibleInEditorView(t *testing.T) {
	m := New("a🙂 b")
	m.SetSize(30, 4)
	m = press(m,
		tea.KeyMsg{Type: tea.KeyShiftRight},
		tea.KeyMsg{Type: tea.KeyShiftRight},
	)

	if got := m.SelectedText(); got != "a🙂" {
		t.Fatalf("unicode selection = %q, want a🙂", got)
	}
	view := m.View()
	if !strings.Contains(view, "a") || !strings.Contains(view, "🙂") {
		t.Fatalf("editor view lost selected unicode text:\n%s", view)
	}
	if !strings.Contains(view, "2 selected") {
		t.Fatalf("editor view should expose visible selection state:\n%s", view)
	}
}

func TestUnshiftedArrowCollapsesSelectionAtTheExpectedEdge(t *testing.T) {
	m := New("abcd")
	m = press(m,
		tea.KeyMsg{Type: tea.KeyShiftRight},
		tea.KeyMsg{Type: tea.KeyShiftRight},
		tea.KeyMsg{Type: tea.KeyLeft},
	)
	if got := m.Cursor(); got != 0 {
		t.Fatalf("left should collapse selection to start, cursor = %d", got)
	}

	m = press(m,
		tea.KeyMsg{Type: tea.KeyShiftRight},
		tea.KeyMsg{Type: tea.KeyShiftRight},
		tea.KeyMsg{Type: tea.KeyRight},
	)
	if got := m.Cursor(); got != 2 {
		t.Fatalf("right should collapse selection to end, cursor = %d", got)
	}
}
