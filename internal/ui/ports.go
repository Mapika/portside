package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mapika/portside/internal/sshconn"
)

type ports struct {
	forwarder *sshconn.Forwarder
	cursor    int
	adding    bool
	localIn   textinput.Model
	remoteIn  textinput.Model
	focus     int // 0 = local input, 1 = remote input
	height    int
}

func newPorts() ports {
	li := textinput.New()
	li.Prompt = "local port: "
	li.CharLimit = 5
	ri := textinput.New()
	ri.Prompt = "remote port: "
	ri.CharLimit = 5
	return ports{localIn: li, remoteIn: ri}
}

func (p ports) typing() bool { return p.adding }

func (p ports) Update(msg tea.Msg) (ports, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}

	if p.adding {
		switch key.String() {
		case "esc":
			p.adding = false
			p.localIn.Blur()
			p.remoteIn.Blur()
			return p, nil
		case "tab", "shift+tab":
			p.focus = 1 - p.focus
			if p.focus == 0 {
				p.remoteIn.Blur()
				return p, p.localIn.Focus()
			}
			p.localIn.Blur()
			return p, p.remoteIn.Focus()
		case "enter":
			lp, err1 := strconv.Atoi(p.localIn.Value())
			rp, err2 := strconv.Atoi(p.remoteIn.Value())
			if err1 != nil || err2 != nil {
				return p, statusCmd("ports must be numbers", true)
			}
			p.adding = false
			p.localIn.Blur()
			p.remoteIn.Blur()
			fw, err := p.forwarder.Add(lp, rp)
			if err != nil {
				return p, statusCmd("forward failed: "+err.Error(), true)
			}
			return p, statusCmd(fmt.Sprintf("forwarding localhost:%d → remote:%d", fw.Local, fw.Remote), false)
		}
		var cmd tea.Cmd
		if p.focus == 0 {
			p.localIn, cmd = p.localIn.Update(key)
		} else {
			p.remoteIn, cmd = p.remoteIn.Update(key)
		}
		return p, cmd
	}

	switch key.String() {
	case "a":
		if p.forwarder == nil {
			return p, statusCmd("connect to a host first (Ctrl+H in the explorer)", true)
		}
		p.adding = true
		p.focus = 0
		p.localIn.SetValue("")
		p.remoteIn.SetValue("")
		return p, p.localIn.Focus()
	case "x":
		if p.forwarder == nil {
			return p, nil
		}
		fws := p.forwarder.List()
		if p.cursor < len(fws) {
			p.forwarder.Stop(fws[p.cursor])
			if p.cursor > 0 {
				p.cursor--
			}
			return p, statusCmd("forward stopped", false)
		}
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.forwarder != nil && p.cursor < len(p.forwarder.List())-1 {
			p.cursor++
		}
	}
	return p, nil
}

func (p ports) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" PORTS") + "\n")

	if p.forwarder == nil {
		b.WriteString(dimStyle.Render(" no connection — press Ctrl+H in the explorer to connect to a host") + "\n")
		return b.String()
	}

	fws := p.forwarder.List()
	if len(fws) == 0 {
		b.WriteString(dimStyle.Render(" no forwards — press a to add one") + "\n")
	}
	for i, fw := range fws {
		state := "open"
		if fw.Err() != nil {
			state = "error: " + fw.Err().Error()
		}
		line := fmt.Sprintf("  localhost:%d → remote:%d  [%s]", fw.Local, fw.Remote, state)
		if i == p.cursor {
			line = cursorStyle.Render(" ▶" + line[2:])
		}
		b.WriteString(line + "\n")
	}

	if p.adding {
		b.WriteString("\n " + p.localIn.View() + "\n " + p.remoteIn.View() + "\n")
		b.WriteString(dimStyle.Render(" tab switch · enter confirm · esc cancel"))
	} else {
		b.WriteString("\n" + dimStyle.Render(" a add · x stop · ctrl+p explorer"))
	}
	return b.String()
}
