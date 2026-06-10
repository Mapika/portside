package ui

import "testing"

func TestParseRightPane(t *testing.T) {
	out := "%0 0\n%1 12\n%2 80\n"
	got, err := parseRightPane(out, "%0")
	if err != nil || got != "%1" {
		t.Fatalf("want %%1 (nearest right), got %q err=%v", got, err)
	}
	got, err = parseRightPane(out, "%1")
	if err != nil || got != "%2" {
		t.Fatalf("want %%2, got %q err=%v", got, err)
	}
	if _, err := parseRightPane(out, "%2"); err == nil {
		t.Fatal("rightmost pane should have no right neighbor")
	}
	if _, err := parseRightPane(out, "%9"); err == nil {
		t.Fatal("unknown own pane should error")
	}
	if _, err := parseRightPane("", "%0"); err == nil {
		t.Fatal("empty output should error")
	}
}
