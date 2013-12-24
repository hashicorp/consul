package consul

import (
	"fmt"
	"github.com/ugorji/go/codec"
	"net"
	"net/rpc"
	"sync"
	"time"
)

// Conn is a pooled connection to a Consul server
type Conn struct {
	addr     net.Addr
	conn     *net.TCPConn
	client   *rpc.Client
	lastUsed time.Time
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

// ConnPool is used to maintain a connection pool to other
// Consul servers. This is used to reduce the latency of
// RPC requests between servers. It is only used to pool
// connections in the rpcConsul mode. Raft connections
// are pooled seperately.
type ConnPool struct {
	sync.Mutex

	// The maximum connectsion to maintain per server
	maxConns int

	// The maximum time to keep a connection open
	maxTime time.Duration

	// Pool maps an address to a list of connections
	pool map[string][]*Conn

	// Used to indicate the pool is shutdown
	shutdown bool
}

// NewPool is used to make a new connection pool
// Maintain at most maxConns per host, for up to maxTime.
// Set maxTime to 0 to disable reaping.
func NewPool(maxConns int, maxTime time.Duration) *ConnPool {
	pool := &ConnPool{
		maxConns: maxConns,
		maxTime:  maxTime,
		pool:     make(map[string][]*Conn),
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

	for _, conns := range p.pool {
		for _, c := range conns {
			c.Close()
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

	// Write the Consul RPC byte to set the mode
	conn.Write([]byte{byte(rpcConsul)})

	// Create the RPC client
	cc := codec.GoRpc.ClientCodec(conn, &codec.MsgpackHandle{})
	client := rpc.NewClientWithCodec(cc)

	// Wrap the connection
	c := &Conn{
		addr:   addr,
		conn:   conn,
		client: client,
	}
	return c, nil
}

// Return is used to return a connection once done. Connections
// that are in an error state should not be returned
func (p *ConnPool) Return(conn *Conn) {
	p.Lock()
	defer p.Unlock()

	// Set the last used time
	conn.lastUsed = time.Now()

	// Look for existing connections
	conns := p.pool[conn.addr.String()]

	// Check for limit on connections or shutdown
	if p.shutdown || len(conns) >= p.maxConns {
		conn.Close()
		return
	}

	// Retain the connection
	conns = append(conns, conn)
	p.pool[conn.addr.String()] = conns
}

// RPC is used to make an RPC call to a remote host
func (p *ConnPool) RPC(addr net.Addr, method string, args interface{}, reply interface{}) error {
	// Try to get a conn first
	conn, err := p.Acquire(addr)
	if err != nil {
		return fmt.Errorf("failed to get conn: %v", err)
	}

	// Make the RPC call
	err = conn.client.Call(method, args, reply)

	// Fast path the non-error case
	if err == nil {
		p.Return(conn)
		return nil
	}

	// If not a network error, save the connection
	if _, ok := err.(net.Error); !ok {
		p.Return(conn)
	} else {
		conn.Close()
	}
	return fmt.Errorf("rpc error: %v", err)
}

// Reap is used to close conns open over maxTime
func (p *ConnPool) reap() {
	for !p.shutdown {
		// Sleep for a while
		time.Sleep(time.Second)

		// Reap all old conns
		p.Lock()
		now := time.Now()
		for host, conns := range p.pool {
			n := len(conns)
			for i := 0; i < n; i++ {
				// Skip new connections
				conn := conns[i]
				if now.Sub(conn.lastUsed) < p.maxTime {
					continue
				}

				// Close the conn
				conn.Close()

				// Remove from pool
				conns[i], conns[n-1] = conns[n-1], nil
				conns = conns[:n-1]
				p.pool[host] = conns
				n--
			}
		}
		p.Unlock()
	}
}
