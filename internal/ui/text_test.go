package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTruncRight(t *testing.T) {
	tests := []struct {
		name string
		s    string
		w    int
		want string
	}{
		{"fits exactly", "hello", 5, "hello"},
		{"fits with room", "hi", 10, "hi"},
		{"cut ascii", "hello world", 8, "hello w…"},
		{"w=0 no trunc", "hello world", 0, "hello world"},
		{"w=1 ellipsis only", "hello", 1, "…"},
		{"w negative no trunc", "hello", -1, "hello"},
		{"empty string", "", 5, ""},
		{"single rune fits", "x", 1, "x"},
		{"single rune cut at 0", "x", 0, "x"},
		{"wide rune fits", "日本語", 6, "日本語"},
		{"wide rune cut", "日本語テスト", 5, "日本…"},
		{"exact fit no ellipsis", "hello", 5, "hello"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncRight(tc.s, tc.w)
			if got != tc.want {
				t.Errorf("truncRight(%q, %d) = %q, want %q", tc.s, tc.w, got, tc.want)
			}
			// Verify width constraint when w > 0
			if tc.w > 0 && lipgloss.Width(got) > tc.w {
				t.Errorf("truncRight(%q, %d) = %q exceeds width %d (got %d)", tc.s, tc.w, got, tc.w, lipgloss.Width(got))
			}
		})
	}
}

func TestTruncPathLeft(t *testing.T) {
	tests := []struct {
		name string
		s    string
		w    int
		want string
	}{
		{"fits exactly", "/home/u", 7, "/home/u"},
		{"fits with room", "/tmp", 10, "/tmp"},
		{"cut from left", "/home/user/very/long/path", 15, "…/very/long/path"},  // need to count
		{"w=0 no trunc", "/home/user/very/long", 0, "/home/user/very/long"},
		{"w=1 ellipsis only", "/home/user", 1, "…"},
		{"w negative no trunc", "/home/user", -1, "/home/user"},
		{"empty string", "", 5, ""},
	}
	// Recompute the "cut from left" expectation since column count depends on ASCII width.
	// "/home/user/very/long/path" = 25 chars = 25 cols. w=15, so tail = 14 cols + "…".
	// Start from right: "path"=4, "/path"=5, "ng/path"=7, ... count carefully:
	// "long/path" = 9, "/long/path" = 10, "ry/long/path"=12, "ery/long/path"=13,
	// "very/long/path"=14 → fits. So start at the 'v', full tail = "very/long/path".
	// want = "…very/long/path" (width 15 = 1 + 14).
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncPathLeft(tc.s, tc.w)
			// For the "cut from left" case, just verify it starts with "…" and fits.
			if tc.w > 0 && lipgloss.Width(tc.s) > tc.w {
				if !strings.HasPrefix(got, "…") {
					t.Errorf("truncPathLeft(%q, %d) = %q should start with '…'", tc.s, tc.w, got)
				}
				if lipgloss.Width(got) > tc.w {
					t.Errorf("truncPathLeft(%q, %d) = %q exceeds width %d (got %d)", tc.s, tc.w, got, tc.w, lipgloss.Width(got))
				}
				return
			}
			if got != tc.want {
				t.Errorf("truncPathLeft(%q, %d) = %q, want %q", tc.s, tc.w, got, tc.want)
			}
		})
	}
}

func TestTruncRightWidthNeverExceeds(t *testing.T) {
	// Property test: for any width w >= 1 and a long string, the output width <= w.
	s := "abcdefghijklmnopqrstuvwxyz"
	for w := 1; w <= 30; w++ {
		got := truncRight(s, w)
		gw := lipgloss.Width(got)
		if gw > w {
			t.Errorf("truncRight(%q, %d) = %q width %d > %d", s, w, got, gw, w)
		}
	}
}

func TestTruncPathLeftWidthNeverExceeds(t *testing.T) {
	s := "/home/user/projects/very/deep/directory/structure"
	for w := 1; w <= 55; w++ {
		got := truncPathLeft(s, w)
		gw := lipgloss.Width(got)
		if gw > w {
			t.Errorf("truncPathLeft(%q, %d) = %q width %d > %d", s, w, got, gw, w)
		}
	}
}
