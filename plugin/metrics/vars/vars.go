package vars

import (
	"github.com/coredns/coredns/plugin"

	"github.com/prometheus/client_golang/prometheus"
)

// Request* and Response* are the prometheus counters and gauges we are using for exporting metrics.
var (
	RequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: subsystem,
		Name:      "request_count_total",
		Help:      "Counter of DNS requests made per zone, protocol and family.",
	}, []string{"server", "zone", "proto", "family"})

	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: subsystem,
		Name:      "request_duration_seconds",
		Buckets:   plugin.TimeBuckets,
		Help:      "Histogram of the time (in seconds) each request took.",
	}, []string{"server", "zone"})

	RequestSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: subsystem,
		Name:      "request_size_bytes",
		Help:      "Size of the EDNS0 UDP buffer in bytes (64K for TCP).",
		Buckets:   []float64{0, 100, 200, 300, 400, 511, 1023, 2047, 4095, 8291, 16e3, 32e3, 48e3, 64e3},
	}, []string{"server", "zone", "proto"})

	RequestDo = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: subsystem,
		Name:      "request_do_count_total",
		Help:      "Counter of DNS requests with DO bit set per zone.",
	}, []string{"server", "zone"})

	RequestType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: subsystem,
		Name:      "request_type_count_total",
		Help:      "Counter of DNS requests per type, per zone.",
	}, []string{"server", "zone", "type"})

	ResponseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: plugin.Namespace,
		Subsystem: subsystem,
		Name:      "response_size_bytes",
		Help:      "Size of the returned response in bytes.",
		Buckets:   []float64{0, 100, 200, 300, 400, 511, 1023, 2047, 4095, 8291, 16e3, 32e3, 48e3, 64e3},
	}, []string{"server", "zone", "proto"})

	ResponseRcode = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Subsystem: subsystem,
		Name:      "response_rcode_count_total",
		Help:      "Counter of response status codes.",
	}, []string{"server", "zone", "rcode"})

	Panic = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: plugin.Namespace,
		Name:      "panic_count_total",
		Help:      "A metrics that counts the number of panics.",
	})

	PluginEnabled = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Name:      "plugin_enabled",
		Help:      "A metric that indicates whether a plugin is enabled on per server and zone basis.",
	}, []string{"server", "zone", "name"})
)

const (
	subsystem = "dns"

	// Dropped indicates we dropped the query before any handling. It has no closing dot, so it can not be a valid zone.
	Dropped = "dropped"
)
