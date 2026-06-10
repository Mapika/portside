// Package sshconn manages SSH config resolution, connections, and port
// forwards for portside.
package sshconn

import (
	"errors"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/kevinburke/ssh_config"
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
		if errors.Is(err, os.ErrNotExist) {
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
// If a Host stanza declares multiple patterns, each non-wildcard pattern appears as its own entry.
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

// ProxyJump returns the ordered list of hop aliases/literals for the given
// alias. Returns nil when ProxyJump is absent, empty, or "none".
func (r *Resolver) ProxyJump(alias string) []string {
	raw := r.get(alias, "ProxyJump")
	if raw == "" || strings.EqualFold(raw, "none") {
		return nil
	}
	parts := strings.Split(raw, ",")
	var hops []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			hops = append(hops, p)
		}
	}
	return hops
}

// resolveHop resolves a hop specifier (alias or [user@]host[:port] literal)
// into Params. If the specifier matches a config Host alias, it uses that
// alias's resolved parameters; otherwise it parses the literal.
func (r *Resolver) resolveHop(hop string) Params {
	// check if this matches a config alias
	if r.cfg != nil {
		for _, h := range r.cfg.Hosts {
			for _, pat := range h.Patterns {
				s := pat.String()
				if s == hop && !strings.ContainsAny(s, "*?!") {
					return r.Resolve(hop)
				}
			}
		}
	}
	// treat as literal [user@]host[:port]
	hopUser, hopHost, hopPort := parseHop(hop)
	addr := net.JoinHostPort(hopHost, hopPort)
	usr := hopUser
	if usr == "" {
		if u, err := user.Current(); err == nil {
			usr = u.Username
		}
	}
	home, _ := os.UserHomeDir()
	var keys []string
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		k := filepath.Join(home, ".ssh", name)
		if _, err := os.Stat(k); err == nil {
			keys = append(keys, k)
		}
	}
	return Params{Alias: hop, Addr: addr, User: usr, KeyPaths: keys}
}

// parseHop parses a [user@]host[:port] hop literal. If port is absent,
// "22" is returned.
func parseHop(s string) (user, host, port string) {
	// strip user@ prefix
	if idx := strings.Index(s, "@"); idx >= 0 {
		user = s[:idx]
		s = s[idx+1:]
	}
	// strip :port suffix — handle IPv6 brackets
	if strings.HasPrefix(s, "[") {
		// [host]:port form
		end := strings.Index(s, "]")
		if end >= 0 {
			host = s[1:end]
			rest := s[end+1:]
			if strings.HasPrefix(rest, ":") {
				port = rest[1:]
			}
		}
	} else if idx := strings.LastIndex(s, ":"); idx >= 0 {
		host = s[:idx]
		port = s[idx+1:]
	} else {
		host = s
	}
	if port == "" {
		port = "22"
	}
	return
}

func expandHome(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}
