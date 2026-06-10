package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mapika/portside/internal/fs"
)

// authErr is a fake error that IsAuthErr will recognise.
var errAuth = errors.New("unable to authenticate")

func TestAppInitWithHostConnects(t *testing.T) {
	a := NewAppWithHost("/tmp", "portside-test-no-such-host")
	cmd := a.Init()
	if cmd == nil {
		t.Fatal("want an init command")
	}
	var sawConnecting, sawResult bool
	for _, m := range collectMsgs(cmd) {
		switch m := m.(type) {
		case statusMsg:
			if strings.Contains(m.text, "connecting to portside-test-no-such-host") {
				sawConnecting = true
			}
		case connectResultMsg:
			sawResult = true
			if m.err == nil {
				t.Fatal("connect to a nonexistent host should fail")
			}
			if m.host != "portside-test-no-such-host" {
				t.Fatalf("wrong host: %s", m.host)
			}
		}
	}
	if !sawConnecting || !sawResult {
		t.Fatalf("want connecting status + connect result, got connecting=%v result=%v", sawConnecting, sawResult)
	}
}

func TestAppInitWithoutHostLoadsLocal(t *testing.T) {
	dir := t.TempDir()
	a := NewApp(dir)
	// Init() returns a batch (loadRootCmd + watchTickCmd). Execute only the
	// root-load part directly to avoid blocking on the 3-second tick.
	msg := loadRootCmd(a.ex.fsys, a.ex.rootPath)()
	if _, ok := msg.(rootLoadedMsg); !ok {
		t.Fatalf("want rootLoadedMsg, got %#v", msg)
	}
}

func TestAppTogglesViews(t *testing.T) {
	a := NewApp("/tmp")
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	a = m.(App)
	if a.active != viewPorts {
		t.Fatal("ctrl+p should switch to ports view")
	}
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	a = m.(App)
	if a.active != viewExplorer {
		t.Fatal("ctrl+p should switch back to explorer")
	}
}

func TestAppStatusBar(t *testing.T) {
	a := NewApp("/tmp")
	a.width, a.height = 60, 20
	m, _ := a.Update(statusMsg{text: "hello status", isErr: false})
	a = m.(App)
	if !strings.Contains(a.View(), "hello status") {
		t.Fatal("status bar should render the message")
	}
}

func TestAppQuitsOnCtrlC(t *testing.T) {
	a := NewApp("/tmp")
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("want quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("want tea.QuitMsg")
	}
}

func TestAppFailedStartupConnectOpensHostPicker(t *testing.T) {
	a := NewAppWithHost("/tmp", "deadhost")
	m, _ := a.Update(connectResultMsg{host: "deadhost", err: errors.New("boom")})
	a = m.(App)
	if !a.statusErr || !strings.Contains(a.status, "deadhost") {
		t.Fatalf("want red connect status, got %q (err=%v)", a.status, a.statusErr)
	}
	if a.ex.mode != modeHosts {
		t.Fatal("want host picker after failed startup connect")
	}
}

func TestAppFailedReconnectKeepsLoadedTree(t *testing.T) {
	a := NewApp("/tmp")
	// simulate a loaded local tree
	a.ex, _ = a.ex.Update(rootLoadedMsg{path: "/tmp", entries: nil})
	a.ex.tree.setRoot([]fs.Entry{{Name: "x", Path: "/tmp/x"}})
	m, _ := a.Update(connectResultMsg{host: "deadhost", err: errors.New("boom")})
	a = m.(App)
	if a.ex.mode == modeHosts {
		t.Fatal("must not hijack an already-loaded view")
	}
	if len(a.ex.tree.visible()) != 1 {
		t.Fatal("loaded tree must survive a failed connect")
	}
}

