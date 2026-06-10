//go:build !windows

package sshconn

import (
	"errors"
	"net"
	"os"
)

// dialAgent connects to the OpenSSH agent via the unix socket in
// SSH_AUTH_SOCK.
func dialAgent() (net.Conn, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK not set")
	}
	return net.Dial("unix", sock)
}
