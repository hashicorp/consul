package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"

	"google.golang.org/grpc"

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
	// Scheme returns the url scheme to use to dial the server. This is primarily
	// needed for testing multiple agents in parallel, because gRPC requires the
	// resolver to be registered globally.
	Scheme() string
}

// TLSWrapper wraps a non-TLS connection and returns a connection with TLS
// possibly enabled.
// The bool indicates whether the returned function will use TLS.
type TLSWrapper func(dc string, conn net.Conn) (func(net.Conn) (net.Conn, error), bool)

type dialer func(context.Context, string) (net.Conn, error)

func NewClientConnPool(servers ServerLocator, tls TLSWrapper) *ClientConnPool {
	return &ClientConnPool{
		dialer:  newDialer(servers, tls),
		servers: servers,
		conns:   make(map[string]*grpc.ClientConn),
	}
}

// ClientConn returns a grpc.ClientConn for the datacenter. If there are no
// existing connections in the pool, a new one will be created, stored in the pool,
// then returned.
func (c *ClientConnPool) ClientConn(datacenter string) (*grpc.ClientConn, error) {
	c.connsLock.Lock()
	defer c.connsLock.Unlock()

	if conn, ok := c.conns[datacenter]; ok {
		return conn, nil
	}

	conn, err := grpc.Dial(
		fmt.Sprintf("%s:///server.%s", c.servers.Scheme(), datacenter),
		// use WithInsecure mode here because we handle the TLS wrapping in the
		// custom dialer based on logic around whether the server has TLS enabled.
		grpc.WithInsecure(),
		grpc.WithContextDialer(c.dialer),
		grpc.WithDisableRetry(),
		grpc.WithStatsHandler(newStatsHandler(defaultMetrics())),
		// nolint:staticcheck // there is no other supported alternative to WithBalancerName
		grpc.WithBalancerName("pick_first"))
	if err != nil {
		return nil, err
	}

	c.conns[datacenter] = conn
	return conn, nil
}

// newDialer returns a gRPC dialer function that conditionally wraps the connection
// with TLS based on the Server.useTLS value.
func newDialer(servers ServerLocator, tlsWrapper TLSWrapper) func(context.Context, string) (net.Conn, error) {
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
		wrapper, tlsEnabled := tlsWrapper(server.Datacenter, conn)
		if server.UseTLS && tlsEnabled {
			// If connection is upgraded to TLS, mark the stream as RPCTLS
			if _, err := conn.Write([]byte{byte(pool.RPCTLS)}); err != nil {
				conn.Close()
				return nil, err
			}
			// Wrap the connection in a TLS client, return same conn if TLS disabled
			tlsConn, err := wrapper(conn)
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
