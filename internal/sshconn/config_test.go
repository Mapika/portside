package sshconn

import (
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
