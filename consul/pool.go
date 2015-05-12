package consul

import (
	"container/list"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-msgpack/codec"
	"github.com/hashicorp/yamux"
	"github.com/inconshreveable/muxado"
)

// msgpackHandle is a shared handle for encoding/decoding of RPC messages
var msgpackHandle = &codec.MsgpackHandle{}

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

func (sc *StreamClient) Close() {
	sc.stream.Close()
	sc.client.Close()
}

// Conn is a pooled connection to a Consul server
type Conn struct {
	refCount    int32
	shouldClose int32

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
	cc := codec.GoRpc.ClientCodec(stream, msgpackHandle)
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
	if c.clients.Len() < c.pool.maxStreams && atomic.LoadInt32(&c.shouldClose) == 0 {
		c.clients.PushFront(client)
		didSave = true
	}
	c.clientLock.Unlock()
	if !didSave {
		client.Close()
	}
}

// ConnPool is used to maintain a connection pool to other
// Consul servers. This is used to reduce the latency of
// RPC requests between servers. It is only used to pool
// connections in the rpcConsul mode. Raft connections
// are pooled separately.
type ConnPool struct {
	sync.Mutex

	// LogOutput is used to control logging
	logOutput io.Writer

	// The maximum time to keep a connection open
	maxTime time.Duration

	// The maximum number of open streams to keep
	maxStreams int

	// Pool maps an address to a open connection
	pool map[string]*Conn

	// TLS wrapper
	tlsWrap tlsutil.DCWrapper

	// Used to indicate the pool is shutdown
	shutdown   bool
	shutdownCh chan struct{}
}

// NewPool is used to make a new connection pool
// Maintain at most one connection per host, for up to maxTime.
// Set maxTime to 0 to disable reaping. maxStreams is used to control
// the number of idle streams allowed.
// If TLS settings are provided outgoing connections use TLS.
func NewPool(logOutput io.Writer, maxTime time.Duration, maxStreams int, tlsWrap tlsutil.DCWrapper) *ConnPool {
	pool := &ConnPool{
		logOutput:  logOutput,
		maxTime:    maxTime,
		maxStreams: maxStreams,
		pool:       make(map[string]*Conn),
		tlsWrap:    tlsWrap,
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
func (p *ConnPool) acquire(dc string, addr net.Addr, version int) (*Conn, error) {
	// Check for a pooled ocnn
	if conn := p.getPooled(addr, version); conn != nil {
		return conn, nil
	}

	// Create a new connection
	return p.getNewConn(dc, addr, version)
}

// getPooled is used to return a pooled connection
func (p *ConnPool) getPooled(addr net.Addr, version int) *Conn {
	p.Lock()
	c := p.pool[addr.String()]
	if c != nil {
		c.lastUsed = time.Now()
		atomic.AddInt32(&c.refCount, 1)
	}
	p.Unlock()
	return c
}

// getNewConn is used to return a new connection
func (p *ConnPool) getNewConn(dc string, addr net.Addr, version int) (*Conn, error) {
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
	if p.tlsWrap != nil {
		// Switch the connection into TLS mode
		if _, err := conn.Write([]byte{byte(rpcTLS)}); err != nil {
			conn.Close()
			return nil, err
		}

		// Wrap the connection in a TLS client
		tlsConn, err := p.tlsWrap(dc, conn)
		if err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
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

		// Setup the logger
		conf := yamux.DefaultConfig()
		conf.LogOutput = p.logOutput

		// Create a multiplexed session
		session, _ = yamux.Client(conn, conf)
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
	if existing := p.pool[addr.String()]; existing != nil {
		c.Close()
		p.Unlock()
		return existing, nil
	} else {
		p.pool[addr.String()] = c
		p.Unlock()
		return c, nil
	}
}

// clearConn is used to clear any cached connection, potentially in response to an erro
func (p *ConnPool) clearConn(conn *Conn) {
	// Ensure returned streams are closed
	atomic.StoreInt32(&conn.shouldClose, 1)

	// Clear from the cache
	p.Lock()
	if c, ok := p.pool[conn.addr.String()]; ok && c == conn {
		delete(p.pool, conn.addr.String())
	}
	p.Unlock()

	// Close down immediately if idle
	if refCount := atomic.LoadInt32(&conn.refCount); refCount == 0 {
		conn.Close()
	}
}

// releaseConn is invoked when we are done with a conn to reduce the ref count
func (p *ConnPool) releaseConn(conn *Conn) {
	refCount := atomic.AddInt32(&conn.refCount, -1)
	if refCount == 0 && atomic.LoadInt32(&conn.shouldClose) == 1 {
		conn.Close()
	}
}

// getClient is used to get a usable client for an address and protocol version
func (p *ConnPool) getClient(dc string, addr net.Addr, version int) (*Conn, *StreamClient, error) {
	retries := 0
START:
	// Try to get a conn first
	conn, err := p.acquire(dc, addr, version)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get conn: %v", err)
	}

	// Get a client
	client, err := conn.getClient()
	if err != nil {
		p.clearConn(conn)
		p.releaseConn(conn)

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
func (p *ConnPool) RPC(dc string, addr net.Addr, version int, method string, args interface{}, reply interface{}) error {
	// Get a usable client
	conn, sc, err := p.getClient(dc, addr, version)
	if err != nil {
		return fmt.Errorf("rpc error: %v", err)
	}

	// Make the RPC call
	err = sc.client.Call(method, args, reply)
	if err != nil {
		sc.Close()
		p.releaseConn(conn)
		return fmt.Errorf("rpc error: %v", err)
	}

	// Done with the connection
	conn.returnClient(sc)
	p.releaseConn(conn)
	return nil
}

// Reap is used to close conns open over maxTime
func (p *ConnPool) reap() {
	for {
		// Sleep for a while
		select {
		case <-p.shutdownCh:
			return
		case <-time.After(time.Second):
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
