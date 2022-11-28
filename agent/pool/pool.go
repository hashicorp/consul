package pool

import (
	"container/list"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/yamux"

	msgpackrpc "github.com/hashicorp/consul-net-rpc/net-rpc-msgpackrpc"
	"github.com/hashicorp/consul-net-rpc/net/rpc"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/tlsutil"
)

const DefaultDialTimeout = 10 * time.Second

// muxSession is used to provide an interface for a stream multiplexer.
type muxSession interface {
	Open() (net.Conn, error)
	Close() error
}

// streamClient is used to wrap a stream with an RPC client
type StreamClient struct {
	stream net.Conn
	codec  rpc.ClientCodec
}

func (sc *StreamClient) Close() {
	sc.stream.Close()
	sc.codec.Close()
}

// Conn is a pooled connection to a Consul server
type Conn struct {
	refCount    int32
	shouldClose int32

	nodeName string
	addr     net.Addr
	session  muxSession
	lastUsed time.Time

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
	codec := msgpackrpc.NewCodecFromHandle(true, true, stream, structs.MsgpackHandle)

	// Return a new stream client
	sc := &StreamClient{
		stream: stream,
		codec:  codec,
	}
	return sc, nil
}

// returnStream is used when done with a stream
// to allow re-use by a future RPC
func (c *Conn) returnClient(client *StreamClient) {
	didSave := false
	c.clientLock.Lock()
	if c.clients.Len() < c.pool.MaxStreams && atomic.LoadInt32(&c.shouldClose) == 0 {
		c.clients.PushFront(client)
		didSave = true

		// If this is a Yamux stream, shrink the internal buffers so that
		// we can GC the idle memory
		if ys, ok := client.stream.(*yamux.Stream); ok {
			ys.Shrink()
		}
	}
	c.clientLock.Unlock()
	if !didSave {
		client.Close()
	}
}

// markForUse does all the bookkeeping required to ready a connection for use.
func (c *Conn) markForUse() {
	c.lastUsed = time.Now()
	atomic.AddInt32(&c.refCount, 1)
}

// ConnPool is used to maintain a connection pool to other Consul
// servers. This is used to reduce the latency of RPC requests between
// servers. It is only used to pool connections in the rpcConsul mode.
// Raft connections are pooled separately. Maintain at most one
// connection per host, for up to MaxTime. When MaxTime connection
// reaping is disabled. MaxStreams is used to control the number of idle
// streams allowed. If TLS settings are provided outgoing connections
// use TLS.
type ConnPool struct {
	// clientTimeoutMs is the default timeout for client RPC requests
	// in milliseconds. Stored as an atomic uint32 value to allow for
	// reloading.
	// TODO: once we move to go1.19, change to atomic.Uint32.
	clientTimeoutMs uint32

	// SrcAddr is the source address for outgoing connections.
	SrcAddr *net.TCPAddr

	// Logger passed to yamux
	// TODO: consider refactoring to accept a full yamux.Config instead of a logger
	Logger *log.Logger

	// RPCHoldTimeout is used as a buffer when calculating timeouts to
	// allow for leader rotation.
	RPCHoldTimeout time.Duration
	// MaxQueryTime is used for calculating timeouts on blocking queries.
	MaxQueryTime time.Duration
	// DefaultQueryTime is used for calculating timeouts on blocking queries.
	DefaultQueryTime time.Duration

	// The maximum time to keep a connection open
	MaxTime time.Duration

	// The maximum number of open streams to keep
	MaxStreams int

	// TLSConfigurator
	TLSConfigurator *tlsutil.Configurator

	// GatewayResolver is a function that returns a suitable random mesh
	// gateway address for dialing servers in a given DC. This is only
	// needed if wan federation via mesh gateways is enabled.
	GatewayResolver func(string) string

	// Datacenter is the datacenter of the current agent.
	Datacenter string

	// Server should be set to true if this connection pool is configured in a
	// server instead of a client.
	Server bool

	sync.Mutex

	// pool maps a nodeName+address to a open connection
	pool map[string]*Conn

	// limiter is used to throttle the number of connect attempts
	// to a given address. The first thread will attempt a connection
	// and put a channel in here, which all other threads will wait
	// on to close.
	limiter map[string]chan struct{}

	// Used to indicate the pool is shutdown
	shutdown   bool
	shutdownCh chan struct{}

	// once initializes the internal data structures and connection
	// reaping on first use.
	once sync.Once
}

// init configures the initial data structures. It should be called
// by p.once.Do(p.init) in all public methods.
func (p *ConnPool) init() {
	p.pool = make(map[string]*Conn)
	p.limiter = make(map[string]chan struct{})
	p.shutdownCh = make(chan struct{})
	if p.MaxTime > 0 {
		go p.reap()
	}
}

