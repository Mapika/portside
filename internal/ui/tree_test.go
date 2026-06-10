package ui

import (
	"testing"
	"time"

	"github.com/Mapika/portside/internal/fs"
)

func sampleRoot() []fs.Entry {
	return []fs.Entry{
		{Name: "docs", Path: "/r/docs", IsDir: true},
		{Name: ".hidden", Path: "/r/.hidden"},
		{Name: "a.txt", Path: "/r/a.txt"},
	}
}

func TestTreeHidesDotfilesByDefault(t *testing.T) {
	tr := newTree()
	tr.setRoot(sampleRoot())
	if len(tr.visible()) != 2 {
		t.Fatalf("want 2 visible, got %d", len(tr.visible()))
	}
	tr.toggleHidden()
	if len(tr.visible()) != 3 {
		t.Fatalf("want 3 visible after toggle, got %d", len(tr.visible()))
	}
}

func TestTreeExpandCollapse(t *testing.T) {
	tr := newTree()
	tr.setRoot(sampleRoot())
	docs := tr.visible()[0]
	tr.setChildren(docs, []fs.Entry{{Name: "b.md", Path: "/r/docs/b.md"}})

	if !docs.expanded || !docs.loaded {
		t.Fatal("setChildren should expand and mark loaded")
	}
	if len(tr.visible()) != 3 {
		t.Fatalf("want 3 visible when expanded, got %d", len(tr.visible()))
	}
	if tr.visible()[1].entry.Name != "b.md" || tr.visible()[1].depth != 1 {
		t.Fatalf("child should follow parent at depth 1: %+v", tr.visible()[1])
	}

	docs.expanded = false
	tr.reflatten()
	if len(tr.visible()) != 2 {
		t.Fatalf("want 2 visible when collapsed, got %d", len(tr.visible()))
	}
}

func TestTreeCursorMovementAndClamping(t *testing.T) {
	tr := newTree()
	tr.setRoot(sampleRoot())
	if tr.current().entry.Name != "docs" {
		t.Fatalf("cursor should start at first node")
	}
	tr.moveUp() // already at top
	if tr.cursor != 0 {
		t.Fatal("moveUp at top should stay")
	}
	tr.moveDown()
	if tr.current().entry.Name != "a.txt" {
		t.Fatalf("want a.txt, got %s", tr.current().entry.Name)
	}
	tr.moveDown() // already at bottom
	if tr.cursor != 1 {
		t.Fatal("moveDown at bottom should stay")
	}
}

func TestTreeEmpty(t *testing.T) {
	tr := newTree()
	tr.setRoot(nil)
	if tr.current() != nil {
		t.Fatal("current on empty tree should be nil")
	}
	tr.moveDown()
	tr.moveUp() // must not panic
}

func TestTreeParentPointers(t *testing.T) {
	tr := newTree()
	tr.setRoot(sampleRoot())
	// root nodes have nil parent
	for _, n := range tr.roots {
		if n.parent != nil {
			t.Fatalf("root node %q should have nil parent", n.entry.Name)
		}
	}
	docs := tr.roots[0]
	if !docs.entry.IsDir {
		t.Fatal("first root should be docs dir")
	}
	tr.setChildren(docs, []fs.Entry{
		{Name: "b.md", Path: "/r/docs/b.md"},
		{Name: "c.md", Path: "/r/docs/c.md"},
	})
	for _, ch := range docs.children {
		if ch.parent != docs {
			t.Fatalf("child %q should have docs as parent, got %v", ch.entry.Name, ch.parent)
		}
	}
}

// ---- mergeChildren tests ----

func baseTime() time.Time { return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) }

func TestMergePreservesExpansionAndCursor(t *testing.T) {
	tr := newTree()
	tr.setRoot(sampleRoot())
	// Expand docs.
	docs := tr.roots[0]
	tr.setChildren(docs, []fs.Entry{{Name: "b.md", Path: "/r/docs/b.md", Size: 10, ModTime: baseTime()}})
	// Move cursor to b.md (index 1 in the flat list).
	tr.cursor = 1

	now := baseTime().Add(time.Minute)
	// Merge root with same entries (no metadata change) — keep state.
	tr.mergeChildren(nil, sampleRoot(), now)
	if !tr.roots[0].expanded {
		t.Fatal("mergeChildren should preserve expanded state")
	}
	if !tr.roots[0].loaded {
		t.Fatal("mergeChildren should preserve loaded state")
	}
	// cursor was pointing at a position that may have shifted; just confirm no panic and valid range.
	if tr.cursor < 0 || tr.cursor >= len(tr.flat) {
		t.Fatalf("cursor out of range: %d (flat len %d)", tr.cursor, len(tr.flat))
	}
}

