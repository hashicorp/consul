package proxy

import (
	"sync"

	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics the proxy plugin exports.
var (
	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: "proxy",
		Name:      "request_duration_milliseconds",
		Buckets:   append(prometheus.DefBuckets, []float64{50, 100, 200, 500, 1000, 2000, 3000, 4000, 5000, 10000}...),
		Help:      "Histogram of the time (in milliseconds) each request took.",
	}, []string{"proto", "proxy_proto", "from"})
)

// OnStartupMetrics sets up the metrics on startup. This is done for all proxy protocols.
func OnStartupMetrics() error {
	metricsOnce.Do(func() {
		prometheus.MustRegister(RequestDuration)
	})
	return nil
}

var metricsOnce sync.Once
