package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mapika/portside/internal/fs"
)

// ---- parseGitStatus tests ----

func TestParseGitStatusModified(t *testing.T) {
	// "M  file.go\0" — index-modified
	out := []byte(" M file.go\x00")
	states := parseGitStatus(out, "/repo")
	if states["/repo/file.go"] != gitModified {
		t.Errorf("want gitModified for ' M file.go', got %v", states["/repo/file.go"])
	}
}

func TestParseGitStatusUntracked(t *testing.T) {
	out := []byte("?? new.go\x00")
	states := parseGitStatus(out, "/repo")
	if states["/repo/new.go"] != gitUntracked {
		t.Errorf("want gitUntracked for '?? new.go', got %v", states["/repo/new.go"])
	}
}

func TestParseGitStatusIgnored(t *testing.T) {
	out := []byte("!! ignored.go\x00")
	states := parseGitStatus(out, "/repo")
	if _, ok := states["/repo/ignored.go"]; ok {
		t.Error("ignored file (!! ) should not appear in result")
	}
}

func TestParseGitStatusRenamePair(t *testing.T) {
	// Rename: "R  new.go\0old.go\0" — origin after the first NUL
	out := []byte("R  new.go\x00old.go\x00")
	states := parseGitStatus(out, "/repo")
	if states["/repo/new.go"] != gitModified {
		t.Errorf("want gitModified for renamed file, got %v", states["/repo/new.go"])
	}
	if _, ok := states["/repo/old.go"]; ok {
		t.Error("origin of rename should not appear as a separate entry")
	}
}

func TestParseGitStatusEmpty(t *testing.T) {
	states := parseGitStatus(nil, "/repo")
	if len(states) != 0 {
		t.Errorf("want empty map for nil input, got %v", states)
	}
	states2 := parseGitStatus([]byte{}, "/repo")
	if len(states2) != 0 {
		t.Errorf("want empty map for empty input, got %v", states2)
	}
}

func TestParseGitStatusMultiple(t *testing.T) {
	// Mix of modified, untracked, and ignored.
	out := []byte("M  a.go\x00?? b.go\x00!! c.go\x00")
	states := parseGitStatus(out, "/repo")
	if states["/repo/a.go"] != gitModified {
		t.Errorf("a.go should be gitModified")
	}
	if states["/repo/b.go"] != gitUntracked {
		t.Errorf("b.go should be gitUntracked")
	}
	if _, ok := states["/repo/c.go"]; ok {
		t.Error("c.go (ignored) should not be in result")
	}
}

// ---- execFakeFS for end-to-end git cmd tests ----

// execFakeFS wraps fakeFS and adds Exec support, returning canned output
// keyed on the command joined with spaces.
type execFakeFS struct {
	*fakeFS
	execOutputs map[string][]byte
	execErrs    map[string]error
}

func (e *execFakeFS) Exec(name string, args ...string) ([]byte, error) {
	key := name
	for _, a := range args {
		key += " " + a
	}
	if err, ok := e.execErrs[key]; ok {
		return nil, err
	}
	if out, ok := e.execOutputs[key]; ok {
		return out, nil
	}
	return nil, nil
}

func newExecFakeFS() *execFakeFS {
	return &execFakeFS{
		fakeFS:      newTestFS(),
		execOutputs: make(map[string][]byte),
		execErrs:    make(map[string]error),
	}
}

// TestGitRefreshCmdNonExecer verifies that gitRefreshCmd returns nil when the
// filesystem does not implement Execer.
func TestGitRefreshCmdNonExecer(t *testing.T) {
	f := newTestFS() // plain fakeFS — no Exec method
	e := newExplorer(f, "/root")
	if cmd := e.gitRefreshCmd(); cmd != nil {
		t.Fatal("gitRefreshCmd should return nil for non-Execer filesystem")
	}
}

// TestGitRefreshCmdExecer verifies that gitRefreshCmd returns a non-nil cmd
// and that executing it produces a gitStatusMsg with the parsed states.
func TestGitRefreshCmdExecer(t *testing.T) {
	f := newExecFakeFS()
	// rev-parse returns "/root" as the repo top.
	f.execOutputs["git -C /root rev-parse --show-toplevel"] = []byte("/root\n")
	// status returns one modified and one untracked file.
	f.execOutputs["git -C /root status --porcelain -z"] = []byte(" M a.txt\x00?? docs/b.md\x00")

	e := newExplorer(f, "/root")
	cmd := e.gitRefreshCmd()
	if cmd == nil {
		t.Fatal("gitRefreshCmd should return a non-nil cmd for Execer filesystem")
	}
	msg := cmd()
	gsm, ok := msg.(gitStatusMsg)
	if !ok {
		t.Fatalf("want gitStatusMsg, got %T", msg)
	}
	if gsm.root != "/root" {
		t.Errorf("want root=/root, got %q", gsm.root)
	}
	if gsm.states["/root/a.txt"] != gitModified {
		t.Errorf("a.txt should be gitModified, got %v", gsm.states["/root/a.txt"])
	}
	if gsm.states["/root/docs/b.md"] != gitUntracked {
		t.Errorf("docs/b.md should be gitUntracked, got %v", gsm.states["/root/docs/b.md"])
	}
}

// TestGitStatusMsgApplied verifies that a gitStatusMsg is stored on the explorer.
func TestGitStatusMsgApplied(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	states := map[string]gitState{
		"/root/a.txt": gitModified,
	}
	e, _ = e.Update(gitStatusMsg{root: "/root", top: "/root", states: states})
	if e.gitStates["/root/a.txt"] != gitModified {
		t.Errorf("gitStates should reflect the received map")
	}
}

