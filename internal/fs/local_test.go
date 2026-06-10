package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalList(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "zsub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := Local{}.List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	// directories sort first even though "zsub" > "a.txt" alphabetically
	if entries[0].Name != "zsub" || !entries[0].IsDir {
		t.Fatalf("want zsub dir first, got %+v", entries[0])
	}
	if entries[0].Path != filepath.Join(dir, "zsub") {
		t.Fatalf("wrong dir path: %s", entries[0].Path)
	}
	if entries[1].Name != "a.txt" || entries[1].IsDir {
		t.Fatalf("want a.txt file second, got %+v", entries[1])
	}
	if entries[1].Path != filepath.Join(dir, "a.txt") {
		t.Fatalf("wrong path: %s", entries[1].Path)
	}
}

func TestLocalListMissingDir(t *testing.T) {
	if _, err := (Local{}).List("/nonexistent-portside-test"); err == nil {
		t.Fatal("want error for missing dir")
	}
}

func TestLocalDownloadFile(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "f.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (Local{}).Download(filepath.Join(src, "f.txt"), dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("want hello, got %q", got)
	}
}

func TestLocalDownloadDirRecursive(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "deep.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (Local{}).Download(src, dest); err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(src)
	got, err := os.ReadFile(filepath.Join(dest, base, "sub", "deep.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "world" {
		t.Fatalf("want world, got %q", got)
	}
}
