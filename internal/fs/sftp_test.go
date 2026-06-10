package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/Mapika/portside/internal/testssh"
)

func newTestSFTP(t *testing.T) *SFTP {
	t.Helper()
	addr := testssh.Start(t)
	cfg := &ssh.ClientConfig{User: "test", HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	sc, err := sftp.NewClient(client)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sc.Close() })
	return NewSFTP(sc, "testhost")
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
