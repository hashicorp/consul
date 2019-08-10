package consul

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/tlsutil"
	"google.golang.org/grpc"
)

const (
	grpcBasePath = "/consul"
)

type GRPCClient struct {
	grpcConns     map[string]*grpc.ClientConn
	grpcConnsLock sync.RWMutex
	routers       *router.Manager
	tlsWrap       tlsutil.DCWrapper
}

func NewGRPCClient(logger *log.Logger, routers *router.Manager, tlsWrap tlsutil.DCWrapper) *GRPCClient {
	return &GRPCClient{
		grpcConns: make(map[string]*grpc.ClientConn),
		routers:   routers,
		tlsWrap:   tlsWrap,
	}
}

func (c *GRPCClient) GRPCConn(server *metadata.Server) (*grpc.ClientConn, error) {
	if server == nil {
		server = c.routers.FindServer()
		if server == nil {
			return nil, structs.ErrNoServers
		}
	}

	host, _, _ := net.SplitHostPort(server.Addr.String())
	addr := fmt.Sprintf("%s:%d", host, server.Port)

	c.grpcConnsLock.RLock()
	conn, ok := c.grpcConns[addr]
	c.grpcConnsLock.RUnlock()
	if ok {
		return conn, nil
	}

	c.grpcConnsLock.Lock()
	defer c.grpcConnsLock.Unlock()

	dialer := newDialer(server.UseTLS, server.Datacenter, c.tlsWrap)
	co, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithDialer(dialer), grpc.WithDisableRetry())
	if err != nil {
		return nil, err
	}

	c.grpcConns[addr] = conn
	return co, nil
}

// newDialer returns a gRPC dialer function that conditionally wraps the connection
// with TLS depending on the given useTLS value.
func newDialer(useTLS bool, datacenter string, wrapper tlsutil.DCWrapper) func(string, time.Duration) (net.Conn, error) {
	return func(addr string, _ time.Duration) (net.Conn, error) {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}

		// Check if TLS is enabled
		if useTLS && wrapper != nil {
			// Switch the connection into TLS mode
			if _, err := conn.Write([]byte{byte(pool.RPCTLS)}); err != nil {
				conn.Close()
				return nil, err
			}

			// Wrap the connection in a TLS client
			tlsConn, err := wrapper(datacenter, conn)
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
