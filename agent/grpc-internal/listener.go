package internal

import (
	"fmt"
	"net"
)

// PipeListener implements the net.Listener interface and allows you to manually
// pass connections to it. This is useful when you need to accept connections
// and do something with them yourself first (e.g. handling our multiplexing
// scheme) before giving them to the gRPC server.
type PipeListener struct {
	addr  net.Addr
	conns chan net.Conn
	done  chan struct{}
}

var _ net.Listener = (*PipeListener)(nil)

// NewPipeListener creates a PipeListener with the given address.
func NewPipeListener(addr net.Addr) *PipeListener {
	return &PipeListener{
		addr:  addr,
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}

}

// Handle makes the given connection available to Accept.
func (l *PipeListener) Handle(conn net.Conn) {
	select {
	case l.conns <- conn:
	case <-l.done:
	}
}

// Accept a connection.
func (l *PipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		return nil, &net.OpError{
			Op:   "accept",
			Net:  l.addr.Network(),
			Addr: l.addr,
			Err:  fmt.Errorf("listener closed"),
		}
	}
}

// Addr returns the listener's address.
func (l *PipeListener) Addr() net.Addr { return l.addr }

// Close the listener.
func (l *PipeListener) Close() error {
	close(l.done)
	return nil
}
