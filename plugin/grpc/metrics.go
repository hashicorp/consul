package grpc

import (
	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Variables declared for monitoring.
var (
	RequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "grpc",
		Name:      "request_count_total",
		Help:      "Counter of requests made per upstream.",
	}, []string{"to"})
	RcodeCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "grpc",
		Name:      "response_rcode_count_total",
		Help:      "Counter of requests made per upstream.",
	}, []string{"rcode", "to"})
	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: "grpc",
		Name:      "request_duration_seconds",
		Buckets:   plugin.TimeBuckets,
		Help:      "Histogram of the time each request took.",
	}, []string{"to"})
)
