package ui

import (
	"testing"

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
