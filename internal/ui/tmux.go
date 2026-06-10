package ui

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// rightPaneID returns the tmux pane id of the nearest pane to the right of
// the pane portside runs in. The {right-of} target token is unreliable here:
// it resolves against the window's ACTIVE pane, which is usually the agent
// pane, not ours.
func rightPaneID() (string, error) {
	own := os.Getenv("TMUX_PANE")
	if own == "" {
		return "", errors.New("not inside tmux")
	}
	out, err := exec.Command("tmux", "list-panes", "-F", "#{pane_id} #{pane_left}").Output()
	if err != nil {
		return "", err
	}
	return parseRightPane(string(out), own)
}

// parseRightPane picks the pane with the smallest pane_left greater than the
// own pane's, from `tmux list-panes -F '#{pane_id} #{pane_left}'` output.
func parseRightPane(out, own string) (string, error) {
	type pane struct {
		id   string
		left int
	}
	var panes []pane
	ownLeft := -1
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		left, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		if fields[0] == own {
			ownLeft = left
			continue
		}
		panes = append(panes, pane{id: fields[0], left: left})
	}
	if ownLeft < 0 {
		return "", errors.New("own pane not found")
	}
	best := pane{left: int(^uint(0) >> 1)}
	for _, p := range panes {
		if p.left > ownLeft && p.left < best.left {
			best = p
		}
	}
	if best.id == "" {
		return "", errors.New("no pane to the right")
	}
	return best.id, nil
}
