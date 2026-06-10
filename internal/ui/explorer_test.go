package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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
	// Use loadRootCmd directly instead of Init() to avoid blocking on watchTickCmd.
	e, _ = e.Update(loadRootCmd(f, "/root")())
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
	e, _ = e.Update(loadRootCmd(f, "/root")())
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

// TestExplorerDeleteTyping verifies that typing() returns true while the
// explorer is waiting for a y/N confirmation, so that App.Update("q") does
// NOT quit the application during a delete prompt.
func TestExplorerDeleteTyping(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	if e.typing() {
		t.Fatal("typing() should be false in modeTree")
	}
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if e.mode != modeDelete {
		t.Fatalf("want modeDelete after 'D', got %v", e.mode)
	}
	if !e.typing() {
		t.Fatal("typing() must be true in modeDelete so that 'q' is not intercepted as quit")
	}
}

// TestAppQNotQuitInDeleteMode verifies that the App does not quit when 'q' is
// pressed while the explorer is in modeDelete (the delete confirmation prompt).
func TestAppQNotQuitInDeleteMode(t *testing.T) {
	dir := t.TempDir()
	a := NewApp(dir)
	// Load the explorer so we have an actual local tree with something to select.
	msgs := collectMsgs(a.Init())
	for _, m := range msgs {
		a.ex, _ = a.ex.Update(m)
	}
	// Press D to enter delete confirm mode.
	a.ex, _ = a.ex.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if a.ex.mode != modeDelete {
		t.Skip("no entries in temp dir, cannot enter modeDelete")
	}
	// Now send 'q' through the App. It must NOT return a quit command.
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd != nil {
		msg := cmd()
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			t.Fatal("App must not quit when 'q' is pressed during a delete confirmation prompt")
		}
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

// ---- Watch mode tests ----

// TestWatchTickOnReturnsRefreshBatch verifies that a watchTickMsg while watch
// is on returns a non-nil batch cmd that, when applied, delivers refreshedMsgs
// for root and each expanded dir.
func TestWatchTickOnReturnsRefreshBatch(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	// watch is on by default; expand docs so we have an expanded dir.
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter}) // expand docs
	e, _ = e.Update(collectMsgs(cmd)[0])               // apply childrenLoadedMsg

	// Inject watchTickMsg directly — do NOT execute watchTickCmd (blocks).
	_, cmd = e.Update(watchTickMsg{})
	if cmd == nil {
		t.Fatal("watch tick with watch=on should return a batch cmd")
	}
	// The batch must produce refreshedMsgs for root and the expanded dir.
	// We collect only the refreshedMsg results (non-blocking sub-cmds);
	// the watchTickCmd sub-cmd is skipped by checking only synchronous msgs.
	batchMsg := cmd()
	batch, ok := batchMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("want BatchMsg from watchTickMsg handler, got %T", batchMsg)
	}
	var refreshPaths []string
	for _, c := range batch {
		if c == nil {
			continue
		}
		// Execute each sub-cmd with a timeout: refreshCmds finish instantly;
		// we detect them by type assertion on the message.
		ch := make(chan tea.Msg, 1)
		go func(cmd tea.Cmd) { ch <- cmd() }(c)
		select {
		case msg := <-ch:
			if rm, ok := msg.(refreshedMsg); ok {
				refreshPaths = append(refreshPaths, rm.path)
			}
		case <-time.After(100 * time.Millisecond):
			// This is the watchTickCmd or similar long-running cmd — skip it.
		}
	}
	var hasRoot, hasDocs bool
	for _, p := range refreshPaths {
		if p == "/root" {
			hasRoot = true
		}
		if p == "/root/docs" {
			hasDocs = true
		}
	}
	if !hasRoot {
		t.Errorf("want root refresh in batch, got paths: %v", refreshPaths)
	}
	if !hasDocs {
		t.Errorf("want /root/docs refresh in batch (expanded dir), got paths: %v", refreshPaths)
	}
}

// TestWatchTickOffReturnsNil verifies that a watchTickMsg while watch is off
// returns nil (stopping the tick chain).
func TestWatchTickOff(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	e.watch = false
	_, cmd := e.Update(watchTickMsg{})
	if cmd != nil {
		t.Fatal("watch tick with watch=off should return nil cmd")
	}
}