// Shutdown is used to close the connection pool
func (p *ConnPool) Shutdown() error {
	p.once.Do(p.init)

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

// acquire will return a pooled connection, if available. Otherwise it will
// wait for an existing connection attempt to finish, if one if in progress,
// and will return that one if it succeeds. If all else fails, it will return a
// newly-created connection and add it to the pool.
func (p *ConnPool) acquire(dc string, nodeName string, addr net.Addr) (*Conn, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("pool: ConnPool.acquire requires a node name")
	}

	addrStr := addr.String()

	poolKey := nodeName + ":" + addrStr

	// Check to see if there's a pooled connection available. This is up
	// here since it should the vastly more common case than the rest
	// of the code here.
	p.Lock()
	c := p.pool[poolKey]
	if c != nil {
		c.markForUse()
		p.Unlock()
		return c, nil
	}

	// If not (while we are still locked), set up the throttling structure
	// for this address, which will make everyone else wait until our
	// attempt is done.
	var wait chan struct{}
	var ok bool
	if wait, ok = p.limiter[addrStr]; !ok {
		wait = make(chan struct{})
		p.limiter[addrStr] = wait
	}
	isLeadThread := !ok
	p.Unlock()

	// If we are the lead thread, make the new connection and then wake
	// everybody else up to see if we got it.
	if isLeadThread {
		c, err := p.getNewConn(dc, nodeName, addr)
		p.Lock()
		delete(p.limiter, addrStr)
		close(wait)
		if err != nil {
			p.Unlock()
			return nil, err
		}

		p.pool[poolKey] = c
		p.Unlock()
		return c, nil
	}

	// Otherwise, wait for the lead thread to attempt the connection
	// and use what's in the pool at that point.
	select {
	case <-p.shutdownCh:
		return nil, fmt.Errorf("rpc error: shutdown")
	case <-wait:
	}

	// See if the lead thread was able to get us a connection.
	p.Lock()
	if c := p.pool[poolKey]; c != nil {
		c.markForUse()
		p.Unlock()
		return c, nil
	}

	p.Unlock()
	return nil, fmt.Errorf("rpc error: lead thread didn't get connection")
}

// HalfCloser is an interface that exposes a TCP half-close without exposing
// the underlying TLS or raw TCP connection.
type HalfCloser interface {
	CloseWrite() error
}

// DialTimeout is used to establish a raw connection to the given server, with
// given connection timeout. It also writes RPCTLS as the first byte.
func (p *ConnPool) DialTimeout(
	dc string,
	nodeName string,
	addr net.Addr,
	actualRPCType RPCType,
) (net.Conn, HalfCloser, error) {
	p.once.Do(p.init)

	if p.Server &&
		p.GatewayResolver != nil &&
		p.TLSConfigurator != nil &&
		dc != p.Datacenter {
		// NOTE: TLS is required on this branch.
		nextProto := actualRPCType.ALPNString()
		if nextProto == "" {
			return nil, nil, fmt.Errorf("rpc type %d cannot be routed through a mesh gateway", actualRPCType)
		}
		return DialRPCViaMeshGateway(
			context.Background(),
			dc,
			nodeName,
			p.SrcAddr,
			p.TLSConfigurator.OutgoingALPNRPCWrapper(),
			nextProto,
			p.Server,
			p.GatewayResolver,
		)
	}

	return p.dial(dc, addr, actualRPCType, RPCTLS)
}

func (p *ConnPool) dial(
	dc string,
	addr net.Addr,
	actualRPCType RPCType,
	tlsRPCType RPCType,
) (net.Conn, HalfCloser, error) {
	// Try to dial the conn
	d := &net.Dialer{LocalAddr: p.SrcAddr, Timeout: DefaultDialTimeout}
	conn, err := d.Dial("tcp", addr.String())
	if err != nil {
		return nil, nil, err
	}

	var hc HalfCloser

	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetNoDelay(true)

		// Expose TCPConn CloseWrite method on HalfCloser
		hc = tcp
	}

	// Check if TLS is enabled
	if p.TLSConfigurator.UseTLS(dc) {
		wrapper := p.TLSConfigurator.OutgoingRPCWrapper()
		// Switch the connection into TLS mode
		if _, err := conn.Write([]byte{byte(tlsRPCType)}); err != nil {
			conn.Close()
			return nil, nil, err
		}

		// Wrap the connection in a TLS client
		tlsConn, err := wrapper(dc, conn)
		if err != nil {
			conn.Close()
			return nil, nil, err
		}
		conn = tlsConn

		// If this is a tls.Conn, expose HalfCloser to caller
		if tlsConn, ok := conn.(*tls.Conn); ok {
			hc = tlsConn
		}
	}

	// Send the type-byte for the protocol if one is required.
	//
	// When using insecure TLS there is no inner type-byte as these connections
	// aren't wrapped like the standard TLS ones are.
	if tlsRPCType != RPCTLSInsecure {
		if _, err := conn.Write([]byte{byte(actualRPCType)}); err != nil {
			conn.Close()
			return nil, nil, err
		}
	}

	return conn, hc, nil
}

