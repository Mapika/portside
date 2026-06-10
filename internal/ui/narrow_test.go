package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/Mapika/portside/internal/fs"
)

// stripANSI removes ANSI escape sequences so we can check display widths.
// We use lipgloss.Width which already handles this, but we also need raw
// line splitting which can confuse escape codes.
func viewLines(v string) []string {
	return strings.Split(v, "\n")
}

// maxLineWidth returns the maximum display width across all lines of v,
// using lipgloss.Width (which strips ANSI) per line.
func maxLineWidth(v string) int {
	max := 0
	for _, line := range viewLines(v) {
		w := lipgloss.Width(line)
		if w > max {
			max = w
		}
	}
	return max
}

func TestExplorerNarrowRender18(t *testing.T) {
	// Build an explorer with long names and render at width 18.
	f := &fakeFS{name: "local", listings: map[string][]fs.Entry{
		"/home/user/projects": {
			{Name: "very-long-directory-name", Path: "/home/user/projects/very-long-directory-name", IsDir: true},
			{Name: "another-long-file-name.go", Path: "/home/user/projects/another-long-file-name.go"},
		},
	}}
	e := newExplorer(f, "/home/user/projects")
	e, _ = e.Update(loadRootCmd(f, "/home/user/projects")())
	e.width = 18
	e.height = 20

	v := e.View()

	// Each line must not exceed 18 display columns.
	for _, line := range viewLines(v) {
		w := lipgloss.Width(line)
		if w > 18 {
			t.Errorf("line exceeds 18 cols (got %d): %q", w, line)
		}
	}

	// At least one line should contain "…" indicating truncation happened.
	if !strings.Contains(v, "…") {
		t.Errorf("expected truncation ellipsis in narrow view:\n%s", v)
	}
}

func TestExplorerNarrowRenderZeroWidthNoTruncation(t *testing.T) {
	// width=0 means unconstrained — existing tests don't set WindowSizeMsg,
	// so width defaults to 0. View() should work without truncation.
	f := newTestFS()
	e := loadedExplorer(t, newTestFS())
	_ = f
	e.width = 0
	e.height = 20
	v := e.View()
	if !strings.Contains(v, "docs") || !strings.Contains(v, "a.txt") {
		t.Fatalf("zero-width view should render normally:\n%s", v)
	}
}

func TestExplorerTitleTruncatesPath(t *testing.T) {
	f := &fakeFS{name: "local", listings: map[string][]fs.Entry{
		"/very/long/path/to/some/directory": {},
	}}
	e := newExplorer(f, "/very/long/path/to/some/directory")
	e, _ = e.Update(loadRootCmd(f, "/very/long/path/to/some/directory")())
	e.width = 20
	e.height = 10
	v := e.View()
	firstLine := viewLines(v)[0]
	w := lipgloss.Width(firstLine)
	if w > 20 {
		t.Errorf("title line exceeds 20 cols (got %d): %q", w, firstLine)
	}
	if !strings.Contains(firstLine, "…") {
		t.Errorf("title should contain ellipsis for long path at width 20: %q", firstLine)
	}
}
