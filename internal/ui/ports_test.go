package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mapika/portside/internal/sshconn"
)

func typeRunes(p ports, s string) ports {
	for _, r := range s {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return p
}

func TestPortsRequiresConnection(t *testing.T) {
	p := newPorts()
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if p.adding {
		t.Fatal("must not enter add mode without a forwarder")
	}
	msgs := collectMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatal("want a status message")
	}
	if s, ok := msgs[0].(statusMsg); !ok || !s.isErr {
		t.Fatalf("want error status, got %#v", msgs[0])
	}
}

func TestPortsAddAndStopForward(t *testing.T) {
	// nil ssh client is fine: dialing only happens on incoming connections
	p := newPorts()
	p.forwarder = sshconn.NewForwarder(nil)
	defer p.forwarder.CloseAll()

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if !p.adding {
		t.Fatal("want add mode")
	}
	p = typeRunes(p, "0") // local port 0 = pick a free one
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	p = typeRunes(p, "3000")
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.adding {
		t.Fatal("add mode should end on enter")
	}
	if s, ok := collectMsgs(cmd)[0].(statusMsg); !ok || s.isErr {
		t.Fatalf("want success status, got %#v", collectMsgs(cmd)[0])
	}

	fws := p.forwarder.List()
	if len(fws) != 1 || fws[0].Remote != 3000 || fws[0].Local == 0 {
		t.Fatalf("unexpected forwards: %+v", fws)
	}

	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if len(p.forwarder.List()) != 0 {
		t.Fatal("forward should be stopped")
	}
}

func TestPortsRejectsNonNumeric(t *testing.T) {
	p := newPorts()
	p.forwarder = sshconn.NewForwarder(nil)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	p = typeRunes(p, "abc")
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if s, ok := collectMsgs(cmd)[0].(statusMsg); !ok || !s.isErr {
		t.Fatal("want error status for non-numeric port")
	}
}

func TestPortsViewShowsHintWithoutConnection(t *testing.T) {
	p := newPorts()
	if !strings.Contains(p.View(), "no connection") {
		t.Fatalf("view should hint at missing connection:\n%s", p.View())
	}
}
