// Package ui contains the Bubble Tea models for portside.
package ui

import (
	"sort"
	"strings"
	"time"

	"github.com/Mapika/portside/internal/fs"
)

type node struct {
	entry     fs.Entry
	depth     int
	expanded  bool
	loaded    bool
	children  []*node
	parent    *node // nil for root nodes
	changedAt time.Time
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
	t.roots = makeNodes(entries, 0, nil)
	t.cursor = 0
	t.reflatten()
}

func (t *tree) setChildren(n *node, entries []fs.Entry) {
	n.children = makeNodes(entries, n.depth+1, n)
	n.loaded = true
	n.expanded = true
	t.reflatten()
}

func makeNodes(entries []fs.Entry, depth int, parent *node) []*node {
	out := make([]*node, 0, len(entries))
	for _, e := range entries {
		out = append(out, &node{entry: e, depth: depth, parent: parent})
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

// mergeChildren updates parent's children (or t.roots when parent is nil)
// from the new entries list. Existing nodes that match an entry by name keep
// their state (expanded/loaded/children/changedAt); their changedAt is set to
// now when Size or ModTime differs from the stored entry. New entries get
// changedAt = now. Entries not present in the new list are dropped. A
// reflatten is performed at the end (cursor clamps).
func (t *tree) mergeChildren(parent *node, entries []fs.Entry, now time.Time) {
	depth := 0
	if parent != nil {
		depth = parent.depth + 1
	}

	// Build a map of old nodes by name.
	var oldChildren []*node
	if parent == nil {
		oldChildren = t.roots
	} else {
		oldChildren = parent.children
	}
	oldByName := make(map[string]*node, len(oldChildren))
	for _, n := range oldChildren {
		oldByName[n.entry.Name] = n
	}

	newChildren := make([]*node, 0, len(entries))
	for _, e := range entries {
		if old, ok := oldByName[e.Name]; ok {
			// Keep the existing node; update entry, mark changed if metadata differs.
			changed := e.Size != old.entry.Size || !e.ModTime.Equal(old.entry.ModTime)
			old.entry = e
			old.depth = depth
			old.parent = parent
			if changed {
				old.changedAt = now
			}
			newChildren = append(newChildren, old)
		} else {
			// New entry — mark as changed.
			newChildren = append(newChildren, &node{
				entry:     e,
				depth:     depth,
				parent:    parent,
				changedAt: now,
			})
		}
	}

	if parent == nil {
		t.roots = newChildren
	} else {
		parent.children = newChildren
	}
	t.reflatten()
}

// expandedDirs returns all loaded+expanded nodes in the tree (recursive).
// Root nodes are always visited; for any node deeper in the tree, children
// are only visited when the node itself is loaded and expanded. This prevents
// collecting dirs that happen to be expanded inside a collapsed parent.
func (t *tree) expandedDirs() []*node {
	var out []*node
	var collect func(nodes []*node)
	collect = func(nodes []*node) {
		for _, n := range nodes {
			if n.loaded && n.expanded {
				out = append(out, n)
				collect(n.children)
			}
		}
	}
	collect(t.roots)
	return out
}

// recentChanges returns all nodes (recursive) whose changedAt is within the
// given duration before now, sorted most-recent first.
func (t *tree) recentChanges(within time.Duration, now time.Time) []*node {
	var out []*node
	var collect func(nodes []*node)
	collect = func(nodes []*node) {
		for _, n := range nodes {
			if !n.changedAt.IsZero() && now.Sub(n.changedAt) < within {
				out = append(out, n)
			}
			collect(n.children)
		}
	}
	collect(t.roots)
	sort.Slice(out, func(i, j int) bool {
		return out[i].changedAt.After(out[j].changedAt)
	})
	return out
}
