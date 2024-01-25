// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package middleware

import (
	"context"
	"sync/atomic"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
)

var StatsGauges = []prometheus.GaugeDefinition{
	{
		Name: []string{"grpc", "server", "connections"},
		Help: "Measures the number of active gRPC connections open on the server.",
	},
	{
		Name: []string{"grpc", "client", "connections"},
		Help: "Measures the number of active gRPC connections open from the client agent to any Consul servers.",
	},
	{
		Name: []string{"grpc", "server", "streams"},
		Help: "Measures the number of active gRPC streams handled by the server.",
	},
}
var StatsCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"grpc", "client", "request", "count"},
		Help: "Counts the number of gRPC requests made by the client agent to a Consul server.",
	},
	{
		Name: []string{"grpc", "server", "request", "count"},
		Help: "Counts the number of gRPC requests received by the server.",
	},
	{
		Name: []string{"grpc", "client", "connection", "count"},
		Help: "Counts the number of new gRPC connections opened by the client agent to a Consul server.",
	},
	{
		Name: []string{"grpc", "server", "connection", "count"},
		Help: "Counts the number of new gRPC connections received by the server.",
	},
	{
		Name: []string{"grpc", "server", "stream", "count"},
		Help: "Counts the number of new gRPC streams received by the server.",
	},
}

// statsHandler is a grpc/stats.StatsHandler which emits connection and
// request metrics to go-metrics.
type statsHandler struct {
	// activeConns is used with sync/atomic and MUST be 64-bit aligned. To ensure
	// alignment on 32-bit platforms this field must remain the first field in
	// the struct. See https://golang.org/pkg/sync/atomic/#pkg-note-BUG.
	activeConns uint64
	metrics     *metrics.Metrics
	labels      []metrics.Label
}

func NewStatsHandler(m *metrics.Metrics, labels []metrics.Label) *statsHandler {
	return &statsHandler{metrics: m, labels: labels}
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
		c.metrics.IncrCounterWithLabels([]string{"grpc", label, "request", "count"}, 1, c.labels)
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
		c.metrics.IncrCounterWithLabels([]string{"grpc", label, "connection", "count"}, 1, c.labels)
	case *stats.ConnEnd:
		// Decrement!
		count = atomic.AddUint64(&c.activeConns, ^uint64(0))
	}
	c.metrics.SetGaugeWithLabels([]string{"grpc", label, "connections"}, float32(count), c.labels)
}

type activeStreamCounter struct {
	// count is used with sync/atomic and MUST be 64-bit aligned. To ensure
	// alignment on 32-bit platforms this field must remain the first field in
	// the struct. See https://golang.org/pkg/sync/atomic/#pkg-note-BUG.
	count   uint64
	metrics *metrics.Metrics
	labels  []metrics.Label
}

func NewActiveStreamCounter(m *metrics.Metrics, labels []metrics.Label) *activeStreamCounter {
	return &activeStreamCounter{metrics: m, labels: labels}
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
	i.metrics.SetGaugeWithLabels([]string{"grpc", "server", "streams"}, float32(count), i.labels)
	i.metrics.IncrCounterWithLabels([]string{"grpc", "server", "stream", "count"}, 1, i.labels)
	defer func() {
		count := atomic.AddUint64(&i.count, ^uint64(0))
		i.metrics.SetGaugeWithLabels([]string{"grpc", "server", "streams"}, float32(count), i.labels)
	}()

	return handler(srv, ss)
}
