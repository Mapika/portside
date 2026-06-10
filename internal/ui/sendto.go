package ui

import (
	"fmt"
	"os"
	"os/exec"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
)

// hasControlChar reports whether text contains any control character (< 0x20
// or == 0x7f). A path with a control character would press Enter or corrupt
// input in the target pane.
func hasControlChar(text string) bool {
	for _, c := range text {
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

// sendToAgentCmd hands the given text to the neighboring agent pane:
// inside tmux it is typed into the pane to the right; otherwise it is
// copied to the clipboard via OSC 52.
func sendToAgentCmd(text string) tea.Cmd {
	return func() tea.Msg {
		// Refuse anything non-printable (a control byte would press Enter or
		// corrupt the target pane's input).
		if hasControlChar(text) {
			return statusMsg{text: "refusing to send a path with control characters", isErr: true}
		}
		if os.Getenv("TMUX") != "" {
			err := exec.Command("tmux", "send-keys", "-t", "{right-of}", "-l", "--", text).Run()
			if err != nil {
				return statusMsg{text: "send failed (no agent pane to the right?): " + err.Error(), isErr: true}
			}
			return statusMsg{text: "sent to agent pane: " + text, isErr: false}
		}
		fmt.Fprint(os.Stderr, osc52.New(text))
		return statusMsg{text: "copied to clipboard: " + text, isErr: false}
	}
}
