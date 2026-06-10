package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const ellipsis = "…"

// truncRight truncates s to at most w display columns, appending "…" when
// the string is cut. s must be a plain (unstyled) string. When w <= 0 no
// truncation is applied. When w == 1 the result is "…" (or the first rune if
// it fits). Returns s unchanged when it already fits.
func truncRight(s string, w int) string {
	if w <= 0 {
		return s
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	// Need to cut and append "…". The ellipsis itself is 1 column wide.
	if w <= 1 {
		return ellipsis
	}
	target := w - 1 // columns available for content before the ellipsis
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > target {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + ellipsis
}

// truncPathLeft truncates s from the left to at most w display columns,
// prepending "…" when cut. Useful for long path strings where the tail is
// more informative than the head. When w <= 0 no truncation is applied.
func truncPathLeft(s string, w int) string {
	if w <= 0 {
		return s
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	if w <= 1 {
		return ellipsis
	}
	target := w - 1 // columns for the kept tail
	runes := []rune(s)
	// Walk from the right until we would exceed target.
	used := 0
	start := len(runes)
	for start > 0 {
		r := runes[start-1]
		rw := lipgloss.Width(string(r))
		if used+rw > target {
			break
		}
		used += rw
		start--
	}
	return ellipsis + string(runes[start:])
}
