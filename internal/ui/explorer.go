package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Mapika/portside/internal/fs"
	"github.com/Mapika/portside/internal/sshconn"
)

type exMode int

const (
	modeTree exMode = iota
	modePath
	modeHosts
	modeDownload
)

type explorer struct {
	fsys     fs.Filesystem
	rootPath string
	tree     *tree
	mode     exMode

	pathInput  textinput.Model
	destInput  textinput.Model
	hosts      []string
	hostCursor int
	pending    *node // node chosen for download

	loading bool
	width   int
	height  int
}

func newExplorer(fsys fs.Filesystem, rootPath string) explorer {
	pi := textinput.New()
	pi.Prompt = "path: "
	di := textinput.New()
	di.Prompt = "save to: "
	home, _ := os.UserHomeDir()
	di.SetValue(filepath.Join(home, "Downloads"))
	return explorer{
		fsys:      fsys,
		rootPath:  rootPath,
		tree:      newTree(),
		pathInput: pi,
		destInput: di,
		loading:   true,
	}
}

func (e explorer) typing() bool { return e.mode == modePath || e.mode == modeDownload }

func (e explorer) Init() tea.Cmd { return loadRootCmd(e.fsys, e.rootPath) }

// setFilesystem switches backend (called by App after a successful connect,
// or internally when the user picks "local").
func (e explorer) setFilesystem(fsys fs.Filesystem, root string) (explorer, tea.Cmd) {
	e.fsys = fsys
	e.rootPath = root
	e.tree = newTree()
	e.loading = true
	return e, loadRootCmd(fsys, root)
}

func (e explorer) Update(msg tea.Msg) (explorer, tea.Cmd) {
	switch msg := msg.(type) {
	case rootLoadedMsg:
		e.loading = false
		if msg.err != nil {
			return e, statusCmd("error: "+msg.err.Error(), true)
		}
		e.rootPath = msg.path
		e.tree.setRoot(msg.entries)
		return e, statusCmd(e.fsys.Name()+" · "+msg.path, false)
	case childrenLoadedMsg:
		e.loading = false
		if msg.err != nil {
			return e, statusCmd("error: "+msg.err.Error(), true)
		}
		e.tree.setChildren(msg.parent, msg.entries)
		return e, nil
	case tea.KeyMsg:
		return e.handleKey(msg)
	case tea.MouseMsg:
		if e.mode == modeTree {
			return e.handleMouse(msg)
		}
		return e, nil
	}
	return e, nil
}

func (e explorer) handleKey(msg tea.KeyMsg) (explorer, tea.Cmd) {
	switch e.mode {
	case modePath:
		switch msg.String() {
		case "enter":
			target := e.pathInput.Value()
			e.mode = modeTree
			e.pathInput.Blur()
			e.loading = true
			return e, loadRootCmd(e.fsys, target)
		case "esc":
			e.mode = modeTree
			e.pathInput.Blur()
			return e, nil
		}
		var cmd tea.Cmd
		e.pathInput, cmd = e.pathInput.Update(msg)
		return e, cmd

	case modeDownload:
		switch msg.String() {
		case "enter":
			dest := e.destInput.Value()
			n := e.pending
			e.mode = modeTree
			e.destInput.Blur()
			if n == nil {
				return e, nil
			}
			return e, tea.Batch(
				statusCmd("downloading "+n.entry.Name+"…", false),
				downloadCmd(e.fsys, n.entry.Path, dest, n.entry.Name),
			)
		case "esc":
			e.mode = modeTree
			e.destInput.Blur()
			return e, nil
		}
		var cmd tea.Cmd
		e.destInput, cmd = e.destInput.Update(msg)
		return e, cmd

	case modeHosts:
		switch msg.String() {
		case "up", "k":
			if e.hostCursor > 0 {
				e.hostCursor--
			}
		case "down", "j":
			if e.hostCursor < len(e.hosts) { // index 0 is "local"
				e.hostCursor++
			}
		case "enter":
			e.mode = modeTree
			if e.hostCursor == 0 {
				local := fs.Local{}
				home, err := local.Home()
				if err != nil {
					home = "/"
				}
				return e.setFilesystem(local, home)
			}
			alias := e.hosts[e.hostCursor-1]
			e.loading = true
			return e, tea.Batch(
				statusCmd("connecting to "+alias+"…", false),
				connectCmd(alias),
			)
		case "esc":
			e.mode = modeTree
		}
		return e, nil
	}

	// modeTree
	switch msg.String() {
	case "up", "k":
		e.tree.moveUp()
	case "down", "j":
		e.tree.moveDown()
	case "enter", "right", "l":
		n := e.tree.current()
		if n == nil || !n.entry.IsDir {
			break
		}
		if n.expanded {
			n.expanded = false
			e.tree.reflatten()
		} else if n.loaded {
			n.expanded = true
			e.tree.reflatten()
		} else {
			e.loading = true
			return e, loadChildrenCmd(e.fsys, n)
		}
	case "left", "h":
		n := e.tree.current()
		if n != nil && n.entry.IsDir && n.expanded {
			n.expanded = false
			e.tree.reflatten()
		}
	case ":", "ctrl+l":
		e.mode = modePath
		e.pathInput.SetValue(e.rootPath)
		e.pathInput.CursorEnd()
		return e, e.pathInput.Focus()
	case "ctrl+h":
		r, err := sshconn.LoadConfig(sshconn.DefaultConfigPath())
		if err != nil {
			return e, statusCmd("ssh config: "+err.Error(), true)
		}
		e.hosts = r.Hosts()
		e.hostCursor = 0
		e.mode = modeHosts
	case "d":
		n := e.tree.current()
		if n == nil {
			break
		}
		e.pending = n
		e.mode = modeDownload
		e.destInput.CursorEnd()
		return e, e.destInput.Focus()
	case "r":
		e.loading = true
		return e, loadRootCmd(e.fsys, e.rootPath)
	case ".":
		e.tree.toggleHidden()
	case "R":
		if e.fsys.Name() != "local" {
			e.loading = true
			return e, tea.Batch(
				statusCmd("reconnecting to "+e.fsys.Name()+"…", false),
				connectCmd(e.fsys.Name()),
			)
		}
	}
	return e, nil
}

