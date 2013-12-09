package consul

import (
	"net"
	"sync"
	"time"
)

// Conn is a pooled connection to a Consul server
type Conn struct {
	addr net.Addr
	conn *net.TCPConn
}

// ConnPool is used to maintain a connection pool to other
// Consul servers. This is used to reduce the latency of
// RPC requests between servers
type ConnPool struct {
	sync.Mutex

	// The maximum connectsion to maintain per server
	maxConns int

	// Pool maps an address to a list of connections
	pool map[string][]*Conn

	// Used to indicate the pool is shutdown
	shutdown bool
}

// NewPool is used to make a new connection pool
// Maintain at most maxConns per host
func NewPool(maxConns int) *ConnPool {
	pool := &ConnPool{
		maxConns: maxConns,
		pool:     make(map[string][]*Conn),
	}
	return pool
}

// Shutdown is used to close the connection pool
func (p *ConnPool) Shutdown() error {
	p.Lock()
	defer p.Unlock()

	for _, conns := range p.pool {
		for _, c := range conns {
			c.conn.Close()
		}
	}
	p.pool = make(map[string][]*Conn)
	p.shutdown = true

	return nil
}

// Acquire is used to get a connection that is
// pooled or to return a new connection
func (p *ConnPool) Acquire(addr net.Addr) (*Conn, error) {
	// Check for a pooled ocnn
	if conn := p.getPooled(addr); conn != nil {
		return conn, nil
	}

	// Create a new connection
	return p.getNewConn(addr)
}

// getPooled is used to return a pooled connection
func (p *ConnPool) getPooled(addr net.Addr) *Conn {
	p.Lock()
	defer p.Unlock()

	// Look for an existing connection
	conns := p.pool[addr.String()]
	if len(conns) == 0 {
		return nil
	}

	// Remove the last conn from the pool
	conn := conns[len(conns)-1]
	conns = conns[:len(conns)-1]
	p.pool[addr.String()] = conns
	return conn
}

// getNewConn is used to return a new connection
func (p *ConnPool) getNewConn(addr net.Addr) (*Conn, error) {
	// Try to dial the conn
	rawConn, err := net.DialTimeout("tcp", addr.String(), 10*time.Second)
	if err != nil {
		return nil, err
	}

	// Cast to TCPConn
	conn := rawConn.(*net.TCPConn)

	// Enable keep alives
	conn.SetKeepAlive(true)
	conn.SetNoDelay(true)

	// Wrap the connection
	c := &Conn{
		addr: addr,
		conn: conn,
	}
	return c, nil
}

// Return is used to return a connection once done. Connections
// that are in an error state should not be returned
func (p *ConnPool) Return(conn *Conn) {
	p.Lock()
	defer p.Unlock()

	// Look for existing connections
	conns := p.pool[conn.addr.String()]

	// Check for limit on connections or shutdown
	if p.shutdown || len(conns) >= p.maxConns {
		conn.conn.Close()
		return
	}

	// Retain the connection
	conns = append(conns, conn)
	p.pool[conn.addr.String()] = conns
}