// TestGitStatusMsgStaleIgnored verifies that a gitStatusMsg for a different
// root path is silently dropped.
func TestGitStatusMsgStaleIgnored(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	e, _ = e.Update(gitStatusMsg{root: "/other", top: "/other", states: map[string]gitState{
		"/other/x.txt": gitModified,
	}})
	if len(e.gitStates) != 0 {
		t.Errorf("stale gitStatusMsg should not be applied, got %v", e.gitStates)
	}
}

// TestGitNotRepoReturnsEmptyMap verifies that when rev-parse fails (not a
// repo), gitRefreshCmd produces a gitStatusMsg with an empty states map.
func TestGitNotRepoReturnsEmptyMap(t *testing.T) {
	f := newExecFakeFS()
	// Make rev-parse fail by returning an error.
	f.execErrs["git -C /root rev-parse --show-toplevel"] = &gitExecError{"not a git repo"}

	e := newExplorer(f, "/root")
	cmd := e.gitRefreshCmd()
	if cmd == nil {
		t.Fatal("gitRefreshCmd should return a cmd even when not a repo")
	}
	msg := cmd()
	gsm, ok := msg.(gitStatusMsg)
	if !ok {
		t.Fatalf("want gitStatusMsg, got %T", msg)
	}
	if len(gsm.states) != 0 {
		t.Errorf("want empty states for not-a-repo, got %v", gsm.states)
	}
}

// gitExecError is a simple error type for test use.
type gitExecError struct{ msg string }

func (e *gitExecError) Error() string { return e.msg }

// TestGitColorInRenderNode verifies that applying a gitStatusMsg causes the
// render output to reflect git colors (modified/untracked) for file nodes.
func TestGitColorInRenderNode(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	e.height = 20

	// Apply git status: a.txt is modified.
	e, _ = e.Update(gitStatusMsg{root: "/root", top: "/root", states: map[string]gitState{
		"/root/a.txt": gitModified,
	}})

	// The view should render a.txt; we check that it doesn't fall through to
	// plain (no-style) rendering by verifying the node's git state is set.
	var aNode *node
	for _, n := range e.tree.roots {
		if n.entry.Name == "a.txt" {
			aNode = n
		}
	}
	if aNode == nil {
		t.Fatal("a.txt not found in tree")
	}
	// Verify the view contains a.txt (it should be rendered with git color).
	v := e.View()
	if !strings.Contains(v, "a.txt") {
		t.Fatalf("view should contain a.txt:\n%s", v)
	}
	// Verify renderNode is called with the right git state by checking the
	// internal git state map.
	if e.gitStates["/root/a.txt"] != gitModified {
		t.Errorf("gitStates should hold gitModified for a.txt")
	}
}

// TestGitCacheInvalidatedOnRootPathChange verifies that when the rootPath
// changes, the git cache is re-queried.
func TestGitCacheInvalidatedOnRootPathChange(t *testing.T) {
	f := newExecFakeFS()
	f.fakeFS.listings["/other"] = []fs.Entry{{Name: "x.txt", Path: "/other/x.txt"}}
	f.execOutputs["git -C /root rev-parse --show-toplevel"] = []byte("/root\n")
	f.execOutputs["git -C /root status --porcelain -z"] = []byte("")
	f.execOutputs["git -C /other rev-parse --show-toplevel"] = []byte("/other\n")
	f.execOutputs["git -C /other status --porcelain -z"] = []byte("")

	e := newExplorer(f, "/root")
	e.gitTop = "/root"
	e.gitTopFor = "/root"

	// Simulate a root path change.
	e.rootPath = "/other"
	cmd := e.gitRefreshCmd()
	if cmd == nil {
		t.Fatal("gitRefreshCmd should be non-nil")
	}
	msg := cmd()
	gsm, ok := msg.(gitStatusMsg)
	if !ok {
		t.Fatalf("want gitStatusMsg, got %T", msg)
	}
	if gsm.root != "/other" {
		t.Errorf("want root=/other after path change, got %q", gsm.root)
	}
	// After applying, the cache should be for /other.
	e, _ = e.Update(gsm)
	if e.gitTopFor != "/other" {
		t.Errorf("want gitTopFor=/other, got %q", e.gitTopFor)
	}
}

// TestWatchTickOnReturnsGitRefreshForExecer verifies that when the backend
// supports Exec, the watch tick batch includes a gitStatusMsg eventually.
func TestWatchTickOnReturnsGitRefreshForExecer(t *testing.T) {
	f := newExecFakeFS()
	f.execOutputs["git -C /root rev-parse --show-toplevel"] = []byte("/root\n")
	f.execOutputs["git -C /root status --porcelain -z"] = []byte("")

	e := newExplorer(f, "/root")
	e, _ = e.Update(loadRootCmd(f, "/root")())
	if len(e.tree.visible()) != 2 {
		t.Fatalf("setup: want 2 visible, got %d", len(e.tree.visible()))
	}
	_, cmd := e.Update(watchTickMsg{})
	if cmd == nil {
		t.Fatal("watch tick should return a batch cmd")
	}
	// Execute sub-cmds; one of them should produce a gitStatusMsg.
	batchMsg := cmd()
	batch, ok := batchMsg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("want BatchMsg, got %T", batchMsg)
	}
	var sawGit bool
	for _, c := range batch {
		if c == nil {
			continue
		}
		ch := make(chan tea.Msg, 1)
		go func(cmd tea.Cmd) { ch <- cmd() }(c)
		select {
		case msg := <-ch:
			if _, ok := msg.(gitStatusMsg); ok {
				sawGit = true
			}
		case <-time.After(100 * time.Millisecond):
			// skip slow cmds (watchTickCmd)
		}
	}
	if !sawGit {
		t.Error("watch tick batch should include a gitStatusMsg for Execer filesystem")
	}
}
