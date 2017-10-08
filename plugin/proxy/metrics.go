package proxy

import (
	"sync"

	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics the proxy plugin exports.
var (
	RequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "proxy",
		Name:      "request_count_total",
		Help:      "Counter of requests made per protocol, proxy protocol, family and upstream.",
	}, []string{"proto", "proxy_proto", "family", "to"})
	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: "proxy",
		Name:      "request_duration_milliseconds",
		Buckets:   append(prometheus.DefBuckets, []float64{15, 20, 25, 30, 40, 50, 100, 200, 500, 1000, 2000, 3000, 4000, 5000, 10000}...),
		Help:      "Histogram of the time (in milliseconds) each request took.",
	}, []string{"proto", "proxy_proto", "family", "to"})
)

// OnStartupMetrics sets up the metrics on startup. This is done for all proxy protocols.
func OnStartupMetrics() error {
	metricsOnce.Do(func() {
		prometheus.MustRegister(RequestCount)
		prometheus.MustRegister(RequestDuration)
	})
	return nil
}

// familyToString returns the string form of either 1, or 2. Returns
// empty string is not a known family
func familyToString(f int) string {
	if f == 1 {
		return "1"
	}
	if f == 2 {
		return "2"
	}
	return ""
}

var metricsOnce sync.Once
