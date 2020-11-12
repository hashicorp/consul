package grpc

import (
	"context"
	"sync/atomic"

	"github.com/armon/go-metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
)

var defaultMetrics = metrics.Default

// statsHandler is a grpc/stats.StatsHandler which emits connection and
// request metrics to go-metrics.
type statsHandler struct {
	metrics     *metrics.Metrics
	activeConns uint64 // must be 8-byte aligned for atomic access
}

func newStatsHandler(m *metrics.Metrics) *statsHandler {
	return &statsHandler{metrics: m}
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
		c.metrics.IncrCounter([]string{"grpc", label, "request", "count"}, 1)
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
		c.metrics.IncrCounter([]string{"grpc", label, "connection", "count"}, 1)
	case *stats.ConnEnd:
		// Decrement!
		count = atomic.AddUint64(&c.activeConns, ^uint64(0))
	}
	c.metrics.SetGauge([]string{"grpc", label, "connections"}, float32(count))
}

type activeStreamCounter struct {
	metrics *metrics.Metrics
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
	i.metrics.SetGauge([]string{"grpc", "server", "streams"}, float32(count))
	i.metrics.IncrCounter([]string{"grpc", "server", "stream", "count"}, 1)
	defer func() {
		count := atomic.AddUint64(&i.count, ^uint64(0))
		i.metrics.SetGauge([]string{"grpc", "server", "streams"}, float32(count))
	}()

	return handler(srv, ss)
}
