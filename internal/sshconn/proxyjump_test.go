package sshconn

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/Mapika/portside/internal/testssh"
)

// writeConfig writes an SSH config file with the given content and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProxyJumpNone(t *testing.T) {
	r, err := LoadConfig(writeConfig(t, "Host target\n    HostName 127.0.0.1\n    ProxyJump none\n"))
	if err != nil {
		t.Fatal(err)
	}
	if hops := r.ProxyJump("target"); len(hops) != 0 {
		t.Fatalf("want no hops for ProxyJump none, got %v", hops)
	}
}

func TestProxyJumpEmpty(t *testing.T) {
	r, err := LoadConfig(writeConfig(t, "Host target\n    HostName 127.0.0.1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if hops := r.ProxyJump("target"); len(hops) != 0 {
		t.Fatalf("want no hops when ProxyJump absent, got %v", hops)
	}
}

func TestProxyJumpSingleHop(t *testing.T) {
	r, err := LoadConfig(writeConfig(t, "Host target\n    HostName 127.0.0.1\n    ProxyJump jump1\n"))
	if err != nil {
		t.Fatal(err)
	}
	hops := r.ProxyJump("target")
	if len(hops) != 1 || hops[0] != "jump1" {
		t.Fatalf("want [jump1], got %v", hops)
	}
}

func TestProxyJumpMultipleHops(t *testing.T) {
	r, err := LoadConfig(writeConfig(t, "Host target\n    HostName 127.0.0.1\n    ProxyJump hop1, hop2, hop3\n"))
	if err != nil {
		t.Fatal(err)
	}
	hops := r.ProxyJump("target")
	if len(hops) != 3 || hops[0] != "hop1" || hops[1] != "hop2" || hops[2] != "hop3" {
		t.Fatalf("want [hop1 hop2 hop3], got %v", hops)
	}
}

func TestParseHop(t *testing.T) {
	tests := []struct {
		in               string
		wantUser         string
		wantHost         string
		wantPort         string
	}{
		{"host.example.com", "", "host.example.com", "22"},
		{"user@host.example.com", "user", "host.example.com", "22"},
		{"host.example.com:2222", "", "host.example.com", "2222"},
		{"user@host.example.com:2222", "user", "host.example.com", "2222"},
	}
	for _, tc := range tests {
		u, h, p := parseHop(tc.in)
		if u != tc.wantUser || h != tc.wantHost || p != tc.wantPort {
			t.Errorf("parseHop(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tc.in, u, h, p, tc.wantUser, tc.wantHost, tc.wantPort)
		}
	}
}

// TestConnectViaProxyJump starts two in-process servers (jump and target),
// writes a config with ProxyJump, and verifies Connect reaches the target.
func TestConnectViaProxyJump(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	jumpAddr := testssh.Start(t)
	targetAddr := testssh.Start(t)

	cfg := "Host jump\n    HostName " + hopHost(jumpAddr) + "\n    Port " + hopPort(jumpAddr) + "\n" +
		"Host target\n    HostName " + hopHost(targetAddr) + "\n    Port " + hopPort(targetAddr) + "\n    ProxyJump jump\n"

	r, err := LoadConfig(writeConfig(t, cfg))
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialChain(r, "target", "", ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatalf("dialChain via proxy: %v", err)
	}
	defer conn.Close()

	if conn.Host != "target" {
		t.Fatalf("wrong host: %s", conn.Host)
	}
	if _, err := conn.SFTP.Getwd(); err != nil {
		t.Fatalf("sftp not working: %v", err)
	}
}

// TestConnectViaThreeHops verifies a 3-hop chain works.
func TestConnectViaThreeHops(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	hop1Addr := testssh.Start(t)
	hop2Addr := testssh.Start(t)
	targetAddr := testssh.Start(t)

	cfg := "Host hop1\n    HostName " + hopHost(hop1Addr) + "\n    Port " + hopPort(hop1Addr) + "\n" +
		"Host hop2\n    HostName " + hopHost(hop2Addr) + "\n    Port " + hopPort(hop2Addr) + "\n" +
		"Host target\n    HostName " + hopHost(targetAddr) + "\n    Port " + hopPort(targetAddr) + "\n    ProxyJump hop1,hop2\n"

	r, err := LoadConfig(writeConfig(t, cfg))
	if err != nil {
		t.Fatal(err)
	}

	conn, err := dialChain(r, "target", "", ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatalf("3-hop dialChain: %v", err)
	}
	defer conn.Close()

	if conn.Host != "target" {
		t.Fatalf("wrong host: %s", conn.Host)
	}
}

// TestProxyJumpChainTooLong verifies the depth cap.
func TestProxyJumpChainTooLong(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	// Build a config with 9 hops (over the limit of 8)
	cfg := "Host target\n    HostName 127.0.0.1\n    ProxyJump h1,h2,h3,h4,h5,h6,h7,h8,h9\n"
	r, err := LoadConfig(writeConfig(t, cfg))
	if err != nil {
		t.Fatal(err)
	}
	_, err = dialChain(r, "target", "", ssh.InsecureIgnoreHostKey())
	if err == nil || err.Error() != "proxyjump chain too long" {
		t.Fatalf("want 'proxyjump chain too long', got %v", err)
	}
}

// hopHost/hopPort helpers split an addr into host/port.
func hopHost(addr string) string {
	h, _, _ := splitHostPort(addr)
	return h
}

func hopPort(addr string) string {
	_, p, _ := splitHostPort(addr)
	return p
}

func splitHostPort(addr string) (string, string, error) {
	// simple: we know it's 127.0.0.1:PORT
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], nil
		}
	}
	return addr, "22", nil
}
