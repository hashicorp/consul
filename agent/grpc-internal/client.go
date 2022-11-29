package internal

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/armon/go-metrics"

	agentmiddleware "github.com/hashicorp/consul/agent/grpc-middleware"
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

type ClientConnPoolConfig struct {
	// Servers is a reference for how to figure out how to dial any server.
	Servers ServerLocator

	// SrcAddr is the source address for outgoing connections.
	SrcAddr *net.TCPAddr

	// TLSWrapper is the specifics of wrapping a socket when doing an TYPE_BYTE+TLS
	// wrapped RPC request.
	TLSWrapper TLSWrapper

	// ALPNWrapper is the specifics of wrapping a socket when doing an ALPN+TLS
	// wrapped RPC request (typically only for wan federation via mesh
	// gateways).
	ALPNWrapper ALPNWrapper

	// UseTLSForDC is a function to determine if dialing a given datacenter
	// should use TLS.
	UseTLSForDC func(dc string) bool

	// DialingFromServer should be set to true if this connection pool is owned
	// by a consul server instance.
	DialingFromServer bool

	// DialingFromDatacenter is the datacenter of the consul agent using this
	// pool.
	DialingFromDatacenter string
}

// NewClientConnPool create new GRPC client pool to connect to servers using
// GRPC over RPC.
func NewClientConnPool(cfg ClientConnPoolConfig) *ClientConnPool {
	c := &ClientConnPool{
		servers: cfg.Servers,
		conns:   make(map[string]*grpc.ClientConn),
	}
	c.dialer = newDialer(cfg, &c.gwResolverDep)
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
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(c.dialer),
		grpc.WithDisableRetry(),
		grpc.WithStatsHandler(agentmiddleware.NewStatsHandler(metrics.Default(), metricsLabels)),
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
		}),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(8*1024*1024), grpc.MaxCallRecvMsgSize(8*1024*1024)),
	)
	if err != nil {
		return nil, err
	}

	c.conns[target] = conn
	return conn, nil
}

// newDialer returns a gRPC dialer function that conditionally wraps the connection
// with TLS based on the Server.useTLS value.
func newDialer(cfg ClientConnPoolConfig, gwResolverDep *gatewayResolverDep) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, globalAddr string) (net.Conn, error) {
		server, err := cfg.Servers.ServerForGlobalAddr(globalAddr)
		if err != nil {
			return nil, err
		}

		if cfg.DialingFromServer &&
			gwResolverDep.GatewayResolver != nil &&
			cfg.ALPNWrapper != nil &&
			server.Datacenter != cfg.DialingFromDatacenter {
			// NOTE: TLS is required on this branch.
			conn, _, err := pool.DialRPCViaMeshGateway(
				ctx,
				server.Datacenter,
				server.ShortName,
				cfg.SrcAddr,
				tlsutil.ALPNWrapper(cfg.ALPNWrapper),
				pool.ALPN_RPCGRPC,
				cfg.DialingFromServer,
				gwResolverDep.GatewayResolver,
			)
			return conn, err
		}

		d := net.Dialer{LocalAddr: cfg.SrcAddr, Timeout: pool.DefaultDialTimeout}
		conn, err := d.DialContext(ctx, "tcp", server.Addr.String())
		if err != nil {
			return nil, err
		}

		if server.UseTLS && cfg.UseTLSForDC(server.Datacenter) {
			if cfg.TLSWrapper == nil {
				conn.Close()
				return nil, fmt.Errorf("TLS enabled but got nil TLS wrapper")
			}

			// Switch the connection into TLS mode
			if _, err := conn.Write([]byte{byte(pool.RPCTLS)}); err != nil {
				conn.Close()
				return nil, err
			}

			// Wrap the connection in a TLS client
			tlsConn, err := cfg.TLSWrapper(server.Datacenter, conn)
			if err != nil {
				conn.Close()
				return nil, err
			}
			conn = tlsConn
		}

		_, err = conn.Write([]byte{byte(pool.RPCGRPC)})
		if err != nil {
			conn.Close()
			return nil, err
		}

		return conn, nil
	}
}
