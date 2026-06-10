// Package testssh runs an in-process SSH server (with SFTP and direct-tcpip
// support) for integration tests. It serves the real local filesystem and
// accepts any authentication.
package testssh

import (
	"net"
	"os/exec"
	"testing"

	gssh "github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
)

// Start launches the server and returns its address (host:port). It is shut
// down automatically when the test finishes.
func Start(t *testing.T) string {
	t.Helper()
	return startServer(t, nil)
}

// StartWithPassword launches a server that requires password authentication
// accepting exactly the given password. It returns the address (host:port).
// The server is shut down automatically when the test finishes.
func StartWithPassword(t *testing.T, password string) string {
	t.Helper()
	handler := func(ctx gssh.Context, pass string) bool {
		return pass == password
	}
	return startServer(t, handler)
}

func startServer(t *testing.T, pwHandler gssh.PasswordHandler) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &gssh.Server{
		Handler: func(s gssh.Session) {
			cmd := s.Command()
			if len(cmd) == 0 {
				return
			}
			c := exec.Command(cmd[0], cmd[1:]...) //nolint:gosec // test-only
			c.Stdout = s
			c.Stderr = s.Stderr()
			if err := c.Run(); err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					s.Exit(exit.ExitCode())
				} else {
					s.Exit(1)
				}
				return
			}
			s.Exit(0)
		},
		LocalPortForwardingCallback: func(ctx gssh.Context, host string, port uint32) bool {
			return true
		},
		ChannelHandlers: map[string]gssh.ChannelHandler{
			"session":      gssh.DefaultSessionHandler,
			"direct-tcpip": gssh.DirectTCPIPHandler,
		},
		SubsystemHandlers: map[string]gssh.SubsystemHandler{
			"sftp": func(s gssh.Session) {
				server, err := sftp.NewServer(s)
				if err != nil {
					return
				}
				server.Serve()
			},
		},
		PasswordHandler: pwHandler,
	}
	go srv.Serve(ln)
	t.Cleanup(func() { srv.Close() })
	return ln.Addr().String()
}
