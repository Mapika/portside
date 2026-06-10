package sshconn

import (
	"fmt"
	"io"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Forward is one active local→remote port forward (the ssh -L equivalent).
type Forward struct {
	Local  int
	Remote int
	ln     net.Listener
}

// Forwarder manages port forwards over one SSH connection.
type Forwarder struct {
	client *ssh.Client
	mu     sync.Mutex
	fws    []*Forward
}

func NewForwarder(client *ssh.Client) *Forwarder {
	return &Forwarder{client: client}
}

// Add listens on 127.0.0.1:local (0 picks a free port) and tunnels every
// connection to 127.0.0.1:remote on the server side.
func (fr *Forwarder) Add(local, remote int) (*Forward, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", local))
	if err != nil {
		return nil, err
	}
	fw := &Forward{
		Local:  ln.Addr().(*net.TCPAddr).Port,
		Remote: remote,
		ln:     ln,
	}
	fr.mu.Lock()
	fr.fws = append(fr.fws, fw)
	fr.mu.Unlock()
	go fr.serve(fw)
	return fw, nil
}

func (fr *Forwarder) serve(fw *Forward) {
	for {
		conn, err := fw.ln.Accept()
		if err != nil {
			return // listener closed by Stop/CloseAll
		}
		go func() {
			defer conn.Close()
			remote, err := fr.client.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", fw.Remote))
			if err != nil {
				return
			}
			defer remote.Close()
			go io.Copy(remote, conn)
			io.Copy(conn, remote)
		}()
	}
}

func (fr *Forwarder) Stop(fw *Forward) {
	fw.ln.Close()
	fr.mu.Lock()
	defer fr.mu.Unlock()
	for i, f := range fr.fws {
		if f == fw {
			fr.fws = append(fr.fws[:i], fr.fws[i+1:]...)
			break
		}
	}
}

func (fr *Forwarder) List() []*Forward {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	return append([]*Forward(nil), fr.fws...)
}

func (fr *Forwarder) CloseAll() {
	fr.mu.Lock()
	fws := append([]*Forward(nil), fr.fws...)
	fr.fws = nil
	fr.mu.Unlock()
	for _, f := range fws {
		f.ln.Close()
	}
}
