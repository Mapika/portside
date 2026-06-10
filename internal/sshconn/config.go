// Package sshconn manages SSH config resolution, connections, and port
// forwards for portside.
package sshconn

import (
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	ssh_config "github.com/kevinburke/ssh_config"
)

// Params holds everything needed to dial a host.
type Params struct {
	Alias    string
	Addr     string // host:port
	User     string
	KeyPaths []string // existing private key files to try
}

type Resolver struct {
	cfg *ssh_config.Config
}

func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "config")
}

// LoadConfig parses an ssh config file. A missing file yields an empty
// (but usable) Resolver.
func LoadConfig(path string) (*Resolver, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Resolver{}, nil
		}
		return nil, err
	}
	defer f.Close()
	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, err
	}
	return &Resolver{cfg: cfg}, nil
}

// Hosts returns concrete host aliases (wildcard patterns are skipped).
func (r *Resolver) Hosts() []string {
	if r.cfg == nil {
		return nil
	}
	var out []string
	for _, h := range r.cfg.Hosts {
		for _, p := range h.Patterns {
			s := p.String()
			if s == "" || strings.ContainsAny(s, "*?!") {
				continue
			}
			out = append(out, s)
		}
	}
	return out
}

func (r *Resolver) get(alias, key string) string {
	if r.cfg == nil {
		return ""
	}
	v, err := r.cfg.Get(alias, key)
	if err != nil {
		return ""
	}
	return v
}

// Resolve computes connection parameters for an alias, falling back to the
// alias as hostname, port 22, the current OS user, and standard key files.
func (r *Resolver) Resolve(alias string) Params {
	hostname := r.get(alias, "HostName")
	if hostname == "" {
		hostname = alias
	}
	port := r.get(alias, "Port")
	if port == "" {
		port = "22"
	}
	usr := r.get(alias, "User")
	if usr == "" {
		if u, err := user.Current(); err == nil {
			usr = u.Username
		}
	}

	var keys []string
	if r.cfg != nil {
		if ks, err := r.cfg.GetAll(alias, "IdentityFile"); err == nil {
			keys = ks
		}
	}
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		keys = append(keys, filepath.Join(home, ".ssh", name))
	}
	var existing []string
	seen := map[string]bool{}
	for _, k := range keys {
		k = expandHome(k, home)
		if seen[k] {
			continue
		}
		seen[k] = true
		if _, err := os.Stat(k); err == nil {
			existing = append(existing, k)
		}
	}

	return Params{
		Alias:    alias,
		Addr:     net.JoinHostPort(hostname, port),
		User:     usr,
		KeyPaths: existing,
	}
}

func expandHome(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
