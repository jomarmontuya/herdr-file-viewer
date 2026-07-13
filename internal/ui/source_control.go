package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/jomarmontuya/herdr-file-viewer/internal/gitdiff"
	"github.com/jomarmontuya/herdr-file-viewer/internal/gitstatus"
)

type treeView int

const (
	treeViewFiles treeView = iota
	treeViewSourceControl
)

const (
	activityRailWidth = 4
	activityFilesRow  = 0
	activityGitRow    = 2
)

type sourceControlLine struct {
	heading    string
	change     gitstatus.Change
	mode       gitdiff.Mode
	selectable bool
}

func (m *Model) setSourceControlChanges(changes []gitstatus.Change) {
	selectedKey := ""
	if line, ok := m.selectedSourceControlLine(); ok {
		selectedKey = sourceControlKey(line)
	}
	m.sourceChanges = append(m.sourceChanges[:0], changes...)
	m.sourceControlLines = buildSourceControlLines(changes)
	m.sourceControlCursor = -1
	for i, line := range m.sourceControlLines {
		if !line.selectable {
			continue
		}
		if m.sourceControlCursor < 0 {
			m.sourceControlCursor = i
		}
		if selectedKey != "" && sourceControlKey(line) == selectedKey {
			m.sourceControlCursor = i
			break
		}
	}
}

func buildSourceControlLines(changes []gitstatus.Change) []sourceControlLine {
	staged := make([]gitstatus.Change, 0, len(changes))
	worktree := make([]gitstatus.Change, 0, len(changes))
	for _, change := range changes {
		if change.Staged() {
			staged = append(staged, change)
		}
		if change.Work != ' ' || change.Untracked() {
			worktree = append(worktree, change)
		}
	}
	lines := []sourceControlLine{{heading: fmt.Sprintf("Staged Changes (%d)", len(staged))}}
	for _, change := range staged {
		lines = append(lines, sourceControlLine{change: change, mode: gitdiff.ModeStaged, selectable: true})
	}
	lines = append(lines, sourceControlLine{heading: fmt.Sprintf("Changes (%d)", len(worktree))})
	for _, change := range worktree {
		mode := gitdiff.ModeWorktree
		if change.Untracked() {
			mode = gitdiff.ModeUntracked
		}
		lines = append(lines, sourceControlLine{change: change, mode: mode, selectable: true})
	}
	return lines
}

func sourceControlKey(line sourceControlLine) string {
	return string(line.mode) + "\x00" + line.change.Path
}

func (m Model) sourceControlCount() int { return len(m.sourceChanges) }

func (m Model) selectedSourceControlLine() (sourceControlLine, bool) {
	if m.sourceControlCursor < 0 || m.sourceControlCursor >= len(m.sourceControlLines) {
		return sourceControlLine{}, false
	}
	line := m.sourceControlLines[m.sourceControlCursor]
	return line, line.selectable
}

func (m *Model) moveSourceControl(delta int) {
	if len(m.sourceControlLines) == 0 || delta == 0 {
		return
	}
	for i := m.sourceControlCursor + delta; i >= 0 && i < len(m.sourceControlLines); i += delta {
		if m.sourceControlLines[i].selectable {
			m.sourceControlCursor = i
			return
		}
	}
}

func (m Model) sourceControlStart(height int) int {
	return scrollStart(m.sourceControlCursor, len(m.sourceControlLines), height)
}

func (m Model) renderSourceControl(width, height int) string {
	if len(m.sourceControlLines) == 0 {
		return lipgloss.NewStyle().Width(width).Height(height).Render(m.st.muted.Render("  loading changes…"))
	}
	start := m.sourceControlStart(height)
	var b strings.Builder
	for i := start; i < start+height && i < len(m.sourceControlLines); i++ {
		line := m.sourceControlLines[i]
		var rendered string
		if !line.selectable {
			rendered = lipgloss.NewStyle().Foreground(colText).Bold(true).Render("▾ " + line.heading)
		} else {
			letter := sourceControlLetter(line)
			color, _, decorated := gitDecor(sourceControlCode(letter))
			style := lipgloss.NewStyle().Foreground(colText)
			if decorated {
				style = lipgloss.NewStyle().Foreground(color)
			}
			rendered = "  " + style.Render(letter+" "+line.change.Path)
			if i == m.sourceControlCursor {
				rendered = m.st.rowSelected.Width(width).Render("▸ " + letter + " " + line.change.Path)
			}
		}
		b.WriteString(truncateLine(rendered, width))
		if i < start+height-1 && i < len(m.sourceControlLines)-1 {
			b.WriteByte('\n')
		}
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(b.String())
}

func sourceControlLetter(line sourceControlLine) string {
	status := line.change.Work
	if line.mode == gitdiff.ModeStaged {
		status = line.change.Index
	}
	if status == '?' {
		return "U"
	}
	if status == ' ' || status == 0 {
		return "M"
	}
	return string(status)
}

func sourceControlCode(letter string) gitstatus.Code {
	switch letter {
	case "U":
		return gitstatus.Untracked
	case "A":
		return gitstatus.Added
	case "D":
		return gitstatus.Deleted
	case "R":
		return gitstatus.Renamed
	case "!":
		return gitstatus.Conflicted
	default:
		return gitstatus.Modified
	}
}

func (m Model) renderActivityRail(width, height int) string {
	rows := make([]string, height)
	for i := range rows {
		rows[i] = "│" + strings.Repeat(" ", max(0, width-1))
	}
	setButton := func(row int, label string, active bool) {
		if row < 0 || row >= len(rows) {
			return
		}
		text := padRow("│"+label, width)
		if active {
			text = m.st.rowSelected.Width(width).Render(text)
		} else {
			text = m.st.muted.Render(text)
		}
		rows[row] = text
	}
	setButton(activityFilesRow, "[F]", m.treeView == treeViewFiles)
	setButton(activityGitRow, "[G]", m.treeView == treeViewSourceControl)
	if activityGitRow+1 < len(rows) && m.sourceControlCount() > 0 {
		count := fmt.Sprintf("%d", m.sourceControlCount())
		if len(count) > width-2 {
			count = "9+"
		}
		rows[activityGitRow+1] = m.st.countBadge.Render(padRow("│ "+count, width))
	}
	return strings.Join(rows, "\n")
}