// TestAppAuthFailedEntersPasswordMode verifies that an authFailed connect
// result puts the explorer into modePassword.
func TestAppAuthFailedEntersPasswordMode(t *testing.T) {
	a := NewAppWithHost("/tmp", "myhost")
	m, _ := a.Update(connectResultMsg{host: "myhost", err: errAuth, authFailed: true})
	a = m.(App)
	if a.ex.mode != modePassword {
		t.Fatalf("want modePassword after authFailed, got mode %v", a.ex.mode)
	}
	if a.ex.pendingHost != "myhost" {
		t.Fatalf("want pendingHost=myhost, got %q", a.ex.pendingHost)
	}
}

// TestAppPasswordPromptSubmit verifies that pressing enter in modePassword
// issues a connectCmd with the typed secret.
func TestAppPasswordPromptSubmit(t *testing.T) {
	a := NewAppWithHost("/tmp", "myhost")
	m, _ := a.Update(connectResultMsg{host: "myhost", err: errAuth, authFailed: true})
	a = m.(App)
	if a.ex.mode != modePassword {
		t.Fatalf("setup: want modePassword, got %v", a.ex.mode)
	}

	// type a secret
	a.ex.passInput.SetValue("topsecret")

	// press enter — should issue connectCmd and return to loading
	m2, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = m2.(App)

	if cmd == nil {
		t.Fatal("want a connect command after submitting password")
	}
	// execute the command and verify it produces a connectResultMsg for the same host
	msgs := collectMsgs(cmd)
	var sawResult bool
	for _, msg := range msgs {
		switch msg := msg.(type) {
		case connectResultMsg:
			sawResult = true
			if msg.host != "myhost" {
				t.Fatalf("want host=myhost in connectResultMsg, got %q", msg.host)
			}
		case statusMsg:
			if strings.Contains(msg.text, "connecting") {
				// fine
			}
		}
	}
	if !sawResult {
		t.Fatal("want connectResultMsg from password submit cmd")
	}
	// input should be cleared
	if a.ex.passInput.Value() != "" {
		t.Fatalf("passInput should be cleared after submit, got %q", a.ex.passInput.Value())
	}
}

// TestAppPasswordPromptEscCancels verifies esc in modePassword goes back to
// modeTree (and to modeHosts if tree is empty).
func TestAppPasswordPromptEscCancels(t *testing.T) {
	// case 1: tree is empty → go to host picker
	a := NewAppWithHost("/tmp", "myhost")
	m, _ := a.Update(connectResultMsg{host: "myhost", err: errAuth, authFailed: true})
	a = m.(App)
	m2, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m2.(App)
	if a.ex.mode != modeHosts {
		t.Fatalf("want modeHosts after esc with empty tree, got %v", a.ex.mode)
	}

	// case 2: tree is loaded → go back to modeTree
	a2 := NewApp("/tmp")
	a2.ex, _ = a2.ex.Update(rootLoadedMsg{path: "/tmp", entries: nil})
	a2.ex.tree.setRoot([]fs.Entry{{Name: "x", Path: "/tmp/x"}})
	m3, _ := a2.Update(connectResultMsg{host: "other", err: errAuth, authFailed: true})
	a2 = m3.(App)
	if a2.ex.mode != modePassword {
		t.Fatalf("setup case2: want modePassword, got %v", a2.ex.mode)
	}
	m4, _ := a2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a2 = m4.(App)
	if a2.ex.mode != modeTree {
		t.Fatalf("want modeTree after esc with loaded tree, got %v", a2.ex.mode)
	}
}

// TestAppPasswordWrongSecondPrompt verifies that a second authFailed while in
// modePassword re-prompts (stays in modePassword).
func TestAppPasswordWrongSecondPrompt(t *testing.T) {
	a := NewAppWithHost("/tmp", "myhost")
	m, _ := a.Update(connectResultMsg{host: "myhost", err: errAuth, authFailed: true})
	a = m.(App)

	// simulate second failure (wrong password)
	m2, _ := a.Update(connectResultMsg{host: "myhost", err: errAuth, authFailed: true})
	a = m2.(App)
	if a.ex.mode != modePassword {
		t.Fatalf("want modePassword after second auth failure, got %v", a.ex.mode)
	}
	if !a.statusErr {
		t.Fatal("want error status after failed password attempt")
	}
}
