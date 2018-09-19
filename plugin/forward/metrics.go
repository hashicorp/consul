package forward

import (
	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Variables declared for monitoring.
var (
	RequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "forward",
		Name:      "request_count_total",
		Help:      "Counter of requests made per upstream.",
	}, []string{"to"})
	RcodeCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "forward",
		Name:      "response_rcode_count_total",
		Help:      "Counter of requests made per upstream.",
	}, []string{"rcode", "to"})
	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: "forward",
		Name:      "request_duration_seconds",
		Buckets:   plugin.TimeBuckets,
		Help:      "Histogram of the time each request took.",
	}, []string{"to"})
	HealthcheckFailureCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "forward",
		Name:      "healthcheck_failure_count_total",
		Help:      "Counter of the number of failed healtchecks.",
	}, []string{"to"})
	HealthcheckBrokenCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: "forward",
		Name:      "healthcheck_broken_count_total",
		Help:      "Counter of the number of complete failures of the healtchecks.",
	})
	SocketGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Subsystem: "forward",
		Name:      "sockets_open",
		Help:      "Gauge of open sockets per upstream.",
	}, []string{"to"})
)
