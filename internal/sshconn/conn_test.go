package sshconn

import (
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/Mapika/portside/internal/testssh"
)

func TestDialOpensSSHAndSFTP(t *testing.T) {
	addr := testssh.Start(t)
	conn, err := Dial("testhost", addr, "tester", nil, ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if conn.Host != "testhost" {
		t.Fatalf("wrong host: %s", conn.Host)
	}
	if _, err := conn.SFTP.Getwd(); err != nil {
		t.Fatalf("sftp not working: %v", err)
	}
}

func TestDialBadAddr(t *testing.T) {
	if _, err := Dial("x", "127.0.0.1:1", "tester", nil, ssh.InsecureIgnoreHostKey()); err == nil {
		t.Fatal("want connection error")
	}
}

func TestAuthMethodsWithoutAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "") // no agent reachable on any platform
	methods, closers := AuthMethods(Params{})
	if len(closers) != 0 {
		t.Fatalf("want no closers without an agent, got %d", len(closers))
	}
	_ = methods // may be empty or key-only; must not panic
}
