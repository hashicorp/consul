package proxy

import (
	"sync"

	"github.com/miekg/coredns/middleware"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics the proxy middleware exports.
var (
	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_duration_milliseconds",
		Buckets:   append(prometheus.DefBuckets, []float64{50, 100, 200, 500, 1000, 2000, 3000, 4000, 5000, 10000}...),
		Help:      "Histogram of the time (in milliseconds) each request took.",
	}, []string{"zone"})
)

// OnStartup sets up the metrics on startup.
func OnStartup() error {
	metricsOnce.Do(func() {
		prometheus.MustRegister(RequestDuration)
	})
	return nil
}

var metricsOnce sync.Once

const subsystem = "proxy"
