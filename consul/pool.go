package consul

import (
	"container/list"
	"crypto/tls"
	"fmt"
	"github.com/hashicorp/yamux"
	"github.com/inconshreveable/muxado"
	"github.com/ugorji/go/codec"
	"net"
	"net/rpc"
	"sync"
	"sync/atomic"
	"time"
)

// muxSession is used to provide an interface for either muxado or yamux
type muxSession interface {
	Open() (net.Conn, error)
	Close() error
}

type muxadoWrapper struct {
	m muxado.Session
}

func (w *muxadoWrapper) Open() (net.Conn, error) {
	return w.m.Open()
}

func (w *muxadoWrapper) Close() error {
	return w.m.Close()
}

// streamClient is used to wrap a stream with an RPC client
type StreamClient struct {
	stream net.Conn
	client *rpc.Client
}

// Conn is a pooled connection to a Consul server
type Conn struct {
	refCount int32
	addr     net.Addr
	session  muxSession
	lastUsed time.Time
	version  int

	pool *ConnPool

	clients    *list.List
	clientLock sync.Mutex
}

func (c *Conn) Close() error {
	return c.session.Close()
}

// getClient is used to get a cached or new client
func (c *Conn) getClient() (*StreamClient, error) {
	// Check for cached client
	c.clientLock.Lock()
	front := c.clients.Front()
	if front != nil {
		c.clients.Remove(front)
	}
	c.clientLock.Unlock()
	if front != nil {
		return front.Value.(*StreamClient), nil
	}

	// Open a new session
	stream, err := c.session.Open()
	if err != nil {
		return nil, err
	}

	// Create the RPC client
	cc := codec.GoRpc.ClientCodec(stream, &codec.MsgpackHandle{})
	client := rpc.NewClientWithCodec(cc)

	// Return a new stream client
	sc := &StreamClient{
		stream: stream,
		client: client,
	}
	return sc, nil
}

// returnStream is used when done with a stream
// to allow re-use by a future RPC
func (c *Conn) returnClient(client *StreamClient) {
	didSave := false
	c.clientLock.Lock()
	if c.clients.Len() < c.pool.maxStreams {
		c.clients.PushFront(client)
		didSave = true
	}
	c.clientLock.Unlock()
	if !didSave {
		client.stream.Close()
	}
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

	// The maximum number of open streams to keep
	maxStreams int

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
// Set maxTime to 0 to disable reaping. maxStreams is used to control
// the number of idle streams allowed.
// If TLS settings are provided outgoing connections use TLS.
func NewPool(maxTime time.Duration, maxStreams int, tlsConfig *tls.Config) *ConnPool {
	pool := &ConnPool{
		maxTime:    maxTime,
		maxStreams: maxStreams,
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
func (p *ConnPool) acquire(addr net.Addr, version int) (*Conn, error) {
	// Check for a pooled ocnn
	if conn := p.getPooled(addr, version); conn != nil {
		return conn, nil
	}

	// Create a new connection
	return p.getNewConn(addr, version)
}

// getPooled is used to return a pooled connection
func (p *ConnPool) getPooled(addr net.Addr, version int) *Conn {
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
func (p *ConnPool) getNewConn(addr net.Addr, version int) (*Conn, error) {
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

	// Switch the multiplexing based on version
	var session muxSession
	if version < 2 {
		// Write the Consul multiplex byte to set the mode
		if _, err := conn.Write([]byte{byte(rpcMultiplex)}); err != nil {
			conn.Close()
			return nil, err
		}

		// Create a multiplexed session
		session = &muxadoWrapper{muxado.Client(conn)}

	} else {
		// Write the Consul multiplex byte to set the mode
		if _, err := conn.Write([]byte{byte(rpcMultiplexV2)}); err != nil {
			conn.Close()
			return nil, err
		}

		// Create a multiplexed session
		session, _ = yamux.Client(conn, nil)
	}

	// Wrap the connection
	c := &Conn{
		refCount: 1,
		addr:     addr,
		session:  session,
		clients:  list.New(),
		lastUsed: time.Now(),
		version:  version,
		pool:     p,
	}

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

// getClient is used to get a usable client for an address and protocol version
func (p *ConnPool) getClient(addr net.Addr, version int) (*Conn, *StreamClient, error) {
	retries := 0
START:
	// Try to get a conn first
	conn, err := p.acquire(addr, version)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get conn: %v", err)
	}

	// Get a client
	client, err := conn.getClient()
	if err != nil {
		p.clearConn(addr)

		// Try to redial, possible that the TCP session closed due to timeout
		if retries == 0 {
			retries++
			goto START
		}
		return nil, nil, fmt.Errorf("failed to start stream: %v", err)
	}
	return conn, client, nil
}

// RPC is used to make an RPC call to a remote host
func (p *ConnPool) RPC(addr net.Addr, version int, method string, args interface{}, reply interface{}) error {
	conn, sc, err := p.getClient(addr, version)
	defer func() {
		conn.returnClient(sc)
		p.releaseConn(conn)
	}()

	// Make the RPC call
	err = sc.client.Call(method, args, reply)

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
