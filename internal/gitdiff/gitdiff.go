// Package gitdiff produces a parsed, structured diff of a single file against
// HEAD — the data behind the "review" view. Like gitstatus it shells out to
// `git` and degrades gracefully: outside a repo, or with no changes, it returns
// an empty diff rather than an error.
package gitdiff

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// LineKind classifies a rendered diff line.
type LineKind int

const (
	Context LineKind = iota
	Add
	Del
	Hunk       // the "@@ … @@" header
	FileHeader // a per-file header in a multi-file (commit) diff
)

// Line is one row of the parsed diff.
type Line struct {
	Kind   LineKind
	OldNum int // 1-based line number in the old file, 0 when not applicable
	NewNum int // 1-based line number in the new file, 0 when not applicable
	Text   string
}

// FileDiff is the full parsed diff for one file.
type FileDiff struct {
	Path    string
	Lines   []Line
	Added   int
	Removed int
	Binary  bool
	Empty   bool // no differences
}

// Mode selects which two Git states a review compares.
type Mode string

const (
	ModeHead      Mode = "head"      // HEAD -> working tree (legacy review)
	ModeStaged    Mode = "staged"    // HEAD -> index
	ModeWorktree  Mode = "worktree"  // index -> working tree
	ModeUntracked Mode = "untracked" // empty file -> untracked file
)

// Valid reports whether mode can be loaded by LoadMode.
func (m Mode) Valid() bool {
	switch m {
	case ModeHead, ModeStaged, ModeWorktree, ModeUntracked:
		return true
	default:
		return false
	}
}

var hunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// Load returns the diff of the file at rel (a slash path relative to root)
// against HEAD. Untracked files have no HEAD entry, so they are diffed against
// an empty file (every line shows as an addition).
func Load(ctx context.Context, root, rel string, untracked bool) FileDiff {
	mode := ModeHead
	if untracked {
		mode = ModeUntracked
	}
	return LoadMode(ctx, root, rel, mode)
}

// LoadMode returns a file diff for one precise Source Control boundary. This is
// important for partially staged files: their staged and worktree reviews are
// intentionally different tabs.
func LoadMode(ctx context.Context, root, rel string, mode Mode) FileDiff {
	fd := FileDiff{Path: rel, Empty: true}

	abs, err := filepath.Abs(root)
	if err != nil {
		return fd
	}

	var cmd *exec.Cmd
	switch mode {
	case ModeUntracked:
		file := filepath.Join(abs, filepath.FromSlash(rel))
		// --no-index always exits non-zero when files differ; that's expected.
		cmd = exec.CommandContext(ctx, "git", "-C", abs,
			"diff", "--no-color", "--no-index", "--", "/dev/null", file)
	case ModeStaged:
		cmd = exec.CommandContext(ctx, "git", "-C", abs,
			"diff", "--cached", "--no-color", "--", rel)
	case ModeWorktree:
		cmd = exec.CommandContext(ctx, "git", "-C", abs,
			"diff", "--no-color", "--", rel)
	case ModeHead:
		cmd = exec.CommandContext(ctx, "git", "-C", abs,
			"diff", "--no-color", "HEAD", "--", rel)
	default:
		return fd
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	// Ignore the error: `git diff --no-index` uses exit code 1 to mean "differs",
	// and a missing repo just yields empty output we handle below.
	_ = cmd.Run()

	return parse(fd, out.Bytes())
}

// LoadRef returns the diff introduced by a commit (or any ref), across all the
// files it touched. label is what the review header should show (e.g. the short
// hash and subject).
func LoadRef(ctx context.Context, root, ref, label string) FileDiff {
	fd := FileDiff{Path: label, Empty: true}
	abs, err := filepath.Abs(root)
	if err != nil {
		return fd
	}
	cmd := exec.CommandContext(ctx, "git", "-C", abs, "show", "--no-color", ref)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return fd
	}
	return parse(fd, out.Bytes())
}

// cleanDiffPath strips the "a/" or "b/" prefix from a diff header path and maps
// /dev/null (added/deleted counterpart) to empty.
func cleanDiffPath(s string) string {
	s = strings.TrimSpace(s)
	if s == "/dev/null" {
		return ""
	}
	s = strings.TrimPrefix(s, "b/")
	s = strings.TrimPrefix(s, "a/")
	return s
}

func parse(fd FileDiff, data []byte) FileDiff {
	if len(data) == 0 {
		return fd
	}
	lines := strings.Split(string(data), "\n")
	oldNum, newNum := 0, 0
	inHunk := false

	for _, raw := range lines {
		switch {
		case strings.HasPrefix(raw, "Binary files"):
			fd.Binary = true

		case strings.HasPrefix(raw, "diff --git "):
			inHunk = false // a new file section begins

		case strings.HasPrefix(raw, "+++ "):
			if p := cleanDiffPath(raw[4:]); p != "" {
				fd.Lines = append(fd.Lines, Line{Kind: FileHeader, Text: p})
				fd.Empty = false
			}

		case strings.HasPrefix(raw, "--- "):
			// old-file header — the "+++" line carries the path we show

		case strings.HasPrefix(raw, "@@"):
			if m := hunkRe.FindStringSubmatch(raw); m != nil {
				oldNum, _ = strconv.Atoi(m[1])
				newNum, _ = strconv.Atoi(m[2])
				inHunk = true
				fd.Empty = false
				fd.Lines = append(fd.Lines, Line{Kind: Hunk, Text: raw})
			}

		case !inHunk:
			// File-level headers (diff, index, ---, +++). Skip.
			continue

		case strings.HasPrefix(raw, "+"):
			fd.Lines = append(fd.Lines, Line{Kind: Add, NewNum: newNum, Text: raw[1:]})
			newNum++
			fd.Added++

		case strings.HasPrefix(raw, "-"):
			fd.Lines = append(fd.Lines, Line{Kind: Del, OldNum: oldNum, Text: raw[1:]})
			oldNum++
			fd.Removed++

		case strings.HasPrefix(raw, "\\"):
			// "\ No newline at end of file" — not a content line.
			continue

		case strings.HasPrefix(raw, " "):
			fd.Lines = append(fd.Lines, Line{Kind: Context, OldNum: oldNum, NewNum: newNum, Text: raw[1:]})
			oldNum++
			newNum++
		}
	}
	return fd
}
