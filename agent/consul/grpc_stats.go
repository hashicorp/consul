package consul

import (
	"context"
	"sync/atomic"

	metrics "github.com/armon/go-metrics"
	"google.golang.org/grpc"
	grpcStats "google.golang.org/grpc/stats"
)

var (
	// grpcStatsHandler is the global stats handler instance. Yes I know global is
	// horrible but go-metrics started it. Now we need to be global to make
	// connection count gauge useful.
	grpcStatsHandler *GRPCStatsHandler

	// grpcActiveStreams is used to keep track of the number of open streaming
	// RPCs on a server. It is accessed atomically, See notes above on global
	// sadness.
	grpcActiveStreams uint64
)

func init() {
	grpcStatsHandler = &GRPCStatsHandler{}
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

// GRPCCountingStreamInterceptor is a grpc.ServerStreamInterceptor that just
// tracks open streaming calls to the server and emits metrics on how many are
// open.
func GRPCCountingStreamInterceptor(srv interface{}, ss grpc.ServerStream,
	info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	// Count the stream
	new := atomic.AddUint64(&grpcActiveStreams, 1)
	metrics.SetGauge([]string{"grpc", "server", "active_streams"}, float32(new))
	defer func() {
		new := atomic.AddUint64(&grpcActiveStreams, ^uint64(0))
		metrics.SetGauge([]string{"grpc", "server", "active_streams"}, float32(new))
	}()

	return handler(srv, ss)
}
