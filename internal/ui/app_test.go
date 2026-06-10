package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
