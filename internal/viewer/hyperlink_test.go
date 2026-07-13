package viewer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTerminalHyperlinksWrapsHTTPURLs(t *testing.T) {
	got := terminalHyperlinks("visit https://example.com/docs?q=1")
	want := "\x1b]8;;https://example.com/docs?q=1\x1b\\https://example.com/docs?q=1\x1b]8;;\x1b\\"
	if !strings.Contains(got, want) {
		t.Fatalf("URL was not wrapped as an OSC 8 hyperlink:\n%q", got)
	}
}

func TestTerminalHyperlinksLeavesTrailingSentencePunctuationOutsideLink(t *testing.T) {
	got := terminalHyperlinks("see https://example.com/docs.")
	want := "\x1b]8;;https://example.com/docs\x1b\\https://example.com/docs\x1b]8;;\x1b\\."
	if !strings.Contains(got, want) {
		t.Fatalf("trailing punctuation should not be part of the URL:\n%q", got)
	}
}

func TestViewerRendersPlainURLsAsTerminalHyperlinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "links.txt")
	if err := os.WriteFile(path, []byte("open https://example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.Load(path)
	m.SetSize(80, 12)
	view := m.View()
	if !strings.Contains(view, "\x1b]8;;https://example.com\x1b\\https://example.com\x1b]8;;\x1b\\") {
		t.Fatalf("viewer did not render URL as terminal hyperlink:\n%q", view)
	}
}
