package sshconn

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Conn is a live SSH connection plus its SFTP client.
type Conn struct {
	Host   string
	Client *ssh.Client
	SFTP   *sftp.Client
}

func (c *Conn) Close() error {
	var errs []error
	if c.SFTP != nil {
		if err := c.SFTP.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.Client != nil {
		if err := c.Client.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Dial opens an SSH connection and an SFTP session over it.
func Dial(host, addr, usr string, auth []ssh.AuthMethod, hk ssh.HostKeyCallback) (*Conn, error) {
	cfg := &ssh.ClientConfig{
		User:            usr,
		Auth:            auth,
		HostKeyCallback: hk,
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	sftpc, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		return nil, err
	}
	return &Conn{Host: host, Client: client, SFTP: sftpc}, nil
}

// Connect resolves an alias from the user's ssh config and dials it using
// the SSH agent and any unencrypted private keys.
func Connect(alias string) (*Conn, error) {
	r, err := LoadConfig(DefaultConfigPath())
	if err != nil {
		return nil, err
	}
	p := r.Resolve(alias)
	auth, closers := AuthMethods(p)
	conn, err := Dial(alias, p.Addr, p.User, auth, hostKeyCallback())
	for _, c := range closers {
		c.Close()
	}
	return conn, err
}

// AuthMethods builds auth from the SSH agent (if running) and the key files
// in p. Encrypted key files are skipped — use the agent for those. The
// returned closers must be closed once dialing has completed.
func AuthMethods(p Params) ([]ssh.AuthMethod, []io.Closer) {
	var methods []ssh.AuthMethod
	var closers []io.Closer
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
			closers = append(closers, conn)
		}
	}
	var signers []ssh.Signer
	for _, kp := range p.KeyPaths {
		data, err := os.ReadFile(kp)
		if err != nil {
			continue
		}
		s, err := ssh.ParsePrivateKey(data)
		if err != nil {
			continue
		}
		signers = append(signers, s)
	}
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}
	return methods, closers
}

func hostKeyCallback() ssh.HostKeyCallback {
	if home, err := os.UserHomeDir(); err == nil {
		kh := filepath.Join(home, ".ssh", "known_hosts")
		if cb, err := knownhosts.New(kh); err == nil {
			return cb
		}
	}
	return ssh.InsecureIgnoreHostKey()
}
