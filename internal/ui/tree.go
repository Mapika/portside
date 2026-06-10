// Package ui contains the Bubble Tea models for portside.
package ui

import (
	"strings"

	"github.com/Mapika/portside/internal/fs"
)

type node struct {
	entry    fs.Entry
	depth    int
	expanded bool
	loaded   bool
	children []*node
}

// tree is the explorer's data structure: lazily-loaded nodes plus a
// flattened list of currently visible rows.
type tree struct {
	roots      []*node
	flat       []*node
	cursor     int
	showHidden bool
}

func newTree() *tree { return &tree{} }

func (t *tree) setRoot(entries []fs.Entry) {
	t.roots = makeNodes(entries, 0)
	t.cursor = 0
	t.reflatten()
}

func (t *tree) setChildren(n *node, entries []fs.Entry) {
	n.children = makeNodes(entries, n.depth+1)
	n.loaded = true
	n.expanded = true
	t.reflatten()
}

func makeNodes(entries []fs.Entry, depth int) []*node {
	out := make([]*node, 0, len(entries))
	for _, e := range entries {
		out = append(out, &node{entry: e, depth: depth})
	}
	return out
}

func (t *tree) reflatten() {
	t.flat = t.flat[:0]
	t.walk(t.roots)
	if t.cursor >= len(t.flat) {
		t.cursor = len(t.flat) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

func (t *tree) walk(nodes []*node) {
	for _, n := range nodes {
		if !t.showHidden && strings.HasPrefix(n.entry.Name, ".") {
			continue
		}
		t.flat = append(t.flat, n)
		if n.expanded {
			t.walk(n.children)
		}
	}
}

func (t *tree) visible() []*node { return t.flat }

func (t *tree) current() *node {
	if len(t.flat) == 0 {
		return nil
	}
	return t.flat[t.cursor]
}

func (t *tree) moveUp() {
	if t.cursor > 0 {
		t.cursor--
	}
}

func (t *tree) moveDown() {
	if t.cursor < len(t.flat)-1 {
		t.cursor++
	}
}

func (t *tree) toggleHidden() {
	t.showHidden = !t.showHidden
	t.reflatten()
}
