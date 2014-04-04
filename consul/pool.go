package consul

import (
	"crypto/tls"
	"fmt"
	"github.com/inconshreveable/muxado"
	"github.com/ugorji/go/codec"
	"net"
	"net/rpc"
	"sync"
	"sync/atomic"
	"time"
)

// Conn is a pooled connection to a Consul server
type Conn struct {
	refCount int32
	addr     net.Addr
	session  muxado.Session
	lastUsed time.Time
}

func (c *Conn) Close() error {
	return c.session.Close()
}

// ConnPool is used to maintain a connection pool to other
// Consul servers. This is used to reduce the latency of
// RPC requests between servers. It is only used to pool
// connections in the rpcConsul mode. Raft connections
// are pooled seperately.
type ConnPool struct {
	sync.Mutex

	// The maximum time to keep a connection open
	maxTime time.Duration

	// Pool maps an address to a open connection
	pool map[string]*Conn

	// TLS settings
	tlsConfig *tls.Config

	// Used to indicate the pool is shutdown
	shutdown   bool
	shutdownCh chan struct{}
}

// NewPool is used to make a new connection pool
// Maintain at most one connection per host, for up to maxTime.
// Set maxTime to 0 to disable reaping. If TLS settings are provided
// outgoing connections use TLS.
func NewPool(maxTime time.Duration, tlsConfig *tls.Config) *ConnPool {
	pool := &ConnPool{
		maxTime:    maxTime,
		pool:       make(map[string]*Conn),
		tlsConfig:  tlsConfig,
		shutdownCh: make(chan struct{}),
	}
	if maxTime > 0 {
		go pool.reap()
	}
	return pool
}

// Shutdown is used to close the connection pool
func (p *ConnPool) Shutdown() error {
	p.Lock()
	defer p.Unlock()

	for _, conn := range p.pool {
		conn.Close()
	}
	p.pool = make(map[string]*Conn)

	if p.shutdown {
		return nil
	}
	p.shutdown = true
	close(p.shutdownCh)
	return nil
}

// Acquire is used to get a connection that is
// pooled or to return a new connection
func (p *ConnPool) acquire(addr net.Addr) (*Conn, error) {
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
	c := p.pool[addr.String()]
	if c != nil {
		c.lastUsed = time.Now()
		atomic.AddInt32(&c.refCount, 1)
	}
	return c
}

// getNewConn is used to return a new connection
func (p *ConnPool) getNewConn(addr net.Addr) (*Conn, error) {
	// Try to dial the conn
	conn, err := net.DialTimeout("tcp", addr.String(), 10*time.Second)
	if err != nil {
		return nil, err
	}

	// Cast to TCPConn
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetNoDelay(true)
	}

	// Check if TLS is enabled
	if p.tlsConfig != nil {
		// Switch the connection into TLS mode
		if _, err := conn.Write([]byte{byte(rpcTLS)}); err != nil {
			conn.Close()
			return nil, err
		}

		// Wrap the connection in a TLS client
		conn = tls.Client(conn, p.tlsConfig)
	}

	// Write the Consul multiplex byte to set the mode
	if _, err := conn.Write([]byte{byte(rpcMultiplex)}); err != nil {
		conn.Close()
		return nil, err
	}

	// Create a multiplexed session
	session := muxado.Client(conn)

	// Wrap the connection
	c := &Conn{
		refCount: 1,
		addr:     addr,
		session:  session,
		lastUsed: time.Now(),
	}

	// Monitor the session
	go func() {
		session.Wait()
		p.Lock()
		defer p.Unlock()
		if conn, ok := p.pool[addr.String()]; ok && conn.session == session {
			delete(p.pool, addr.String())
		}
	}()

	// Track this connection, handle potential race condition
	p.Lock()
	defer p.Unlock()
	if existing := p.pool[addr.String()]; existing != nil {
		session.Close()
		return existing, nil
	} else {
		p.pool[addr.String()] = c
		return c, nil
	}
}

// clearConn is used to clear any cached connection, potentially in response to an erro
func (p *ConnPool) clearConn(addr net.Addr) {
	p.Lock()
	defer p.Unlock()
	delete(p.pool, addr.String())
}

// releaseConn is invoked when we are done with a conn to reduce the ref count
func (p *ConnPool) releaseConn(conn *Conn) {
	atomic.AddInt32(&conn.refCount, -1)
}

// RPC is used to make an RPC call to a remote host
func (p *ConnPool) RPC(addr net.Addr, method string, args interface{}, reply interface{}) error {
	retries := 0
START:
	// Try to get a conn first
	conn, err := p.acquire(addr)
	if err != nil {
		return fmt.Errorf("failed to get conn: %v", err)
	}
	defer p.releaseConn(conn)

	// Create a new stream
	stream, err := conn.session.Open()
	if err != nil {
		p.clearConn(addr)

		// Try to redial, possible that the TCP session closed due to timeout
		if retries == 0 {
			retries++
			goto START
		}
		return fmt.Errorf("failed to start stream: %v", err)
	}
	defer stream.Close()

	// Create the RPC client
	cc := codec.GoRpc.ClientCodec(stream, &codec.MsgpackHandle{})
	client := rpc.NewClientWithCodec(cc)

	// Make the RPC call
	err = client.Call(method, args, reply)

	// Fast path the non-error case
	if err == nil {
		return nil
	}

	// If its a network error, nuke the connection
	if _, ok := err.(net.Error); ok {
		p.clearConn(addr)
	}
	return fmt.Errorf("rpc error: %v", err)
}

// Reap is used to close conns open over maxTime
func (p *ConnPool) reap() {
	for !p.shutdown {
		// Sleep for a while
		select {
		case <-time.After(time.Second):
		case <-p.shutdownCh:
			return
		}

		// Reap all old conns
		p.Lock()
		var removed []string
		now := time.Now()
		for host, conn := range p.pool {
			// Skip recently used connections
			if now.Sub(conn.lastUsed) < p.maxTime {
				continue
			}

			// Skip connections with active streams
			if atomic.LoadInt32(&conn.refCount) > 0 {
				continue
			}

			// Close the conn
			conn.Close()

			// Remove from pool
			removed = append(removed, host)
		}
		for _, host := range removed {
			delete(p.pool, host)
		}
		p.Unlock()
	}
}
