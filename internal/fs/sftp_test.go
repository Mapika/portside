package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/Mapika/portside/internal/testssh"
)

func newTestSSHClient(t *testing.T) *ssh.Client {
	t.Helper()
	addr := testssh.Start(t)
	cfg := &ssh.ClientConfig{User: "test", HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func newTestSFTP(t *testing.T) *SFTP {
	t.Helper()
	client := newTestSSHClient(t)
	sc, err := sftp.NewClient(client)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sc.Close() })
	return NewSFTP(sc, "testhost")
}

func newTestSFTPWithExec(t *testing.T) *SFTP {
	t.Helper()
	client := newTestSSHClient(t)
	sc, err := sftp.NewClient(client)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sc.Close() })
	return NewSFTPWithExec(sc, client, "testhost")
}

func TestSFTPNameAndHome(t *testing.T) {
	s := newTestSFTP(t)
	if s.Name() != "testhost" {
		t.Fatalf("wrong name: %s", s.Name())
	}
	if _, err := s.Home(); err != nil {
		t.Fatal(err)
	}
}

func TestSFTPList(t *testing.T) {
	s := newTestSFTP(t)
	dir := t.TempDir() // server is in-process, so it sees the same filesystem
	if err := os.MkdirAll(filepath.Join(dir, "zsub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := s.List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name != "zsub" || !entries[0].IsDir {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestSFTPUploadFileAndDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		// the in-process test server serves Windows paths, but SFTP ops treat
		// remote paths as POSIX — which is correct for real remotes (Linux
		// servers). The scenario "SFTP to a Windows server" is out of scope for v1.
		t.Skip("test server serves Windows paths; remote paths are POSIX by design")
	}
	s := newTestSFTP(t)
	src := t.TempDir()
	destDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "up.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// upload single file
	if err := s.Upload(filepath.Join(src, "up.txt"), destDir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, "up.txt"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("sftp upload file failed: %q %v", got, err)
	}

	// upload directory recursively
	sub := filepath.Join(src, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "deep.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(src)
	if err := s.Upload(src, destDir); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(filepath.Join(destDir, base, "subdir", "deep.txt"))
	if err != nil || string(got) != "world" {
		t.Fatalf("sftp upload dir recursive failed: %q %v", got, err)
	}
}

func TestSFTPRename(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test server serves Windows paths; remote paths are POSIX by design")
	}
	s := newTestSFTP(t)
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(oldPath, []byte("r"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(oldPath, "new.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldPath); err == nil {
		t.Fatal("old.txt should not exist after rename")
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); err != nil {
		t.Fatalf("new.txt should exist: %v", err)
	}
}

func TestSFTPRenameTraversalRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test server serves Windows paths; remote paths are POSIX by design")
	}
	s := newTestSFTP(t)
	dir := t.TempDir()
	orig := filepath.Join(dir, "orig.txt")
	if err := os.WriteFile(orig, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := []string{"../evil", "a/b", "", "."}
	for _, name := range bad {
		err := s.Rename(orig, name)
		if err == nil {
			t.Errorf("SFTP Rename with %q should return error", name)
		}
		// original must still exist
		if _, statErr := os.Stat(orig); statErr != nil {
			t.Errorf("original file gone after rejected SFTP rename with %q", name)
		}
	}
}

func TestSFTPRemove(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test server serves Windows paths; remote paths are POSIX by design")
	}
	s := newTestSFTP(t)
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(sub); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sub); err == nil {
		t.Fatal("dir should be gone after remove")
	}
}

func TestSFTPMkdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test server serves Windows paths; remote paths are POSIX by design")
	}
	s := newTestSFTP(t)
	dir := t.TempDir()
	newDir := filepath.Join(dir, "newdir")
	if err := s.Mkdir(newDir); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(newDir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("expected directory to exist: %v", err)
	}
}

func TestSFTPDownloadFileAndDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		// the in-process test server serves Windows paths, but Download
		// treats remote paths as POSIX — which is correct for real remotes
		// (Linux servers). The scenario "SFTP to a Windows server" is out
		// of scope for v1.
		t.Skip("test server serves Windows paths; remote paths are POSIX by design")
	}
	s := newTestSFTP(t)
	src := t.TempDir()
	dest := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	// single file
	if err := s.Download(filepath.Join(src, "a.txt"), dest); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "a.txt"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("file download failed: %q %v", got, err)
	}

	// whole directory, recursively
	if err := s.Download(src, dest); err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(src)
	got, err = os.ReadFile(filepath.Join(dest, base, "sub", "b.txt"))
	if err != nil || string(got) != "world" {
		t.Fatalf("dir download failed: %q %v", got, err)
	}
}

// ---- Metadata (Size + ModTime) ----

func TestSFTPListMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test server serves Windows paths; remote paths are POSIX by design")
	}
	s := newTestSFTP(t)
	dir := t.TempDir()
	content := []byte("sftp metadata test")
	fpath := filepath.Join(dir, "meta.txt")
	before := time.Now().Truncate(time.Second)
	if err := os.WriteFile(fpath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Add(time.Second)

	entries, err := s.List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Size != int64(len(content)) {
		t.Errorf("want Size %d, got %d", len(content), e.Size)
	}
	if e.ModTime.Before(before) || e.ModTime.After(after) {
		t.Errorf("ModTime %v not in expected range [%v, %v]", e.ModTime, before, after)
	}
}

// ---- Exec ----

func TestSFTPExec(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exec test uses /bin/echo which is POSIX-only")
	}
	s := newTestSFTPWithExec(t)
	out, err := s.Exec("/bin/echo", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hi\n" {
		t.Errorf("want %q, got %q", "hi\n", string(out))
	}
}

func TestSFTPExecWithoutClient(t *testing.T) {
	s := newTestSFTP(t) // plain NewSFTP — no ssh.Client
	_, err := s.Exec("/bin/echo", "hi")
	if err == nil {
		t.Fatal("want error when no ssh.Client provided")
	}
}
