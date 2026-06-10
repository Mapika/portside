package ui

import (
	"bytes"
	gopath "path"
	"path/filepath"
	"strings"
)

type gitState int

const (
	gitNone gitState = iota
	gitModified
	gitUntracked
)

type gitStatusMsg struct {
	root   string // rootPath the status was computed for
	top    string // resolved repo top (cached on receipt)
	states map[string]gitState
}

// parseGitStatus parses `git status --porcelain -z` output. Entries are
// NUL-separated; each entry starts with a two-character XY status code,
// a space, then the path. Rename entries (X == 'R') are followed by a second
// NUL-separated origin path (which is skipped). Paths in the result are
// joined onto repoTop (absolute).
//
//   - "??" → gitUntracked
//   - "!!" → ignored (skipped)
//   - anything else → gitModified
func parseGitStatus(out []byte, repoTop string) map[string]gitState {
	result := make(map[string]gitState)
	if len(out) == 0 {
		return result
	}
	// Trim any trailing NUL so that splitting on NUL gives clean entries.
	out = bytes.TrimRight(out, "\x00")
	entries := bytes.Split(out, []byte{0})

	i := 0
	for i < len(entries) {
		e := entries[i]
		i++
		if len(e) < 3 {
			continue
		}
		xy := string(e[:2])
		path := strings.TrimPrefix(string(e[2:]), " ")

		// Rename and copy entries: the next NUL-separated item is the origin path — skip it.
		if len(xy) > 0 && (xy[0] == 'R' || xy[0] == 'C') {
			i++ // skip origin path
		}

		// Keys are slash-normalized so lookups work for both POSIX remote
		// paths and Windows local paths (the render side normalizes too).
		abs := gopath.Join(filepath.ToSlash(repoTop), path)
		switch xy {
		case "!!":
			// ignored file — skip
			continue
		case "??":
			result[abs] = gitUntracked
		default:
			result[abs] = gitModified
		}
	}
	return result
}
