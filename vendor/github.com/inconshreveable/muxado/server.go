package muxado

import (
	"crypto/tls"
	"github.com/inconshreveable/muxado/proto"
	"github.com/inconshreveable/muxado/proto/ext"
	"net"
)

// A Listener accepts new connections from its net.Listener
// and begins muxado server connections on them.
//
// It's API is very similar to a net.Listener, but it returns
// muxado.Sessions instead of net.Conn's.
type Listener struct {
	wrapped net.Listener
}

// Accept the next connection from the listener and begin
// a muxado session on it.
func (l *Listener) Accept() (Session, error) {
	conn, err := l.wrapped.Accept()
	if err != nil {
		return nil, err
	}

	return Server(conn), nil
}

// Addr returns the bound address of the wrapped net.Listener
func (l *Listener) Addr() net.Addr {
	return l.wrapped.Addr()
}

// Close closes the wrapped net.Listener
func (l *Listener) Close() error {
	return l.wrapped.Close()
}

// Server returns a muxado server session using conn as the transport.
func Server(conn net.Conn) Session {
	return &sessionAdaptor{proto.NewSession(conn, proto.NewStream, false, []proto.Extension{ext.NewDefaultHeartbeat()})}
}

// Listen binds to a network address and returns a Listener which accepts
// new connections and starts muxado server sessions on them.
func Listen(network, addr string) (*Listener, error) {
	l, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}

	return &Listener{l}, nil
}

// ListenTLS binds to a network address and accepts new TLS-encrypted connections.
// It returns a Listener which starts new muxado server sessions on the connections.
func ListenTLS(network, addr string, tlsConfig *tls.Config) (*Listener, error) {
	l, err := tls.Listen(network, addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	return &Listener{l}, nil
}

// NewListener creates a new muxado listener which creates new muxado server sessions
// by accepting connections from the given net.Listener
func NewListener(l net.Listener) *Listener {
	return &Listener{l}
}
