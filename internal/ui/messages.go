package ui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mapika/portside/internal/fs"
	"github.com/Mapika/portside/internal/sshconn"
)

type rootLoadedMsg struct {
	path    string
	entries []fs.Entry
	err     error
}

type childrenLoadedMsg struct {
	parent  *node
	entries []fs.Entry
	err     error
}

type connectResultMsg struct {
	host string
	conn *sshconn.Conn
	err  error
}

type downloadResultMsg struct {
	name string
	err  error
}

type statusMsg struct {
	text  string
	isErr bool
}

func statusCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return statusMsg{text: text, isErr: isErr} }
}

func loadRootCmd(fsys fs.Filesystem, path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := fsys.List(path)
		return rootLoadedMsg{path: path, entries: entries, err: err}
	}
}

func loadChildrenCmd(fsys fs.Filesystem, n *node) tea.Cmd {
	return func() tea.Msg {
		entries, err := fsys.List(n.entry.Path)
		return childrenLoadedMsg{parent: n, entries: entries, err: err}
	}
}

func connectCmd(alias string) tea.Cmd {
	return func() tea.Msg {
		conn, err := sshconn.Connect(alias)
		return connectResultMsg{host: alias, conn: conn, err: err}
	}
}

func downloadCmd(fsys fs.Filesystem, src, destDir, name string) tea.Cmd {
	return func() tea.Msg {
		return downloadResultMsg{name: name, err: fsys.Download(src, destDir)}
	}
}
