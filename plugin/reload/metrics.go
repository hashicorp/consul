package reload

import (
	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics for the reload plugin
var (
	FailedCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "reload",
		Name:      "failed_count_total",
		Help:      "Counter of the number of failed reload attempts.",
	})
)
