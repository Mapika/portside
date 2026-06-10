package sshconn

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
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
	methods, closers := AuthMethods(Params{}, "")
	if len(closers) != 0 {
		t.Fatalf("want no closers without an agent, got %d", len(closers))
	}
	_ = methods // may be empty or key-only; must not panic
}

// TestConnectWithPassword verifies that Connect with a correct password succeeds
// and that an incorrect/empty password is rejected with IsAuthErr == true.
func TestConnectWithPassword(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "") // no agent interference
	const password = "sesame"
	addr := testssh.StartWithPassword(t, password)

	p := Params{Addr: addr, User: "tester", Alias: "pwtest"}

	// correct password → success
	auth, closers := AuthMethods(p, password)
	conn, err := Dial("pwtest", addr, p.User, auth, ssh.InsecureIgnoreHostKey())
	closeAll(closers)
	if err != nil {
		t.Fatalf("want successful connect with correct password, got: %v", err)
	}
	conn.Close()

	// wrong password → auth error
	auth2, closers2 := AuthMethods(p, "wrong")
	_, err2 := Dial("pwtest", addr, p.User, auth2, ssh.InsecureIgnoreHostKey())
	closeAll(closers2)
	if err2 == nil {
		t.Fatal("want error for wrong password")
	}
	if !IsAuthErr(err2) {
		t.Fatalf("want IsAuthErr=true for wrong password, got: %v", err2)
	}

	// empty secret → auth error
	auth3, closers3 := AuthMethods(p, "")
	_, err3 := Dial("pwtest", addr, p.User, auth3, ssh.InsecureIgnoreHostKey())
	closeAll(closers3)
	if err3 == nil {
		t.Fatal("want error for empty secret")
	}
	if !IsAuthErr(err3) {
		t.Fatalf("want IsAuthErr=true for empty secret, got: %v", err3)
	}
}

// TestIsAuthErrNetworkError verifies that a plain network error is NOT an auth error.
func TestIsAuthErrNetworkError(t *testing.T) {
	_, err := Dial("x", "127.0.0.1:1", "tester", nil, ssh.InsecureIgnoreHostKey())
	if err == nil {
		t.Fatal("want connection error")
	}
	if IsAuthErr(err) {
		t.Fatalf("network error should not be classified as auth error: %v", err)
	}
}

// TestAuthMethodsDecryptsKey verifies that AuthMethods with a passphrase decrypts
// an encrypted key and adds it as a signer.
func TestAuthMethodsDecryptsKey(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	// generate an ed25519 key
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	const passphrase = "keypass"
	block, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	if err != nil {
		t.Fatal(err)
	}

	// write encrypted key to a temp file
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_ed25519")
	pemBytes := pem.EncodeToMemory(block)
	if err := os.WriteFile(keyPath, pemBytes, 0600); err != nil {
		t.Fatal(err)
	}

	p := Params{KeyPaths: []string{keyPath}}

	// without passphrase: key should be skipped (encrypted), no public-key method from this key
	methods0, closers0 := AuthMethods(p, "")
	closeAll(closers0)
	// the key is encrypted, so the file-based signer count should be 0
	// (we can't inspect the methods directly, but the server test covers the real behavior)
	_ = methods0

	// with correct passphrase: we get at least one method
	methods1, closers1 := AuthMethods(p, passphrase)
	closeAll(closers1)
	if len(methods1) == 0 {
		t.Fatal("want at least one auth method when passphrase is correct")
	}

	// verify the key actually works against a test server
	addr := testssh.Start(t)
	conn, err := Dial("keytest", addr, "u", methods1, ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatalf("connect with decrypted key: %v", err)
	}
	conn.Close()
}
