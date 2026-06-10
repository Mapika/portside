package ui

import (
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mapika/portside/internal/fs"
	"github.com/Mapika/portside/internal/sshconn"
)

// shq wraps s in POSIX single quotes, escaping any embedded single quotes via
// the '\'' idiom. The result is safe to embed in a shell command string.
func shq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// respawnArgv builds the tmux respawn-pane argv for moving the agent pane
// (target = explicit pane id) to dir. For a local backend the argv drives
// exec.Command directly (no shell). For a remote backend (fsys.Name() !=
// "local") a single shell-command string is passed so that ssh is re-invoked
// with the right working directory.
func respawnArgv(fsysName, host, dir, agent, target string) []string {
	if fsysName == "local" {
		return []string{"tmux", "respawn-pane", "-k", "-t", target, "-c", dir, agent}
	}
	// remote: build cmdstring = ssh -t <host> -- <bash -lc 'cd <dir> && exec <agent>'>
	inner := "cd " + shq(dir) + " && exec " + agent
	bashCmd := "bash -lc " + shq(inner)
	cmdstring := "ssh -t " + shq(host) + " -- " + shq(bashCmd)
	return []string{"tmux", "respawn-pane", "-k", "-t", target, cmdstring}
}

// respawnAgentCmd issues the tmux respawn-pane command so the agent pane
// restarts in dir. When $TMUX is unset it returns a status message explaining
// that only the explorer moved.
func respawnAgentCmd(fsysName, host, dir, agent string) tea.Cmd {
	return func() tea.Msg {
		if os.Getenv("TMUX") == "" {
			return statusMsg{text: "moved the explorer only (agent pane needs tmux)", isErr: false}
		}
		target, err := rightPaneID()
		if err != nil {
			return statusMsg{text: "agent pane: " + err.Error(), isErr: true}
		}
		argv := respawnArgv(fsysName, host, dir, agent, target)
		// argv[0] == "tmux"
		if err := exec.Command(argv[0], argv[1:]...).Run(); err != nil {
			return statusMsg{text: "respawn-pane: " + err.Error(), isErr: true}
		}
		return statusMsg{text: "workspace → " + dir, isErr: false}
	}
}

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
	host       string
	conn       *sshconn.Conn
	err        error
	authFailed bool
}

type downloadResultMsg struct {
	name string
	err  error
}

type fileOpResultMsg struct {
	parent *node  // nil means reload from root
	verb   string // "uploaded", "renamed", "deleted", "created"
	name   string // name of the entry for status messages
	err    error
}

func fileOpCmd(fsys fs.Filesystem, verb, name string, parent *node, fn func() error) tea.Cmd {
	return func() tea.Msg {
		return fileOpResultMsg{parent: parent, verb: verb, name: name, err: fn()}
	}
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

func connectCmd(alias, secret string) tea.Cmd {
	return func() tea.Msg {
		conn, err := sshconn.Connect(alias, secret)
		return connectResultMsg{host: alias, conn: conn, err: err, authFailed: sshconn.IsAuthErr(err)}
	}
}

func downloadCmd(fsys fs.Filesystem, src, destDir, name string) tea.Cmd {
	return func() tea.Msg {
		return downloadResultMsg{name: name, err: fsys.Download(src, destDir)}
	}
}

type watchTickMsg struct{ gen int }

type refreshedMsg struct {
	parent  *node // nil = root
	path    string
	entries []fs.Entry
	err     error
}

func watchTickCmd(gen int) tea.Cmd {
	return tea.Tick(watchInterval, func(time.Time) tea.Msg { return watchTickMsg{gen: gen} })
}

func refreshCmd(fsys fs.Filesystem, parent *node, path string) tea.Cmd {
	return func() tea.Msg {
		entries, err := fsys.List(path)
		return refreshedMsg{parent: parent, path: path, entries: entries, err: err}
	}
}
