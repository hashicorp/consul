// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package internal

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
)

// ErrPipeClosed is returned when calling Accept or DialContext on a closed
// PipeListener.
var ErrPipeClosed = errors.New("pipe listener has been closed")

// PipeListener implements the net.Listener interface using a net.Pipe so that
// you can interact with a gRPC service in the same process without going over
// the network.
type PipeListener struct {
	conns  chan net.Conn
	closed atomic.Bool
	done   chan struct{}
}

var _ net.Listener = (*PipeListener)(nil)

// NewPipeListener creates a new PipeListener.
func NewPipeListener() *PipeListener {
	return &PipeListener{
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
	}
}

// Accept a connection.
func (p *PipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-p.conns:
		return conn, nil
	case <-p.done:
		return nil, ErrPipeClosed
	}
}

// Close the listener.
func (p *PipeListener) Close() error {
	if p.closed.CompareAndSwap(false, true) {
		close(p.done)
	}
	return nil
}

// DialContext dials the server over an in-process pipe.
func (p *PipeListener) DialContext(ctx context.Context, _ string) (net.Conn, error) {
	if p.closed.Load() {
		return nil, ErrPipeClosed
	}

	serverConn, clientConn := net.Pipe()

	select {
	// Send the server connection to whatever is accepting connections from the
	// PipeListener. This will block until something has accepted the conn.
	case p.conns <- serverConn:
		return clientConn, nil
	case <-ctx.Done():
		serverConn.Close()
		clientConn.Close()
		return nil, ctx.Err()
	case <-p.done:
		serverConn.Close()
		clientConn.Close()
		return nil, ErrPipeClosed
	}
}

// Add returns the listener's address.
func (*PipeListener) Addr() net.Addr { return pipeAddr{} }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "pipe" }
