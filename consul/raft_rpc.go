package consul

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// RaftLayer implements the raft.StreamLayer interface,
// so that we can use a single RPC layer for Raft and Consul
type RaftLayer struct {
	// Addr is the listener address to return
	addr net.Addr

	// connCh is used to accept connections
	connCh chan net.Conn

	// ConnPool is used to make outbound connections
	connPool *ConnPool

	// Tracks if we are closed
	closed    bool
	closeCh   chan struct{}
	closeLock sync.Mutex
}

// NewRaftLayer is used to initialize a new RaftLayer which can
// be used as a StreamLayer for Raft
func NewRaftLayer(addr net.Addr, pool *ConnPool) *RaftLayer {
	layer := &RaftLayer{
		addr:     addr,
		connCh:   make(chan net.Conn),
		connPool: pool,
		closeCh:  make(chan struct{}),
	}
	return layer
}

// Handoff is used to hand off a connection to the
// RaftLayer. This allows it to be Accept()'ed
func (l *RaftLayer) Handoff(c net.Conn) error {
	select {
	case l.connCh <- c:
		return nil
	case <-l.closeCh:
		return fmt.Errorf("Raft RPC layer closed")
	}
}

// Accept is used to return connection which are
// dialed to be used with the Raft layer
func (l *RaftLayer) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connCh:
		return conn, nil
	case <-l.closeCh:
		return nil, fmt.Errorf("Raft RPC layer closed")
	}
}

// Close is used to stop listening for Raft connections
func (l *RaftLayer) Close() error {
	l.closeLock.Lock()
	defer l.closeLock.Unlock()

	if !l.closed {
		l.closed = true
		close(l.closeCh)
	}
	return nil
}

// Addr is used to return the address of the listener
func (l *RaftLayer) Addr() net.Addr {
	return l.addr
}

// Dial is used to create a new outgoing connection
func (l *RaftLayer) Dial(address string, timeout time.Duration) (net.Conn, error) {
	// Get a net.Addr
	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	// Use the conn pool
	conn, err := l.connPool.Acquire(addr)
	if err != nil {
		return nil, err
	}

	// Discard the Conn wrapper
	return conn.conn, nil
}
