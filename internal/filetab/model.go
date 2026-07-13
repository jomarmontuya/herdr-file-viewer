// Package filetab provides the standalone read-only file pane opened in Herdr
// tabs by the explorer.
package filetab

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jomarmontuya/herdr-file-viewer/internal/viewer"
)

// Model is a full-pane wrapper around the shared read-only viewer.
type Model struct {
	viewer viewer.Model
	width  int
	height int
}

// New loads a regular file into a standalone tab model.
func New(path string) (Model, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Model{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Model{}, err
	}
	if info.IsDir() {
		return Model{}, fmt.Errorf("file tab path is a directory: %s", abs)
	}
	v := viewer.New()
	v.Load(abs)
	return Model{viewer: v}, nil
}

func (m Model) Init() tea.Cmd {
	if !m.viewer.ShouldHighlight() {
		return nil
	}
	path, source := m.viewer.Path(), m.viewer.Raw()
	return func() tea.Msg {
		return highlightedMsg{path: path, lines: viewer.Highlight(path, source)}
	}
}

type highlightedMsg struct {
	path  string
	lines []string
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case highlightedMsg:
		m.viewer.SetHighlighted(msg.path, msg.lines)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "m":
			m.viewer.ToggleMarkdown()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewer, cmd = m.viewer.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return m.viewer.View()
	}
	foot := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Width(m.width).
		Render(" q close · ↑↓ scroll · m markdown")
	return lipgloss.JoinVertical(lipgloss.Left, m.viewer.View(), strings.TrimSuffix(foot, "\n"))
}

func (m *Model) resize() {
	contentHeight := m.height - 1
	if contentHeight < 1 {
		contentHeight = 1
	}
	m.viewer.SetSize(m.width, contentHeight)
}