func TestMergeMtimeDeltaMarksChanged(t *testing.T) {
	tr := newTree()
	mtime1 := baseTime()
	tr.setRoot([]fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt", Size: 5, ModTime: mtime1},
	})
	// Initial setRoot — changedAt not set.
	if !tr.roots[0].changedAt.IsZero() {
		t.Fatal("setRoot should not mark changedAt")
	}

	// Merge with changed mtime.
	mtime2 := mtime1.Add(time.Second)
	now := mtime2.Add(time.Millisecond)
	tr.mergeChildren(nil, []fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt", Size: 5, ModTime: mtime2},
	}, now)
	if tr.roots[0].changedAt.IsZero() {
		t.Fatal("mergeChildren should mark changedAt when ModTime differs")
	}
	if !tr.roots[0].changedAt.Equal(now) {
		t.Errorf("changedAt want %v, got %v", now, tr.roots[0].changedAt)
	}
}

func TestMergeSizeDeltaMarksChanged(t *testing.T) {
	tr := newTree()
	mtime := baseTime()
	tr.setRoot([]fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt", Size: 5, ModTime: mtime},
	})

	now := mtime.Add(time.Minute)
	tr.mergeChildren(nil, []fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt", Size: 10, ModTime: mtime},
	}, now)
	if tr.roots[0].changedAt.IsZero() {
		t.Fatal("mergeChildren should mark changedAt when Size differs")
	}
}

func TestMergeIdenticalEntriesNotMarked(t *testing.T) {
	tr := newTree()
	mtime := baseTime()
	entries := []fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt", Size: 5, ModTime: mtime},
	}
	tr.setRoot(entries)
	now := mtime.Add(time.Minute)
	// Merge with identical entries — nothing should be marked.
	tr.mergeChildren(nil, entries, now)
	if !tr.roots[0].changedAt.IsZero() {
		t.Fatal("mergeChildren with identical entries should not mark changedAt")
	}
}

func TestMergeNewEntryMarked(t *testing.T) {
	tr := newTree()
	mtime := baseTime()
	tr.setRoot([]fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt", Size: 5, ModTime: mtime},
	})

	now := mtime.Add(time.Minute)
	tr.mergeChildren(nil, []fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt", Size: 5, ModTime: mtime},
		{Name: "b.txt", Path: "/r/b.txt", Size: 3, ModTime: mtime},
	}, now)
	if len(tr.roots) != 2 {
		t.Fatalf("want 2 roots, got %d", len(tr.roots))
	}
	// b.txt is new — should be marked.
	var bNode *node
	for _, n := range tr.roots {
		if n.entry.Name == "b.txt" {
			bNode = n
		}
	}
	if bNode == nil {
		t.Fatal("b.txt not found after merge")
	}
	if bNode.changedAt.IsZero() {
		t.Fatal("new entry should have changedAt set")
	}
	// a.txt unchanged — should NOT be marked.
	for _, n := range tr.roots {
		if n.entry.Name == "a.txt" && !n.changedAt.IsZero() {
			t.Fatal("unchanged a.txt should not be marked")
		}
	}
}

func TestMergeVanishedEntryDropped(t *testing.T) {
	tr := newTree()
	mtime := baseTime()
	tr.setRoot([]fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt", Size: 5, ModTime: mtime},
		{Name: "b.txt", Path: "/r/b.txt", Size: 3, ModTime: mtime},
	})
	tr.cursor = 1 // on b.txt

	now := mtime.Add(time.Minute)
	// b.txt vanishes.
	tr.mergeChildren(nil, []fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt", Size: 5, ModTime: mtime},
	}, now)
	if len(tr.roots) != 1 {
		t.Fatalf("want 1 root after vanish, got %d", len(tr.roots))
	}
	// cursor should clamp.
	if tr.cursor != 0 {
		t.Fatalf("cursor should clamp to 0, got %d", tr.cursor)
	}
}

// ---- expandedDirs tests ----

