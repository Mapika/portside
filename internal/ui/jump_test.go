package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mapika/portside/internal/fs"
)

// ---- respawnArgv pure builder tests ----

func TestRespawnArgvLocal(t *testing.T) {
	argv := respawnArgv("local", "", "/srv/my app", "claude", "%7")
	want := []string{"tmux", "respawn-pane", "-k", "-t", "%7", "-c", "/srv/my app", "claude"}
	if len(argv) != len(want) {
		t.Fatalf("local argv: want %v, got %v", want, argv)
	}
	for i, v := range want {
		if argv[i] != v {
			t.Errorf("local argv[%d]: want %q, got %q", i, v, argv[i])
		}
	}
}

func TestRespawnArgvLocalWithSingleQuote(t *testing.T) {
	// dir with single quote in name
	argv := respawnArgv("local", "", "/srv/mark's app", "claude", "%7")
	want := []string{"tmux", "respawn-pane", "-k", "-t", "%7", "-c", "/srv/mark's app", "claude"}
	if len(argv) != len(want) {
		t.Fatalf("local argv with quote: want %v, got %v", want, argv)
	}
	for i, v := range want {
		if argv[i] != v {
			t.Errorf("local argv[%d]: want %q, got %q", i, v, argv[i])
		}
	}
}

func TestRespawnArgvRemote(t *testing.T) {
	// host="web", dir="/srv/my app", agent="claude"
	// argv = ["tmux","respawn-pane","-k","-t","%7", cmdstring]  — 6 elements
	argv := respawnArgv("web", "web", "/srv/my app", "claude", "%7")
	if len(argv) != 6 {
		t.Fatalf("remote argv: want 6 elements, got %d: %v", len(argv), argv)
	}
	if argv[0] != "tmux" || argv[1] != "respawn-pane" || argv[2] != "-k" || argv[3] != "-t" || argv[4] != "%7" {
		t.Fatalf("remote argv prefix wrong: %v", argv)
	}
	cmdstring := argv[5]
	// Must start with ssh -t 'web' --
	if !strings.HasPrefix(cmdstring, "ssh -t 'web' -- ") {
		t.Errorf("cmdstring should start with ssh -t 'web' -- , got: %q", cmdstring)
	}
	// Must contain the dir
	if !strings.Contains(cmdstring, "/srv/my app") {
		t.Errorf("cmdstring should contain dir, got: %q", cmdstring)
	}
	// Must contain bash -lc
	if !strings.Contains(cmdstring, "bash -lc") {
		t.Errorf("cmdstring should contain bash -lc, got: %q", cmdstring)
	}
	// Must contain exec claude
	if !strings.Contains(cmdstring, "exec claude") {
		t.Errorf("cmdstring should contain exec claude, got: %q", cmdstring)
	}
}

func TestRespawnArgvRemoteLiteralQuoting(t *testing.T) {
	// host="web", dir="/srv/my app", agent="claude"
	// Verify the exact cmdstring byte-for-byte (argv[5]).
	argv := respawnArgv("web", "web", "/srv/my app", "claude", "%7")
	cmdstring := argv[5]

	// inner = "cd '/srv/my app' && exec claude"
	inner := "cd " + shq("/srv/my app") + " && exec claude"
	// bashCmd = "bash -lc " + shq(inner)
	bashCmd := "bash -lc " + shq(inner)
	// cmdstring = "ssh -t " + shq("web") + " -- " + shq(bashCmd)
	want := "ssh -t " + shq("web") + " -- " + shq(bashCmd)

	if cmdstring != want {
		t.Errorf("cmdstring mismatch:\ngot:  %q\nwant: %q", cmdstring, want)
	}
}

func TestShq(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"hello", "'hello'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
		{"/srv/my app", "'/srv/my app'"},
		{"it's a 'test'", "'it'\\''s a '\\''test'\\'''"},
	}
	for _, tc := range tests {
		got := shq(tc.in)
		if got != tc.want {
			t.Errorf("shq(%q): want %q, got %q", tc.in, tc.want, got)
		}
	}
}

// ---- modeJump UI tests ----

func newRemoteTestFS() *fakeFS {
	return &fakeFS{name: "web", listings: map[string][]fs.Entry{
		"/root": {
			{Name: "docs", Path: "/root/docs", IsDir: true},
			{Name: "a.txt", Path: "/root/a.txt"},
		},
		"/root/docs": {{Name: "b.md", Path: "/root/docs/b.md"}},
		"/other":     {},
	}}
}

func TestJumpModePrefillDir(t *testing.T) {
	// cursor on "docs" (a dir) → prefill should be "/root/docs"
	e := loadedExplorer(t, newTestFS())
	// cursor is at 0 = docs (a dir)
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	if e.mode != modeJump {
		t.Fatalf("want modeJump after '>', got %v", e.mode)
	}
	if e.jumpInput.Value() != "/root/docs" {
		t.Fatalf("want prefill /root/docs (selected dir), got %q", e.jumpInput.Value())
	}
}

