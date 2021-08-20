package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/tlsutil"
)

// ClientConnPool creates and stores a connection for each datacenter.
type ClientConnPool struct {
	dialer        dialer
	servers       ServerLocator
	gwResolverDep gatewayResolverDep
	conns         map[string]*grpc.ClientConn
	connsLock     sync.Mutex
}

type ServerLocator interface {
	// ServerForGlobalAddr returns server metadata for a server with the specified globally unique address.
	ServerForGlobalAddr(globalAddr string) (*metadata.Server, error)

	// Authority returns the target authority to use to dial the server. This is primarily
	// needed for testing multiple agents in parallel, because gRPC requires the
	// resolver to be registered globally.
	Authority() string
}

// gatewayResolverDep is just a holder for a function pointer that can be
// updated lazily after the structs are instantiated (but before first use)
// and all structs with a reference to this struct will see the same update.
type gatewayResolverDep struct {
	// GatewayResolver is a function that returns a suitable random mesh
	// gateway address for dialing servers in a given DC. This is only
	// needed if wan federation via mesh gateways is enabled.
	GatewayResolver func(string) string
}

// TLSWrapper wraps a non-TLS connection and returns a connection with TLS
// enabled.
type TLSWrapper func(dc string, conn net.Conn) (net.Conn, error)

// ALPNWrapper is a function that is used to wrap a non-TLS connection and
// returns an appropriate TLS connection or error. This taks a datacenter and
// node name as argument to configure the desired SNI value and the desired
// next proto for configuring ALPN.
type ALPNWrapper func(dc, nodeName, alpnProto string, conn net.Conn) (net.Conn, error)

type dialer func(context.Context, string) (net.Conn, error)

// NewClientConnPool create new GRPC client pool to connect to servers using GRPC over RPC
func NewClientConnPool(
	servers ServerLocator,
	srcAddr *net.TCPAddr, // SrcAddr is the source address for outgoing connections.
	tlsWrapper TLSWrapper,
	alpnWrapper ALPNWrapper,
	useTLSForDC func(dc string) bool,
	dialingFromServer bool,
	dialingFromDatacenter string,
) *ClientConnPool {
	c := &ClientConnPool{
		servers: servers,
		conns:   make(map[string]*grpc.ClientConn),
	}

	c.dialer = newDialer(
		servers,
		srcAddr,
		&c.gwResolverDep,
		tlsWrapper,
		alpnWrapper,
		useTLSForDC,
		dialingFromServer,
		dialingFromDatacenter,
	)
	return c
}

// SetGatewayResolver is only to be called during setup before the pool is used.
func (c *ClientConnPool) SetGatewayResolver(gatewayResolver func(string) string) {
	c.gwResolverDep.GatewayResolver = gatewayResolver
}

// ClientConn returns a grpc.ClientConn for the datacenter. If there are no
// existing connections in the pool, a new one will be created, stored in the pool,
// then returned.
func (c *ClientConnPool) ClientConn(datacenter string) (*grpc.ClientConn, error) {
	return c.dial(datacenter, "server")
}

// TODO: godoc
func (c *ClientConnPool) ClientConnLeader() (*grpc.ClientConn, error) {
	return c.dial("local", "leader")
}

func (c *ClientConnPool) dial(datacenter string, serverType string) (*grpc.ClientConn, error) {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()

	target := fmt.Sprintf("consul://%s/%s.%s", c.servers.Authority(), serverType, datacenter)
	if conn, ok := c.conns[target]; ok {
		return conn, nil
	}

	conn, err := grpc.Dial(
		target,
		// use WithInsecure mode here because we handle the TLS wrapping in the
		// custom dialer based on logic around whether the server has TLS enabled.
		grpc.WithInsecure(),
		grpc.WithContextDialer(c.dialer),
		grpc.WithDisableRetry(),
		grpc.WithStatsHandler(newStatsHandler(defaultMetrics())),
		// nolint:staticcheck // there is no other supported alternative to WithBalancerName
		grpc.WithBalancerName("pick_first"),
		// Keep alive parameters are based on the same default ones we used for
		// Yamux. These are somewhat arbitrary but we did observe in scale testing
		// that the gRPC defaults (servers send keepalives only every 2 hours,
		// clients never) seemed to result in TCP drops going undetected until
		// actual updates needed to be sent which caused unnecessary delays for
		// deliveries. These settings should be no more work for servers than
		// existing yamux clients but hopefully allow TCP drops to be detected
		// earlier and so have a smaller chance of going unnoticed until there are
		// actual updates to send out from the servers. The servers have a policy to
		// not accept pings any faster than once every 15 seconds to protect against
		// abuse.
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}))
	if err != nil {
		return nil, err
	}

	c.conns[target] = conn
	return conn, nil
}

// newDialer returns a gRPC dialer function that conditionally wraps the connection
// with TLS based on the Server.useTLS value.
func newDialer(
	servers ServerLocator,
	srcAddr *net.TCPAddr, // SrcAddr is the source address for outgoing connections.
	gwResolverDep *gatewayResolverDep,
	wrapper TLSWrapper,
	alpnWrapper ALPNWrapper,
	useTLSForDC func(dc string) bool,
	// dialingFromServer should be set to true if this connection pool is
	// configured in a server instead of a client.
	dialingFromServer bool,
	dialingFromDatacenter string,
) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, globalAddr string) (net.Conn, error) {
		server, err := servers.ServerForGlobalAddr(globalAddr)
		if err != nil {
			return nil, err
		}

		if dialingFromServer &&
			gwResolverDep.GatewayResolver != nil &&
			alpnWrapper != nil &&
			server.Datacenter != dialingFromDatacenter {
			// NOTE: TLS is required on this branch.
			conn, _, err := pool.DialRPCViaMeshGateway(
				ctx,
				server.Datacenter,
				server.ShortName,
				srcAddr,
				tlsutil.ALPNWrapper(alpnWrapper),
				pool.ALPN_RPCGRPC,
				dialingFromServer,
				gwResolverDep.GatewayResolver,
			)
			return conn, err
		}

		d := net.Dialer{LocalAddr: srcAddr, Timeout: pool.DefaultDialTimeout}
		conn, err := d.DialContext(ctx, "tcp", server.Addr.String())
		if err != nil {
			return nil, err
		}

		if server.UseTLS && useTLSForDC(server.Datacenter) {
			if wrapper == nil {
				conn.Close()
				return nil, fmt.Errorf("TLS enabled but got nil TLS wrapper")
			}

			// Switch the connection into TLS mode
			if _, err := conn.Write([]byte{byte(pool.RPCTLS)}); err != nil {
				conn.Close()
				return nil, err
			}

			// Wrap the connection in a TLS client
			tlsConn, err := wrapper(server.Datacenter, conn)
			if err != nil {
				conn.Close()
				return nil, err
			}
			conn = tlsConn
		}

		// TODO: wanfed more like pool.ConnPool.DialTimeout
		_, err = conn.Write([]byte{byte(pool.RPCGRPC)})
		if err != nil {
			conn.Close()
			return nil, err
		}

		return conn, nil
	}
}