func (p *ConnPool) RPCClientTimeout() time.Duration {
	return time.Duration(atomic.LoadUint32(&p.clientTimeoutMs)) * time.Millisecond
}

func (p *ConnPool) SetRPCClientTimeout(timeout time.Duration) {
	if timeout > time.Hour {
		// Prevent unreasonably large timeouts that might overflow a uint32
		timeout = time.Hour
	}
	atomic.StoreUint32(&p.clientTimeoutMs, uint32(timeout.Milliseconds()))
}

// DialRPCViaMeshGateway dials the destination node and sets up the connection
// to be the correct RPC type using ALPN. This currently is exclusively used to
// dial other servers in foreign datacenters via mesh gateways.
func DialRPCViaMeshGateway(
	ctx context.Context,
	dc string, // (metadata.Server).Datacenter
	nodeName string, // (metadata.Server).ShortName
	srcAddr *net.TCPAddr,
	alpnWrapper tlsutil.ALPNWrapper,
	nextProto string,
	dialingFromServer bool,
	gatewayResolver func(string) string,
) (net.Conn, HalfCloser, error) {
	if !dialingFromServer {
		return nil, nil, fmt.Errorf("must dial via mesh gateways from a server agent")
	} else if gatewayResolver == nil {
		return nil, nil, fmt.Errorf("gatewayResolver is nil")
	} else if alpnWrapper == nil {
		return nil, nil, fmt.Errorf("cannot dial via a mesh gateway when outgoing TLS is disabled")
	}

	gwAddr := gatewayResolver(dc)
	if gwAddr == "" {
		return nil, nil, structs.ErrDCNotAvailable
	}

	dialer := &net.Dialer{LocalAddr: srcAddr, Timeout: DefaultDialTimeout}

	rawConn, err := dialer.DialContext(ctx, "tcp", gwAddr)
	if err != nil {
		return nil, nil, err
	}

	if nextProto != ALPN_RPCGRPC {
		// agent/grpc-internal/client.go:dial() handles this in another way for gRPC
		if tcp, ok := rawConn.(*net.TCPConn); ok {
			_ = tcp.SetKeepAlive(true)
			_ = tcp.SetNoDelay(true)
		}
	}

	// NOTE: now we wrap the connection in a TLS client.
	tlsConn, err := alpnWrapper(dc, nodeName, nextProto, rawConn)
	if err != nil {
		return nil, nil, err
	}

	var conn net.Conn = tlsConn

	var hc HalfCloser
	if tlsConn, ok := conn.(*tls.Conn); ok {
		// Expose *tls.Conn CloseWrite method on HalfCloser
		hc = tlsConn
	}

	return conn, hc, nil
}

// getNewConn is used to return a new connection
func (p *ConnPool) getNewConn(dc string, nodeName string, addr net.Addr) (*Conn, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("pool: ConnPool.getNewConn requires a node name")
	}

	// Get a new, raw connection and write the Consul multiplex byte to set the mode
	conn, _, err := p.DialTimeout(dc, nodeName, addr, RPCMultiplexV2)
	if err != nil {
		return nil, err
	}

	conf := yamux.DefaultConfig()
	// override the default because LogOutput conflicts with Logger.
	conf.LogOutput = nil
	conf.Logger = p.Logger

	// Create a multiplexed session
	session, err := yamux.Client(conn, conf)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("Failed to create yamux client: %w", err)
	}

	// Wrap the connection
	c := &Conn{
		refCount: 1,
		nodeName: nodeName,
		addr:     addr,
		session:  session,
		clients:  list.New(),
		lastUsed: time.Now(),
		pool:     p,
	}
	return c, nil
}