func TestExpandedDirs(t *testing.T) {
	tr := newTree()
	tr.setRoot(sampleRoot())
	docs := tr.roots[0]
	tr.setChildren(docs, []fs.Entry{
		{Name: "sub", Path: "/r/docs/sub", IsDir: true},
	})
	// docs is expanded+loaded; sub is a dir but not yet loaded.
	dirs := tr.expandedDirs()
	if len(dirs) != 1 || dirs[0] != docs {
		t.Fatalf("want [docs], got %v", dirs)
	}

	// expand sub too.
	sub := docs.children[0]
	tr.setChildren(sub, []fs.Entry{{Name: "c.txt", Path: "/r/docs/sub/c.txt"}})
	dirs = tr.expandedDirs()
	if len(dirs) != 2 {
		t.Fatalf("want 2 expanded dirs, got %d", len(dirs))
	}
}

func TestExpandedDirsEmpty(t *testing.T) {
	tr := newTree()
	tr.setRoot(sampleRoot())
	if dirs := tr.expandedDirs(); len(dirs) != 0 {
		t.Fatalf("want no expanded dirs on fresh tree, got %d", len(dirs))
	}
}

// TestExpandedDirsCollapsedParent verifies that an expanded subdir under a
// COLLAPSED parent is NOT returned by expandedDirs.
func TestExpandedDirsCollapsedParent(t *testing.T) {
	tr := newTree()
	tr.setRoot(sampleRoot())
	docs := tr.roots[0]

	// Load and expand sub under docs, then collapse docs.
	tr.setChildren(docs, []fs.Entry{
		{Name: "sub", Path: "/r/docs/sub", IsDir: true},
	})
	sub := docs.children[0]
	tr.setChildren(sub, []fs.Entry{{Name: "c.txt", Path: "/r/docs/sub/c.txt"}})
	// At this point docs and sub are both expanded.
	dirs := tr.expandedDirs()
	if len(dirs) != 2 {
		t.Fatalf("want 2 expanded dirs (docs+sub), got %d", len(dirs))
	}

	// Collapse docs — sub is still expanded+loaded inside it.
	docs.expanded = false
	tr.reflatten()

	dirs = tr.expandedDirs()
	if len(dirs) != 0 {
		t.Errorf("want 0 expanded dirs when parent is collapsed, got %d: %v", len(dirs), dirs)
	}
}

// ---- recentChanges tests ----

func TestRecentChangesOrdering(t *testing.T) {
	tr := newTree()
	base := baseTime()
	tr.setRoot([]fs.Entry{
		{Name: "a.txt", Path: "/r/a.txt"},
		{Name: "b.txt", Path: "/r/b.txt"},
		{Name: "c.txt", Path: "/r/c.txt"},
	})
	// Manually set changedAt on nodes: all within 45s of now.
	now := base.Add(10 * time.Second)
	tr.roots[0].changedAt = base.Add(8 * time.Second)  // 2s ago
	tr.roots[1].changedAt = base.Add(7 * time.Second)  // 3s ago
	tr.roots[2].changedAt = base.Add(9 * time.Second)  // 1s ago

	within := 45 * time.Second
	changes := tr.recentChanges(within, now)
	if len(changes) != 3 {
		t.Fatalf("want 3 recent changes, got %d", len(changes))
	}
	// Most recent first: c (9s), a (8s), b (7s).
	if changes[0].entry.Name != "c.txt" || changes[1].entry.Name != "a.txt" || changes[2].entry.Name != "b.txt" {
		t.Errorf("wrong order: %v", []string{changes[0].entry.Name, changes[1].entry.Name, changes[2].entry.Name})
	}
}

func TestRecentChangesWithin(t *testing.T) {
	tr := newTree()
	base := baseTime()
	tr.setRoot([]fs.Entry{
		{Name: "old.txt", Path: "/r/old.txt"},
		{Name: "new.txt", Path: "/r/new.txt"},
	})
	tr.roots[0].changedAt = base // old — 60s ago
	tr.roots[1].changedAt = base.Add(50 * time.Second) // recent

	now := base.Add(60 * time.Second)
	within := 45 * time.Second
	changes := tr.recentChanges(within, now)
	if len(changes) != 1 || changes[0].entry.Name != "new.txt" {
		t.Fatalf("want only new.txt in recent changes, got %v", changes)
	}
}
