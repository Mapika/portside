package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mapika/portside/internal/fs"
)

type fakeFS struct {
	name      string
	listings  map[string][]fs.Entry
	downloads []string
}

func (f *fakeFS) Name() string          { return f.name }
func (f *fakeFS) Home() (string, error) { return "/home/u", nil }
func (f *fakeFS) List(path string) ([]fs.Entry, error) {
	e, ok := f.listings[path]
	if !ok {
		return nil, errors.New("no such dir: " + path)
	}
	return e, nil
}
func (f *fakeFS) Download(src, dest string) error {
	f.downloads = append(f.downloads, src+"→"+dest)
	return nil
}

func newTestFS() *fakeFS {
	return &fakeFS{name: "local", listings: map[string][]fs.Entry{
		"/root": {
			{Name: "docs", Path: "/root/docs", IsDir: true},
			{Name: "a.txt", Path: "/root/a.txt"},
		},
		"/root/docs": {{Name: "b.md", Path: "/root/docs/b.md"}},
	}}
}

// collectMsgs executes a cmd (flattening batches) and returns all messages.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, collectMsgs(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func loadedExplorer(t *testing.T, f *fakeFS) explorer {
	t.Helper()
	e := newExplorer(f, "/root")
	e, _ = e.Update(e.Init()())
	if len(e.tree.visible()) != 2 {
		t.Fatalf("setup: want 2 visible, got %d", len(e.tree.visible()))
	}
	return e
}

func TestExplorerExpandDirectory(t *testing.T) {
	e := loadedExplorer(t, newTestFS())
	// cursor starts on "docs"; enter triggers an async children load
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("want a load command")
	}
	e, _ = e.Update(collectMsgs(cmd)[0])
	vis := e.tree.visible()
	if len(vis) != 3 || vis[1].entry.Name != "b.md" {
		t.Fatalf("want docs expanded with b.md, got %d rows", len(vis))
	}
	// enter again collapses without reloading
	e, cmd = e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("collapse should not produce a command")
	}
	if len(e.tree.visible()) != 2 {
		t.Fatal("want collapsed back to 2 rows")
	}
}

func TestExplorerPathBar(t *testing.T) {
	e := loadedExplorer(t, newTestFS())
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	if e.mode != modePath {
		t.Fatal("want path mode after ':'")
	}
	e.pathInput.SetValue("/root/docs")
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for _, m := range collectMsgs(cmd) {
		e, _ = e.Update(m)
	}
	if e.rootPath != "/root/docs" {
		t.Fatalf("want root /root/docs, got %s", e.rootPath)
	}
	if len(e.tree.visible()) != 1 || e.tree.visible()[0].entry.Name != "b.md" {
		t.Fatal("tree should show docs contents")
	}
}

func TestExplorerPathBarBadPathKeepsOldTree(t *testing.T) {
	e := loadedExplorer(t, newTestFS())
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	e.pathInput.SetValue("/nope")
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	var sawErr bool
	for _, m := range collectMsgs(cmd) {
		var cmd2 tea.Cmd
		e, cmd2 = e.Update(m)
		for _, m2 := range collectMsgs(cmd2) {
			if s, ok := m2.(statusMsg); ok && s.isErr {
				sawErr = true
			}
		}
	}
	if !sawErr {
		t.Fatal("want an error status for a bad path")
	}
	if len(e.tree.visible()) != 2 {
		t.Fatal("old tree should remain on failed load")
	}
}

func TestExplorerDownload(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyDown}) // move to a.txt
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if e.mode != modeDownload {
		t.Fatal("want download mode after 'd'")
	}
	e.destInput.SetValue("/tmp/dl")
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	var done bool
	for _, m := range collectMsgs(cmd) {
		if _, ok := m.(downloadResultMsg); ok {
			done = true
		}
	}
	if !done {
		t.Fatal("want a downloadResultMsg")
	}
	if len(f.downloads) != 1 || f.downloads[0] != "/root/a.txt→/tmp/dl" {
		t.Fatalf("wrong download call: %v", f.downloads)
	}
}

func TestExplorerViewRendersTree(t *testing.T) {
	e := loadedExplorer(t, newTestFS())
	e.height = 20
	v := e.View()
	if !strings.Contains(v, "docs") || !strings.Contains(v, "a.txt") {
		t.Fatalf("view missing entries:\n%s", v)
	}
}

func TestExplorerMouseClickOnScrolledTree(t *testing.T) {
	f := &fakeFS{name: "local", listings: map[string][]fs.Entry{"/root": {}}}
	for i := 0; i < 20; i++ {
		f.listings["/root"] = append(f.listings["/root"], fs.Entry{
			Name: fmt.Sprintf("f%02d.txt", i),
			Path: fmt.Sprintf("/root/f%02d.txt", i),
		})
	}
	e := newExplorer(f, "/root")
	e, _ = e.Update(e.Init()())
	e.height = 12 // maxRows = 10

	// scroll down to row 15 → window starts at 6
	for i := 0; i < 15; i++ {
		e, _ = e.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if e.tree.cursor != 15 {
		t.Fatalf("setup: want cursor 15, got %d", e.tree.cursor)
	}

	// click the first content row (Y=1) → should select vis[6], not vis[0]
	e, _ = e.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: 1})
	if e.tree.cursor != 6 {
		t.Fatalf("want cursor 6 after click, got %d", e.tree.cursor)
	}
	if e.tree.current().entry.Name != "f06.txt" {
		t.Fatalf("want f06.txt selected, got %s", e.tree.current().entry.Name)
	}
}
