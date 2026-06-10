package sshconn

import (
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"testing"
)

func loadFixture(t *testing.T) *Resolver {
	t.Helper()
	r, err := LoadConfig("testdata/config")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestHostsSkipsWildcards(t *testing.T) {
	r := loadFixture(t)
	got := r.Hosts()
	want := []string{"web", "db"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestResolveExplicitValues(t *testing.T) {
	r := loadFixture(t)
	p := r.Resolve("web")
	if p.Addr != "web.example.com:2222" {
		t.Fatalf("wrong addr: %s", p.Addr)
	}
	if p.User != "deploy" {
		t.Fatalf("wrong user: %s", p.User)
	}
}

func TestResolveDefaults(t *testing.T) {
	r := loadFixture(t)
	p := r.Resolve("db")
	if p.Addr != "10.0.0.5:22" {
		t.Fatalf("wrong addr: %s", p.Addr)
	}
	if p.User != "fallback" { // inherited from Host *
		t.Fatalf("wrong user: %s", p.User)
	}
}

func TestResolveUnknownAliasFallsBackToAlias(t *testing.T) {
	r := loadFixture(t)
	p := r.Resolve("unknown.example.org")
	if p.Addr != "unknown.example.org:22" {
		t.Fatalf("wrong addr: %s", p.Addr)
	}
	if p.User != "fallback" { // Host * still applies to unknown aliases
		t.Fatalf("wrong user: %s", p.User)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	r, err := LoadConfig("testdata/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if hosts := r.Hosts(); hosts != nil {
		t.Fatalf("want nil hosts, got %v", hosts)
	}
}

func TestResolveUserDefaultsToOSUser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte("Host bare\n    HostName bare.example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	u, err := user.Current()
	if err != nil {
		t.Skip("cannot determine current user")
	}
	p := r.Resolve("bare")
	if p.User != u.Username {
		t.Fatalf("want OS user %q, got %q", u.Username, p.User)
	}
}
