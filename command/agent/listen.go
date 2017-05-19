package agent

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"
)

func ListenTCP(addr string) (net.Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	l = tcpKeepAliveListener{l.(*net.TCPListener)}
	return l, nil
}

func ListenTLS(addr string, cfg *tls.Config) (net.Listener, error) {
	l, err := ListenTCP(addr)
	if err != nil {
		return nil, err
	}
	return tls.NewListener(l, cfg), nil
}

func ListenUnix(addr string, perm FilePermissions) (net.Listener, error) {
	// todo(fs): move this somewhere else
	//	if _, err := os.Stat(addr); !os.IsNotExist(err) {
	//		s.agent.logger.Printf("[WARN] agent: Replacing socket %q", addr)
	//	}
	if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error removing socket file: %s", err)
	}
	l, err := net.Listen("unix", addr)
	if err != nil {
		return nil, err
	}
	if err := setFilePermissions(addr, perm); err != nil {
		return nil, fmt.Errorf("Failed setting up HTTP socket: %s", err)
	}
	return l, nil
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by NewHttpServer so
// dead TCP connections eventually go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(30 * time.Second)
	return tc, nil
}