// TestWatchRefreshedMsgMarksChanged verifies that a refreshedMsg with a
// changed mtime marks the node.
func TestWatchRefreshedMsgMarksChanged(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	// Deliver a refreshedMsg for the root with an updated mtime on a.txt.
	newEntries := []fs.Entry{
		{Name: "docs", Path: "/root/docs", IsDir: true},
		{Name: "a.txt", Path: "/root/a.txt", Size: 99, ModTime: time.Now()},
	}
	e, _ = e.Update(refreshedMsg{parent: nil, path: "/root", entries: newEntries})
	// Find a.txt in the tree and verify changedAt is set.
	var found *node
	for _, n := range e.tree.roots {
		if n.entry.Name == "a.txt" {
			found = n
		}
	}
	if found == nil {
		t.Fatal("a.txt not found after refresh")
	}
	if found.changedAt.IsZero() {
		t.Fatal("a.txt should have changedAt set after mtime change")
	}
}

// TestWatchRefreshErrorTurnsWatchOff verifies that a refreshedMsg with an
// error disables watch and sets an error status.
func TestWatchRefreshErrorTurnsWatchOff(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	e, cmd := e.Update(refreshedMsg{parent: nil, path: "/root", err: errors.New("connection lost")})
	if e.watch {
		t.Fatal("watch should be off after refresh error")
	}
	// The returned cmd should produce an error status.
	var sawErr bool
	for _, m := range collectMsgs(cmd) {
		if s, ok := m.(statusMsg); ok && s.isErr {
			sawErr = true
		}
	}
	if !sawErr {
		t.Fatal("want error status after refresh error")
	}
}

// TestWatchToggle verifies the w key toggles watch on/off.
func TestWatchToggle(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	if !e.watch {
		t.Fatal("watch should be on by default")
	}
	// Turn off.
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if e.watch {
		t.Fatal("watch should be off after 'w'")
	}
	// Turning off should produce a status msg (not a tick).
	msgs := collectMsgs(cmd)
	var sawStatus bool
	for _, m := range msgs {
		if s, ok := m.(statusMsg); ok && !s.isErr && strings.Contains(s.text, "watch off") {
			sawStatus = true
		}
	}
	if !sawStatus {
		t.Fatal("want 'watch off' status msg")
	}

	// Turn back on — should return a non-nil cmd (the tick).
	e, cmd = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if !e.watch {
		t.Fatal("watch should be on after second 'w'")
	}
	if cmd == nil {
		t.Fatal("turning watch on should return a cmd (the tick)")
	}
	// We do NOT execute cmd here to avoid blocking on the 3-second tick.
}

// TestWatchStaleRootRefreshIgnored verifies that a root refreshedMsg for an
// outdated path (the user navigated away) is silently dropped.
func TestWatchStaleRootRefreshIgnored(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	origLen := len(e.tree.roots)
	// Deliver a refresh for a different path — should be ignored.
	e, _ = e.Update(refreshedMsg{parent: nil, path: "/other", entries: []fs.Entry{
		{Name: "x.txt", Path: "/other/x.txt"},
	}})
	if len(e.tree.roots) != origLen {
		t.Fatalf("stale refresh should not modify tree; want %d roots, got %d", origLen, len(e.tree.roots))
	}
}

// ---- Send recent changes (C) tests ----

// TestSendRecentChangesNoChanges verifies that C with no recently-changed nodes
// returns a "no recent changes" status.
func TestSendRecentChangesNoChanges(t *testing.T) {
	t.Setenv("TMUX", "")
	f := newTestFS()
	e := loadedExplorer(t, f)
	// No nodes have changedAt set.
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	if cmd == nil {
		t.Fatal("C should return a cmd even with no changes")
	}
	msgs := collectMsgs(cmd)
	var sawNoChanges bool
	for _, m := range msgs {
		if s, ok := m.(statusMsg); ok && !s.isErr && strings.Contains(s.text, "no recent changes") {
			sawNoChanges = true
		}
	}
	if !sawNoChanges {
		t.Fatalf("want 'no recent changes' status, got %v", msgs)
	}
}

