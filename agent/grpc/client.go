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
)

// ClientConnPool creates and stores a connection for each datacenter.
type ClientConnPool struct {
	dialer    dialer
	servers   ServerLocator
	conns     map[string]*grpc.ClientConn
	connsLock sync.Mutex
}

type ServerLocator interface {
	// ServerForAddr is used to look up server metadata from an address.
	ServerForAddr(addr string) (*metadata.Server, error)
	// Authority returns the target authority to use to dial the server. This is primarily
	// needed for testing multiple agents in parallel, because gRPC requires the
	// resolver to be registered globally.
	Authority() string
}

// TLSWrapper wraps a non-TLS connection and returns a connection with TLS
// enabled.
type TLSWrapper func(dc string, conn net.Conn) (net.Conn, error)

type dialer func(context.Context, string) (net.Conn, error)

// NewClientConnPool create new GRPC client pool to connect to servers using GRPC over RPC
func NewClientConnPool(servers ServerLocator, tls TLSWrapper, useTLSForDC func(dc string) bool) *ClientConnPool {
	return &ClientConnPool{
		dialer:  newDialer(servers, tls, useTLSForDC),
		servers: servers,
		conns:   make(map[string]*grpc.ClientConn),
	}
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
func newDialer(servers ServerLocator, wrapper TLSWrapper, useTLSForDC func(dc string) bool) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		d := net.Dialer{}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}

		server, err := servers.ServerForAddr(addr)
		if err != nil {
			conn.Close()
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

		_, err = conn.Write([]byte{pool.RPCGRPC})
		if err != nil {
			conn.Close()
			return nil, err
		}

		return conn, nil
	}
}
