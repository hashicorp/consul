// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package internal

import (
	"fmt"
	"net"
)

// Listener implements the net.Listener interface and allows you to manually
// pass connections to it. This is useful when you need to accept connections
// and do something with them yourself first (e.g. handling our multiplexing
// scheme) before giving them to the gRPC server.
type Listener struct {
	addr  net.Addr
	conns chan net.Conn
	done  chan struct{}
}

var _ net.Listener = (*Listener)(nil)

// NewListener creates a Listener with the given address.
func NewListener(addr net.Addr) *Listener {
	return &Listener{
		addr:  addr,
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}

}

// Handle makes the given connection available to Accept.
func (l *Listener) Handle(conn net.Conn) {
	select {
	case l.conns <- conn:
	case <-l.done:
		_ = conn.Close()
	}
}

// Accept a connection.
func (l *Listener) Accept() (net.Conn, error) {
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
func (l *Listener) Addr() net.Addr { return l.addr }

// Close the listener.
func (l *Listener) Close() error {
	close(l.done)
	return nil
}
