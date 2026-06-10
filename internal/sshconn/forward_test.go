package sshconn

import (
	"fmt"
	"io"
	"net"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/Mapika/portside/internal/testssh"
)

func TestForwarderRoundTrip(t *testing.T) {
	addr := testssh.Start(t)
	conn, err := Dial("test", addr, "tester", nil, ssh.InsecureIgnoreHostKey())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// A local TCP echo server plays the role of the remote service.
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echoLn.Close()
	go func() {
		for {
			c, err := echoLn.Accept()
			if err != nil {
				return
			}
			go func() {
				io.Copy(c, c)
				c.Close()
			}()
		}
	}()
	echoPort := echoLn.Addr().(*net.TCPAddr).Port

	fr := NewForwarder(conn.Client)
	defer fr.CloseAll()
	fw, err := fr.Add(0, echoPort) // 0 = pick a free local port
	if err != nil {
		t.Fatal(err)
	}
	if fw.Local == 0 {
		t.Fatal("local port not resolved")
	}
	if len(fr.List()) != 1 {
		t.Fatalf("want 1 forward, got %d", len(fr.List()))
	}

	c, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", fw.Local))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(c, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "ping" {
		t.Fatalf("want ping, got %q", buf)
	}
	c.Close()

	fr.Stop(fw)
	if len(fr.List()) != 0 {
		t.Fatalf("want 0 forwards after stop, got %d", len(fr.List()))
	}
}

func TestForwarderAddBusyPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	busy := ln.Addr().(*net.TCPAddr).Port

	fr := NewForwarder(nil) // client unused: Add fails before any dialing
	if _, err := fr.Add(busy, 80); err == nil {
		t.Fatal("want error for busy port")
	}
}
