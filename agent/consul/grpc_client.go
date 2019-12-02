package consul

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"google.golang.org/grpc"
	grpcStats "google.golang.org/grpc/stats"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/pool"

	"github.com/hashicorp/consul/tlsutil"
)

const (
	grpcBasePath = "/consul"
)

// grpcStatsHandler is the global stats handler instance. Yes I know global is
// horrible but go-metrics started it. Now we need to be global to make
// connection count gauge useful.
var grpcStatsHandler *GRPCStatsHandler

func init() {
	grpcStatsHandler = &GRPCStatsHandler{}
}

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
		// use WithInsecure mode here because we handle the TLS wrapping in the
		// custom dialer based on logic around whether the server has TLS enabled.
		grpc.WithInsecure(),
		grpc.WithContextDialer(dialer),
		grpc.WithDisableRetry(),
		grpc.WithStatsHandler(grpcStatsHandler),
		grpc.WithBalancerName("pick_first"))
	if err != nil {
		return nil, err
	}

	c.grpcConns[datacenter] = conn

	return conn, nil
}

// GRPCStatsHandler is a grpc/stats.StatsHandler which emits stats to
// go-metrics.
type GRPCStatsHandler struct {
	activeConns uint64 // must be 8-byte aligned for atomic access
}

// TagRPC implements grpcStats.StatsHandler
func (c *GRPCStatsHandler) TagRPC(ctx context.Context, i *grpcStats.RPCTagInfo) context.Context {
	// No-op
	return ctx
}

// HandleRPC implements grpcStats.StatsHandler
func (c *GRPCStatsHandler) HandleRPC(ctx context.Context, s grpcStats.RPCStats) {
	label := "server"
	if s.IsClient() {
		label = "client"
	}
	switch s.(type) {
	case *grpcStats.InHeader:
		metrics.IncrCounter([]string{"grpc", label, "request"}, 1)
	}
}

// TagConn implements grpcStats.StatsHandler
func (c *GRPCStatsHandler) TagConn(ctx context.Context, i *grpcStats.ConnTagInfo) context.Context {
	// No-op
	return ctx
}

// HandleConn implements grpcStats.StatsHandler
func (c *GRPCStatsHandler) HandleConn(ctx context.Context, s grpcStats.ConnStats) {
	label := "server"
	if s.IsClient() {
		label = "client"
	}
	var new uint64
	switch s.(type) {
	case *grpcStats.ConnBegin:
		new = atomic.AddUint64(&c.activeConns, 1)
	case *grpcStats.ConnEnd:
		// Decrement!
		new = atomic.AddUint64(&c.activeConns, ^uint64(0))
	}
	metrics.SetGauge([]string{"grpc", label, "active_conns"}, float32(new))
}

// newDialer returns a gRPC dialer function that conditionally wraps the connection
// with TLS depending on the given useTLS value.
func newDialer(serverProvider ServerProvider, wrapper tlsutil.DCWrapper) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		d := net.Dialer{}
		conn, err := d.DialContext(ctx, "tcp", addr)
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
