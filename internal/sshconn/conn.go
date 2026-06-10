package sshconn

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	// hops holds intermediate hop clients that must be closed in reverse order
	// when the connection is torn down.
	hops []*ssh.Client
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
	// close hops in reverse order (outermost first)
	for i := len(c.hops) - 1; i >= 0; i-- {
		if err := c.hops[i].Close(); err != nil {
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

// Connect resolves an alias from the user's ssh config and dials it (through
// any ProxyJump hops) using the SSH agent and key files. If secret is
// non-empty it also tries encrypted keys, password, and keyboard-interactive.
func Connect(alias, secret string) (*Conn, error) {
	r, err := LoadConfig(DefaultConfigPath())
	if err != nil {
		return nil, err
	}
	return dialChain(r, alias, secret, hostKeyCallback())
}

// dialChain connects to alias through any configured ProxyJump hops.
func dialChain(r *Resolver, alias, secret string, hk ssh.HostKeyCallback) (*Conn, error) {
	hops := r.ProxyJump(alias)
	if len(hops) > 8 {
		return nil, errors.New("proxyjump chain too long")
	}

	var hopClients []*ssh.Client
	var prev *ssh.Client

	for _, hop := range hops {
		p, err := r.resolveHop(hop)
		if err != nil {
			for i := len(hopClients) - 1; i >= 0; i-- {
				hopClients[i].Close()
			}
			return nil, err
		}
		auth, closers := AuthMethods(p, secret)
		client, err := dialMaybeVia(prev, p.Addr, p.User, auth, hk)
		closeAll(closers)
		if err != nil {
			// close already-opened hops before returning
			for i := len(hopClients) - 1; i >= 0; i-- {
				hopClients[i].Close()
			}
			return nil, fmt.Errorf("via %s: %w", hop, err)
		}
		hopClients = append(hopClients, client)
		prev = client
	}

	p := r.Resolve(alias)
	auth, closers := AuthMethods(p, secret)
	client, err := dialMaybeVia(prev, p.Addr, p.User, auth, hk)
	closeAll(closers)
	if err != nil {
		for i := len(hopClients) - 1; i >= 0; i-- {
			hopClients[i].Close()
		}
		return nil, err
	}

	sftpc, err := sftp.NewClient(client)
	if err != nil {
		client.Close()
		for i := len(hopClients) - 1; i >= 0; i-- {
			hopClients[i].Close()
		}
		return nil, err
	}
	return &Conn{Host: alias, Client: client, SFTP: sftpc, hops: hopClients}, nil
}

// dialMaybeVia dials directly when via == nil, else tunnels through via.
func dialMaybeVia(via *ssh.Client, addr, user string, auth []ssh.AuthMethod, hk ssh.HostKeyCallback) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		HostKeyCallback: hk,
		Timeout:         10 * time.Second,
	}
	if via == nil {
		return ssh.Dial("tcp", addr, cfg)
	}
	conn, err := via.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// IsAuthErr reports whether err is an SSH authentication failure (as opposed
// to a network or other error).
func IsAuthErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods remain")
}

// AuthMethods builds auth from the SSH agent (if running) and the key files
// in p. When secret is non-empty, encrypted key files are also decrypted
// using the secret, and password + keyboard-interactive methods are appended.
// Unencrypted keys that fail to parse are still skipped. The returned closers
// must be closed once dialing has completed.
func AuthMethods(p Params, secret string) ([]ssh.AuthMethod, []io.Closer) {
	var methods []ssh.AuthMethod
	var closers []io.Closer
	if conn, err := dialAgent(); err == nil {
		methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		closers = append(closers, conn)
	}
	var signers []ssh.Signer
	for _, kp := range p.KeyPaths {
		data, err := os.ReadFile(kp)
		if err != nil {
			continue
		}
		// try plain (unencrypted) parse first
		s, err := ssh.ParsePrivateKey(data)
		if err != nil {
			// if a secret is provided, attempt to decrypt the key
			if secret != "" {
				s2, err2 := ssh.ParsePrivateKeyWithPassphrase(data, []byte(secret))
				if err2 == nil {
					signers = append(signers, s2)
				}
			}
			continue
		}
		signers = append(signers, s)
	}
	if len(signers) > 0 {
		methods = append(methods, ssh.PublicKeys(signers...))
	}
	if secret != "" {
		methods = append(methods, ssh.Password(secret))
		methods = append(methods, ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = secret
			}
			return answers, nil
		}))
	}
	return methods, closers
}

// closeAll closes every closer, discarding errors.
func closeAll(closers []io.Closer) {
	for _, c := range closers {
		c.Close()
	}
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