func TestJumpModePrefillFile(t *testing.T) {
	// cursor on "a.txt" (a file) → prefill should be rootPath "/root"
	e := loadedExplorer(t, newTestFS())
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyDown}) // move to a.txt
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	if e.mode != modeJump {
		t.Fatalf("want modeJump after '>', got %v", e.mode)
	}
	// a.txt has no parent (top-level) → rootPath
	if e.jumpInput.Value() != "/root" {
		t.Fatalf("want prefill /root (file at root level), got %q", e.jumpInput.Value())
	}
}

func TestJumpModePrefillFileWithParent(t *testing.T) {
	// expand docs, move cursor to b.md (has parent docs), expect prefill = /root/docs
	f := newTestFS()
	e := loadedExplorer(t, f)
	// expand docs
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	e, _ = e.Update(collectMsgs(cmd)[0])
	// cursor should be on docs (0), move down to b.md (1)
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyDown})
	n := e.tree.current()
	if n == nil || n.entry.Name != "b.md" {
		t.Fatalf("expected cursor on b.md, got %v", n)
	}
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	if e.mode != modeJump {
		t.Fatalf("want modeJump, got %v", e.mode)
	}
	// b.md has parent docs → prefill = /root/docs
	if e.jumpInput.Value() != "/root/docs" {
		t.Fatalf("want prefill /root/docs (file's parent), got %q", e.jumpInput.Value())
	}
}

func TestJumpModeEscCancels(t *testing.T) {
	e := loadedExplorer(t, newTestFS())
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	if e.mode != modeJump {
		t.Fatal("want modeJump")
	}
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if e.mode != modeTree {
		t.Fatalf("want modeTree after esc, got %v", e.mode)
	}
	if cmd != nil {
		t.Fatal("esc from jump should return nil cmd")
	}
}

func TestJumpModeConfirmProducesRootLoadAndRespawn(t *testing.T) {
	t.Setenv("TMUX", "") // no tmux in tests → respawn returns explorer-only status
	f := newTestFS()
	e := loadedExplorer(t, f)
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	e.jumpInput.SetValue("/root/docs")
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if e.mode == modeJump {
		t.Fatal("should have left modeJump after enter")
	}
	msgs := collectMsgs(cmd)
	var sawRootLoad, sawStatus bool
	for _, m := range msgs {
		switch m := m.(type) {
		case rootLoadedMsg:
			sawRootLoad = true
			if m.path != "/root/docs" {
				t.Errorf("rootLoadedMsg path: want /root/docs, got %q", m.path)
			}
		case statusMsg:
			// expect the "explorer only" message since TMUX=""
			if strings.Contains(m.text, "explorer only") || strings.Contains(m.text, "workspace →") {
				sawStatus = true
			}
		}
	}
	if !sawRootLoad {
		t.Error("want rootLoadedMsg from jump confirm")
	}
	if !sawStatus {
		t.Errorf("want explorer-only or workspace status, got msgs: %v", msgs)
	}
}

func TestJumpModeControlCharRefused(t *testing.T) {
	t.Setenv("TMUX", "")
	f := newTestFS()
	e := loadedExplorer(t, f)
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	// textinput strips control chars via SetValue, so inject directly into
	// the mode handler by bypassing the textinput — simulate what the handler
	// would do with a control char path by calling the guard directly.
	e.mode = modeJump
	// manually set jumpInput value to a path with control char by poking
	// the struct field (same package, so accessible in test)
	// bubbles textinput.SetValue sanitises - instead verify hasControlChar guard
	// is correct and reachable, then also test the jump confirm path with a
	// clean value to ensure normal operation works.
	if !hasControlChar("/root/ba\x01d") {
		t.Fatal("hasControlChar should detect control char")
	}
	// For a path without control chars the guard must not trigger
	e.jumpInput.SetValue("/root/docs")
	e, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if e.mode == modeJump {
		t.Fatal("should have left modeJump after enter")
	}
	msgs := collectMsgs(cmd)
	var sawErr bool
	for _, m := range msgs {
		if s, ok := m.(statusMsg); ok && s.isErr {
			sawErr = true
		}
	}
	if sawErr {
		t.Fatal("clean path should not produce an error status")
	}
}

func TestJumpTypingGuard(t *testing.T) {
	f := newTestFS()
	e := loadedExplorer(t, f)
	if e.typing() {
		t.Fatal("typing() should be false in modeTree")
	}
	e, _ = e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(">")})
	if e.mode != modeJump {
		t.Fatalf("want modeJump, got %v", e.mode)
	}
	if !e.typing() {
		t.Fatal("typing() must be true in modeJump so 'q' doesn't quit")
	}
}

func TestNewAppOptsAgent(t *testing.T) {
	a := NewAppOpts("/tmp", "", "mycli")
	if a.ex.agent != "mycli" {
		t.Fatalf("want agent=mycli, got %q", a.ex.agent)
	}
}

func TestNewAppOptsDefaultAgent(t *testing.T) {
	a := NewAppOpts("/tmp", "", "")
	if a.ex.agent != "claude" {
		t.Fatalf("want agent=claude by default, got %q", a.ex.agent)
	}
}
