package consul

import (
	"fmt"
	"log"
	"net"
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
	serverProvider  ServerProvider
	tlsConfigurator *tlsutil.Configurator
}

func NewGRPCClient(logger *log.Logger, serverProvider ServerProvider, tlsConfigurator *tlsutil.Configurator) *GRPCClient {
	return &GRPCClient{
		serverProvider:  serverProvider,
		tlsConfigurator: tlsConfigurator,
	}
}

func (c *GRPCClient) GRPCConn() (*grpc.ClientConn, error) {
	dialer := newDialer(c.serverProvider, c.tlsConfigurator.OutgoingRPCWrapper())
	conn, err := grpc.Dial("consul:///server.local",
		grpc.WithInsecure(),
		grpc.WithDialer(dialer),
		grpc.WithDisableRetry(),
		grpc.WithBalancerName("pick_first"))
	if err != nil {
		return nil, err
	}

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
