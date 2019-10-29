package consul

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"

	"github.com/hashicorp/consul/tlsutil"
	"google.golang.org/grpc"
)

const (
	grpcBasePath = "/consul"
)

type ServerProvider interface {
	Servers() []*metadata.Server
}

type GRPCClient struct {
	scheme          string
	serverProvider  ServerProvider
	tlsConfigurator *tlsutil.Configurator
	grpcConns       map[string]*grpc.ClientConn
	grpcConnLock    sync.Mutex
}

func NewGRPCClient(logger *log.Logger, serverProvider ServerProvider, tlsConfigurator *tlsutil.Configurator, scheme string) *GRPCClient {
	return &GRPCClient{
		scheme:          scheme,
		serverProvider:  serverProvider,
		tlsConfigurator: tlsConfigurator,
		grpcConns:       make(map[string]*grpc.ClientConn),
	}
}

func (c *GRPCClient) GRPCConn(datacenter string) (*grpc.ClientConn, error) {
	c.grpcConnLock.Lock()
	defer c.grpcConnLock.Unlock()

	// If there's an existing ClientConn for the given DC, return it.
	if conn, ok := c.grpcConns[datacenter]; ok {
		return conn, nil
	}

	dialer := newDialer(c.serverProvider, c.tlsConfigurator.OutgoingRPCWrapper())
	conn, err := grpc.Dial(fmt.Sprintf("%s:///server.%s", c.scheme, datacenter),
		// use WithInsecure mode here because we handle the TLS wrapping in the custom dialer
		// based on logic around whether the server has TLS enabled.
		grpc.WithInsecure(),
		grpc.WithDialer(dialer),
		grpc.WithDisableRetry(),
		grpc.WithBalancerName("pick_first"))
	if err != nil {
		return nil, err
	}

	c.grpcConns[datacenter] = conn

	return conn, nil
}

// newDialer returns a gRPC dialer function that conditionally wraps the connection
// with TLS depending on the given useTLS value.
func newDialer(serverProvider ServerProvider, wrapper tlsutil.DCWrapper) func(string, time.Duration) (net.Conn, error) {
	return func(addr string, _ time.Duration) (net.Conn, error) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}

		// Check if TLS is enabled for the server.
		var found bool
		var server *metadata.Server
		for _, s := range serverProvider.Servers() {
			if s.Addr.String() == addr {
				found = true
				server = s
			}
		}
		if !found {
			return nil, fmt.Errorf("could not find Consul server for address %q", addr)
		}

		if server.UseTLS {
			if wrapper == nil {
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
			return nil, err
		}

		return conn, nil
	}
}
