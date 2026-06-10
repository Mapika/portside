package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
	a := NewApp(t.TempDir())
	msgs := collectMsgs(a.Init())
	if len(msgs) != 1 {
		t.Fatalf("want 1 msg, got %d", len(msgs))
	}
	if _, ok := msgs[0].(rootLoadedMsg); !ok {
		t.Fatalf("want rootLoadedMsg, got %#v", msgs[0])
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
