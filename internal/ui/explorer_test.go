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
	ops       []string // records Upload/Rename/Remove/Mkdir calls
	opErr     error    // if non-nil, returned by all op methods
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
func (f *fakeFS) Upload(localSrc, destDir string) error {
	f.ops = append(f.ops, "upload:"+localSrc+"→"+destDir)
	return f.opErr
}
func (f *fakeFS) Rename(oldPath, newName string) error {
	f.ops = append(f.ops, "rename:"+oldPath+"→"+newName)
	return f.opErr
}
func (f *fakeFS) Remove(path string) error {
	f.ops = append(f.ops, "remove:"+path)
	return f.opErr
}
func (f *fakeFS) Mkdir(path string) error {
	f.ops = append(f.ops, "mkdir:"+path)
	return f.opErr
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

func TestExplorerSendToClaude(t *testing.T) {
	t.Setenv("TMUX", "") // force the clipboard path: no tmux dependency in tests
	e := loadedExplorer(t, newTestFS())
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd == nil {
		t.Fatal("want a send command")
	}
	msgs := collectMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("want 1 msg, got %d", len(msgs))
	}
	s, ok := msgs[0].(statusMsg)
	if !ok || s.isErr {
		t.Fatalf("want success status, got %#v", msgs[0])
	}
	if !strings.Contains(s.text, "/root/docs") {
		t.Fatalf("status should name the sent path: %q", s.text)
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

// ---- File operation mode tests ----

func TestExplorerUploadMode(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	// cursor is on "docs" (a dir) — u should enter modeUpload
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if e.mode != modeUpload {
		t.Fatalf("want modeUpload after 'u', got %v", e.mode)
	}
	// type a path and press enter
	e.opInput.SetValue("/tmp/file.txt")
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if e.mode == modeUpload {
		t.Fatal("should have left modeUpload after enter")
	}
	msgs := collectMsgs(cmd)
	var gotOp bool
	for _, m := range msgs {
		if _, ok := m.(fileOpResultMsg); ok {
			gotOp = true
		}
	}
	if !gotOp {
		t.Fatalf("want fileOpResultMsg, got %v", msgs)
	}
	if len(f.ops) == 0 || f.ops[0][:6] != "upload" {
		t.Fatalf("want upload op recorded, got %v", f.ops)
	}
}

func TestExplorerUploadCancelWithEsc(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if e.mode != modeUpload {
		t.Fatal("want modeUpload")
	}
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if e.mode != modeTree {
		t.Fatalf("want modeTree after esc, got %v", e.mode)
	}
	if len(f.ops) != 0 {
		t.Fatal("esc should not perform any op")
	}
}

func TestExplorerRenameMode(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	// cursor on "docs"
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	if e.mode != modeRename {
		t.Fatalf("want modeRename after 'm', got %v", e.mode)
	}
	// input should be prefilled with the current name
	if e.opInput.Value() != "docs" {
		t.Fatalf("want opInput prefilled with 'docs', got %q", e.opInput.Value())
	}
	e.opInput.SetValue("newname")
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if e.mode == modeRename {
		t.Fatal("should have left modeRename after enter")
	}
	msgs := collectMsgs(cmd)
	var gotOp bool
	for _, m := range msgs {
		if _, ok := m.(fileOpResultMsg); ok {
			gotOp = true
		}
	}
	if !gotOp {
		t.Fatalf("want fileOpResultMsg, got %v", msgs)
	}
	if len(f.ops) == 0 || f.ops[0][:6] != "rename" {
		t.Fatalf("want rename op recorded, got %v", f.ops)
	}
}

func TestExplorerDeleteConfirmY(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	// D on "docs"
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if e.mode != modeDelete {
		t.Fatalf("want modeDelete after 'D', got %v", e.mode)
	}
	// confirm with "y"
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if e.mode == modeDelete {
		t.Fatal("should have left modeDelete after y")
	}
	msgs := collectMsgs(cmd)
	var gotOp bool
	for _, m := range msgs {
		if _, ok := m.(fileOpResultMsg); ok {
			gotOp = true
		}
	}
	if !gotOp {
		t.Fatalf("want fileOpResultMsg, got %v", msgs)
	}
	if len(f.ops) == 0 || f.ops[0][:6] != "remove" {
		t.Fatalf("want remove op recorded, got %v", f.ops)
	}
}

func TestExplorerDeleteConfirmReject(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if e.mode != modeDelete {
		t.Fatal("want modeDelete")
	}
	// any key other than y should cancel
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if e.mode != modeTree {
		t.Fatalf("want modeTree after rejection, got %v", e.mode)
	}
	if cmd != nil {
		for _, m := range collectMsgs(cmd) {
			if _, ok := m.(fileOpResultMsg); ok {
				t.Fatal("rejection should not produce a fileOpResultMsg")
			}
		}
	}
	if len(f.ops) != 0 {
		t.Fatal("rejection should not call remove")
	}
}

func TestExplorerMkdirMode(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if e.mode != modeMkdir {
		t.Fatalf("want modeMkdir after 'n', got %v", e.mode)
	}
	e.opInput.SetValue("myfolder")
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if e.mode == modeMkdir {
		t.Fatal("should have left modeMkdir after enter")
	}
	msgs := collectMsgs(cmd)
	var gotOp bool
	for _, m := range msgs {
		if _, ok := m.(fileOpResultMsg); ok {
			gotOp = true
		}
	}
	if !gotOp {
		t.Fatalf("want fileOpResultMsg, got %v", msgs)
	}
	if len(f.ops) == 0 || f.ops[0][:5] != "mkdir" {
		t.Fatalf("want mkdir op recorded, got %v", f.ops)
	}
}

func TestExplorerOpReloadsAfterSuccess(t *testing.T) {
	f := newTestFS()
	// expand docs first so it has children loaded
	e := loadedExplorer(t, f)
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand docs
	e, _ = e.Update(collectMsgs(cmd)[0])               // apply childrenLoadedMsg
	// docs is now expanded; its child b.md is at index 1
	docsNode := e.tree.roots[0]
	if !docsNode.loaded {
		t.Fatal("docs should be loaded")
	}
	// now rename docs
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	e.opInput.SetValue("docs2")
	e, cmd = e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// execute the fileOpResultMsg
	for _, m := range collectMsgs(cmd) {
		e, _ = e.Update(m)
	}
	// after success, should have reloaded root listing (docs has nil parent)
	if e.rootPath != "/root" {
		t.Fatalf("expected root reload, rootPath=%s", e.rootPath)
	}
}

func TestExplorerOpError(t *testing.T) {
	f := newTestFS()
	f.opErr = errors.New("permission denied")
	e := loadedExplorer(t, f)
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	e.opInput.SetValue("newname")
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
		t.Fatal("want error status msg on op failure")
	}
}
