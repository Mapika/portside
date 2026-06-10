package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

// sendToClaudeCmd hands the selected path to the neighboring Claude pane:
// inside tmux it is typed into the pane to the right; otherwise it is
// copied to the clipboard via OSC 52.
func sendToClaudeCmd(path string) tea.Cmd {
	return func() tea.Msg {
		// a remote-controlled filename containing a newline would press
		// Enter in the target pane — refuse anything with control chars
		if strings.ContainsAny(path, "\n\r\x00\x1b") {
			return statusMsg{text: "refusing to send a path with control characters", isErr: true}
		}
		if os.Getenv("TMUX") != "" {
			err := exec.Command("tmux", "send-keys", "-t", "{right-of}", "-l", "--", path+" ").Run()
			if err != nil {
				return statusMsg{text: "send to claude: " + err.Error(), isErr: true}
			}
			return statusMsg{text: "sent to claude: " + path, isErr: false}
		}
		fmt.Fprint(os.Stderr, osc52.New(path))
		return statusMsg{text: "copied to clipboard: " + path, isErr: false}
	}
}