// clearConn is used to clear any cached connection, potentially in response to an error
func (p *ConnPool) clearConn(conn *Conn) {
	if conn.nodeName == "" {
		panic("pool: ConnPool.acquire requires a node name")
	}

	// Ensure returned streams are closed
	atomic.StoreInt32(&conn.shouldClose, 1)

	// Clear from the cache
	addrStr := conn.addr.String()
	poolKey := conn.nodeName + ":" + addrStr
	p.Lock()
	if c, ok := p.pool[poolKey]; ok && c == conn {
		delete(p.pool, poolKey)
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

// getClient is used to get a usable client for an address
func (p *ConnPool) getClient(dc string, nodeName string, addr net.Addr) (*Conn, *StreamClient, error) {
	retries := 0
START:
	// Try to get a conn first
	conn, err := p.acquire(dc, nodeName, addr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get conn: %w", err)
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
		return nil, nil, fmt.Errorf("failed to start stream: %w", err)
	}
	return conn, client, nil
}

// RPC is used to make an RPC call to a remote host
func (p *ConnPool) RPC(
	dc string,
	nodeName string,
	addr net.Addr,
	method string,
	args interface{},
	reply interface{},
) error {
	if nodeName == "" {
		return fmt.Errorf("pool: ConnPool.RPC requires a node name")
	}

	// TODO (autoconf) probably will want to have a way to invoke the
	// secure or insecure variant depending on whether its an ongoing
	// or first time config request. For now though this is fine until
	// those ongoing requests are implemented.
	if method == "AutoEncrypt.Sign" || method == "AutoConfig.InitialConfiguration" {
		return p.rpcInsecure(dc, addr, method, args, reply)
	} else {
		return p.rpc(dc, nodeName, addr, method, args, reply)
	}
}

// rpcInsecure is used to make an RPC call to a remote host.
// It doesn't actually use any of the pooling, it is here so that it is
// transparent for the consumer. The pool cannot be used because
// AutoEncrypt.Sign is a one-off call and it doesn't make sense to pool that
// connection if it is not being reused.
func (p *ConnPool) rpcInsecure(dc string, addr net.Addr, method string, args interface{}, reply interface{}) error {
	if dc != p.Datacenter {
		return fmt.Errorf("insecure dialing prohibited between datacenters")
	}

	var codec rpc.ClientCodec
	conn, _, err := p.dial(dc, addr, 0, RPCTLSInsecure)
	if err != nil {
		return fmt.Errorf("rpcinsecure: error establishing connection: %w", err)
	}
	codec = msgpackrpc.NewCodecFromHandle(true, true, conn, structs.MsgpackHandle)

	// Make the RPC call
	err = msgpackrpc.CallWithCodec(codec, method, args, reply)
	if err != nil {
		return fmt.Errorf("rpcinsecure: error making call: %w", err)
	}

	return nil
}

// BlockableQuery represents a read query which can be blocking or non-blocking.
// This interface is used to override the rpc_client_timeout for blocking queries.
type BlockableQuery interface {
	// BlockingTimeout returns duration > 0 if the query is blocking.
	// Otherwise returns 0 for non-blocking queries.
	BlockingTimeout(maxQueryTime, defaultQueryTime time.Duration) time.Duration
}

var _ BlockableQuery = (*structs.QueryOptions)(nil)
var _ BlockableQuery = (*pbcommon.QueryOptions)(nil)

func (p *ConnPool) rpc(dc string, nodeName string, addr net.Addr, method string, args interface{}, reply interface{}) error {
	p.once.Do(p.init)

	// Get a usable client
	conn, sc, err := p.getClient(dc, nodeName, addr)
	if err != nil {
		return fmt.Errorf("rpc error getting client: %w", err)
	}

	var deadline time.Time
	timeout := p.RPCClientTimeout()
	if bq, ok := args.(BlockableQuery); ok {
		blockingTimeout := bq.BlockingTimeout(p.MaxQueryTime, p.DefaultQueryTime)
		if blockingTimeout > 0 {
			// Override the default client timeout but add RPCHoldTimeout
			// as a buffer for retries during leadership changes.
			timeout = blockingTimeout + p.RPCHoldTimeout
		}
	}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	if err := sc.stream.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("rpc error setting read deadline: %w", err)
	}

	// Make the RPC call
	err = msgpackrpc.CallWithCodec(sc.codec, method, args, reply)
	if err != nil {
		sc.Close()

		// See the comment in leader_test.go TestLeader_ChangeServerID
		// about how we found this. The tldr is that if we see this
		// error, we know this connection is toast, so we should clear
		// it and make a new one on the next attempt.
		if lib.IsErrEOF(err) {
			p.clearConn(conn)
		}

		p.releaseConn(conn)
		return fmt.Errorf("rpc error making call: %w", err)
	}

	// Done with the connection
	conn.returnClient(sc)
	p.releaseConn(conn)
	return nil
}

// Ping sends a Status.Ping message to the specified server and
// returns true if healthy, false if an error occurred
func (p *ConnPool) Ping(dc string, nodeName string, addr net.Addr) (bool, error) {
	var out struct{}
	err := p.RPC(dc, nodeName, addr, "Status.Ping", struct{}{}, &out)
	return err == nil, err
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
			if now.Sub(conn.lastUsed) < p.MaxTime {
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
