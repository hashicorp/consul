package grpc

import (
	"context"
	"sync/atomic"

	"github.com/armon/go-metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
)

// statsHandler is a grpc/stats.StatsHandler which emits connection and
// request metrics to go-metrics.
type statsHandler struct {
	activeConns uint64 // must be 8-byte aligned for atomic access
}

// TagRPC implements grpcStats.StatsHandler
func (c *statsHandler) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context {
	// No-op
	return ctx
}

// HandleRPC implements grpcStats.StatsHandler
func (c *statsHandler) HandleRPC(_ context.Context, s stats.RPCStats) {
	label := "server"
	if s.IsClient() {
		label = "client"
	}
	switch s.(type) {
	case *stats.InHeader:
		metrics.IncrCounter([]string{"grpc", label, "request"}, 1)
	}
}

// TagConn implements grpcStats.StatsHandler
func (c *statsHandler) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	// No-op
	return ctx
}

// HandleConn implements grpcStats.StatsHandler
func (c *statsHandler) HandleConn(_ context.Context, s stats.ConnStats) {
	label := "server"
	if s.IsClient() {
		label = "client"
	}
	var count uint64
	switch s.(type) {
	case *stats.ConnBegin:
		count = atomic.AddUint64(&c.activeConns, 1)
	case *stats.ConnEnd:
		// Decrement!
		count = atomic.AddUint64(&c.activeConns, ^uint64(0))
	}
	metrics.SetGauge([]string{"grpc", label, "active_conns"}, float32(count))
}

type activeStreamCounter struct {
	// count of the number of open streaming RPCs on a server. It is accessed
	// atomically.
	count uint64
}

// GRPCCountingStreamInterceptor is a grpc.ServerStreamInterceptor that emits a
// a metric of the count of open streams.
func (i *activeStreamCounter) Intercept(
	srv interface{},
	ss grpc.ServerStream,
	_ *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {

	count := atomic.AddUint64(&i.count, 1)
	metrics.SetGauge([]string{"grpc", "server", "active_streams"}, float32(count))
	defer func() {
		count := atomic.AddUint64(&i.count, ^uint64(0))
		metrics.SetGauge([]string{"grpc", "server", "active_streams"}, float32(count))
	}()

	return handler(srv, ss)
}