// TestSendRecentChangesWithChanges verifies that C sends all recently changed
// paths (most recent first, space-joined).
func TestSendRecentChangesWithChanges(t *testing.T) {
	t.Setenv("TMUX", "")
	f := newTestFS()
	e := loadedExplorer(t, f)

	// Manually set changedAt on both nodes so they appear as recent.
	now := time.Now()
	for _, n := range e.tree.roots {
		n.changedAt = now.Add(-5 * time.Second) // within 45s
	}
	// Make docs more recent than a.txt.
	e.tree.roots[0].changedAt = now.Add(-2 * time.Second) // docs — more recent
	e.tree.roots[1].changedAt = now.Add(-10 * time.Second) // a.txt — older

	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	if cmd == nil {
		t.Fatal("C should return a cmd when there are recent changes")
	}
	msgs := collectMsgs(cmd)
	var sentText string
	for _, m := range msgs {
		if s, ok := m.(statusMsg); ok && !s.isErr {
			sentText = s.text
		}
	}
	if sentText == "" {
		t.Fatalf("want a success status msg, got %v", msgs)
	}
	// Both paths should be present in the sent text.
	if !strings.Contains(sentText, "/root/docs") {
		t.Errorf("want /root/docs in sent text, got %q", sentText)
	}
	if !strings.Contains(sentText, "/root/a.txt") {
		t.Errorf("want /root/a.txt in sent text, got %q", sentText)
	}
}

// TestSendRecentChangesCap verifies that C caps at 20 paths.
func TestSendRecentChangesCap(t *testing.T) {
	t.Setenv("TMUX", "")
	// Build a fake FS with 25 files.
	f := &fakeFS{name: "local", listings: map[string][]fs.Entry{"/root": {}}}
	for i := 0; i < 25; i++ {
		f.listings["/root"] = append(f.listings["/root"], fs.Entry{
			Name: fmt.Sprintf("f%02d.txt", i),
			Path: fmt.Sprintf("/root/f%02d.txt", i),
		})
	}
	e := newExplorer(f, "/root")
	e, _ = e.Update(loadRootCmd(f, "/root")())

	// Mark all 25 as recently changed.
	now := time.Now()
	for i, n := range e.tree.roots {
		n.changedAt = now.Add(-time.Duration(i+1) * time.Second)
	}

	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	if cmd == nil {
		t.Fatal("C should return a cmd")
	}
	msgs := collectMsgs(cmd)
	var sentText string
	for _, m := range msgs {
		if s, ok := m.(statusMsg); ok && !s.isErr {
			sentText = s.text
		}
	}
	// Count the number of paths sent (each path separated by space).
	// The text ends with " " so split and remove trailing empty.
	parts := strings.Fields(sentText)
	// statusMsg text is "copied to clipboard: /root/f00.txt /root/f01.txt ..."
	// or "sent to agent pane: ...". Strip the prefix.
	var pathCount int
	for _, p := range parts {
		if strings.HasPrefix(p, "/root/") {
			pathCount++
		}
	}
	if pathCount > 20 {
		t.Errorf("C should cap at 20 paths, got %d", pathCount)
	}
}

// TestSendRecentChangesFilterControlChars verifies that paths with control
// characters are filtered out from the C send.
func TestSendRecentChangesFilterControlChars(t *testing.T) {
	t.Setenv("TMUX", "")
	f := newTestFS()
	e := loadedExplorer(t, f)

	now := time.Now()
	// Set a.txt's path to contain a control character by directly manipulating
	// the node entry.
	for _, n := range e.tree.roots {
		if n.entry.Name == "a.txt" {
			n.entry.Path = "/root/a\x01txt" // control char
			n.changedAt = now.Add(-5 * time.Second)
		}
		if n.entry.Name == "docs" {
			n.changedAt = now.Add(-3 * time.Second)
		}
	}

	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	if cmd == nil {
		t.Fatal("C should return a cmd")
	}
	msgs := collectMsgs(cmd)
	var sentText string
	for _, m := range msgs {
		if s, ok := m.(statusMsg); ok && !s.isErr {
			sentText = s.text
		}
	}
	// The control-char path should be excluded; docs should still be sent.
	if strings.Contains(sentText, "\x01") {
		t.Error("control-char path should be filtered from C send")
	}
	if !strings.Contains(sentText, "/root/docs") {
		t.Errorf("valid path /root/docs should be in sent text, got %q", sentText)
	}
}
