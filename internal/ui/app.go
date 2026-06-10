package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Mapika/portside/internal/fs"
	"github.com/Mapika/portside/internal/sshconn"
)

type activeView int

const (
	viewExplorer activeView = iota
	viewPorts
)

// App is the root Bubble Tea model: it owns the SSH connection, routes
// messages to the explorer and ports views, and renders the status bar.
type App struct {
	ex     explorer
	pt     ports
	active activeView

	conn *sshconn.Conn
	fwd  *sshconn.Forwarder

	status    string
	statusErr bool
	width     int
	height    int
}

func NewApp(startDir string) App {
	return App{
		ex: newExplorer(fs.Local{}, startDir),
		pt: newPorts(),
	}
}

func (a App) Init() tea.Cmd { return a.ex.Init() }

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.ex.width, a.ex.height = msg.Width, msg.Height-1
		a.pt.height = msg.Height - 1
		return a, nil

	case statusMsg:
		a.status, a.statusErr = msg.text, msg.isErr
		return a, nil

	case connectResultMsg:
		a.ex.loading = false
		if msg.err != nil {
			a.status, a.statusErr = "connect "+msg.host+": "+msg.err.Error(), true
			return a, nil
		}
		if a.conn != nil {
			if a.fwd != nil {
				a.fwd.CloseAll()
			}
			a.conn.Close()
		}
		a.conn = msg.conn
		a.fwd = sshconn.NewForwarder(msg.conn.Client)
		a.pt.forwarder = a.fwd
		sfs := fs.NewSFTP(msg.conn.SFTP, msg.host)
		home, err := sfs.Home()
		if err != nil {
			home = "/"
		}
		var cmd tea.Cmd
		a.ex, cmd = a.ex.setFilesystem(sfs, home)
		a.status, a.statusErr = "connected to "+msg.host, false
		return a, cmd

	case downloadResultMsg:
		if msg.err != nil {
			a.status, a.statusErr = "download "+msg.name+": "+msg.err.Error(), true
		} else {
			a.status, a.statusErr = "downloaded "+msg.name, false
		}
		return a, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			a.cleanup()
			return a, tea.Quit
		case "q":
			if !a.typing() {
				a.cleanup()
				return a, tea.Quit
			}
		case "ctrl+p":
			if a.active == viewExplorer {
				a.active = viewPorts
			} else {
				a.active = viewExplorer
			}
			return a, nil
		}
	}

	var cmd tea.Cmd
	switch msg.(type) {
	case tea.KeyMsg, tea.MouseMsg:
		if a.active == viewExplorer {
			a.ex, cmd = a.ex.Update(msg)
		} else {
			a.pt, cmd = a.pt.Update(msg)
		}
	default:
		a.ex, cmd = a.ex.Update(msg)
	}
	return a, cmd
}

func (a App) typing() bool {
	if a.active == viewExplorer {
		return a.ex.typing()
	}
	return a.pt.typing()
}

func (a *App) cleanup() {
	if a.fwd != nil {
		a.fwd.CloseAll()
	}
	if a.conn != nil {
		a.conn.Close()
	}
}

func (a App) View() string {
	var body string
	if a.active == viewExplorer {
		body = a.ex.View()
	} else {
		body = a.pt.View()
	}

	style := statusStyle
	if a.statusErr {
		style = statusErrStyle
	}
	status := style.Width(max(a.width, 1)).Render(" " + a.status)

	if gap := a.height - 1 - lipgloss.Height(body); gap > 0 {
		body += strings.Repeat("\n", gap)
	}
	return body + "\n" + status
}
