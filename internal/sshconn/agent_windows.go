//go:build windows

package sshconn

import (
	"net"

	"github.com/Microsoft/go-winio"
)

// dialAgent connects to the Windows OpenSSH agent service's named pipe.
func dialAgent() (net.Conn, error) {
	return winio.DialPipe(`\\.\pipe\openssh-ssh-agent`, nil)
}
