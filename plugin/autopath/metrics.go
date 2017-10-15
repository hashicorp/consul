package autopath

import (
	"sync"

	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics for autopath.
var (
	AutoPathCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "autopath",
		Name:      "success_count_total",
		Help:      "Counter of requests that did autopath.",
	}, []string{})
)

// OnStartupMetrics sets up the metrics on startup.
func OnStartupMetrics() error {
	metricsOnce.Do(func() {
		prometheus.MustRegister(AutoPathCount)
	})
	return nil
}

var metricsOnce sync.Once