// window returns the first visible row index and the row capacity, matching
// what View renders.
func (e explorer) window() (start, maxRows int) {
	vis := e.tree.visible()
	maxRows = e.height - 2
	if maxRows < 1 {
		maxRows = len(vis)
	}
	if e.tree.cursor >= maxRows {
		start = e.tree.cursor - maxRows + 1
	}
	return start, maxRows
}

func (e explorer) handleMouse(msg tea.MouseMsg) (explorer, tea.Cmd) {
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		e.tree.moveUp()
	case tea.MouseButtonWheelDown:
		e.tree.moveDown()
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return e, nil
		}
		start, maxRows := e.window()
		idx := msg.Y - 1 // row 0 is the title bar
		if idx >= 0 && idx < maxRows && start+idx < len(e.tree.visible()) {
			e.tree.cursor = start + idx
		}
	}
	return e, nil
}

func (e explorer) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" "+e.fsys.Name()+" · "+e.rootPath) + "\n")

	if e.mode == modeHosts {
		b.WriteString(dimStyle.Render(" select host · enter connect · esc cancel") + "\n")
		items := append([]string{"local"}, e.hosts...)
		for i, it := range items {
			if i == e.hostCursor {
				b.WriteString(cursorStyle.Render(" ▶ "+it) + "\n")
			} else {
				b.WriteString("   " + it + "\n")
			}
		}
		return b.String()
	}

	vis := e.tree.visible()
	start, maxRows := e.window()
	for i := start; i < len(vis) && i-start < maxRows; i++ {
		b.WriteString(e.renderNode(vis[i], i == e.tree.cursor) + "\n")
	}

	switch e.mode {
	case modePath:
		b.WriteString(e.pathInput.View())
	case modeDownload:
		b.WriteString(e.destInput.View())
	default:
		if e.loading {
			b.WriteString(dimStyle.Render(" loading…"))
		}
	}
	return b.String()
}

func (e explorer) renderNode(n *node, selected bool) string {
	indent := strings.Repeat("  ", n.depth)
	marker := "  "
	if n.entry.IsDir {
		if n.expanded {
			marker = "▾ "
		} else {
			marker = "▸ "
		}
	}
	line := " " + indent + marker + n.entry.Name
	switch {
	case selected:
		return cursorStyle.Render(line)
	case strings.HasPrefix(n.entry.Name, "."):
		return dimStyle.Render(line)
	case n.entry.IsDir:
		return dirStyle.Render(line)
	}
	return line
}
