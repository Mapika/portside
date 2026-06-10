// Demo SSH server used to record demo.gif: serves the current directory
// over SFTP on 127.0.0.1:2222 and accepts any credentials. Only meant for
// the demo recording — don't run it anywhere that matters.
package main

import (
	"log"

	gssh "github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
)

func main() {
	srv := &gssh.Server{
		Addr:    "127.0.0.1:2222",
		Handler: func(s gssh.Session) {},
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
	}
	log.Println("demo ssh server on", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
