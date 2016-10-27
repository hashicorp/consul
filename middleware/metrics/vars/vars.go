package vars

import (
	"github.com/miekg/coredns/middleware"

	"github.com/prometheus/client_golang/prometheus"
)

// Request* and Reponse* are the prometheus counters and gauges we are using for exporting metrics.
var (
	RequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_count_total",
		Help:      "Counter of DNS requests made per zone, protocol and family.",
	}, []string{"zone", "proto", "family"})

	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_duration_milliseconds",
		Buckets:   append(prometheus.DefBuckets, []float64{50, 100, 200, 500, 1000, 2000, 3000, 4000, 5000, 10000}...),
		Help:      "Histogram of the time (in milliseconds) each request took.",
	}, []string{"zone"})

	RequestSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_size_bytes",
		Help:      "Size of the EDNS0 UDP buffer in bytes (64K for TCP).",
		Buckets:   []float64{0, 100, 200, 300, 400, 511, 1023, 2047, 4095, 8291, 16e3, 32e3, 48e3, 64e3},
	}, []string{"zone", "proto"})

	RequestDo = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_do_count_total",
		Help:      "Counter of DNS requests with DO bit set per zone.",
	}, []string{"zone"})

	RequestType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_type_count_total",
		Help:      "Counter of DNS requests per type, per zone.",
	}, []string{"zone", "type"})

	ResponseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "response_size_bytes",
		Help:      "Size of the returned response in bytes.",
		Buckets:   []float64{0, 100, 200, 300, 400, 511, 1023, 2047, 4095, 8291, 16e3, 32e3, 48e3, 64e3},
	}, []string{"zone", "proto"})

	ResponseRcode = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "response_rcode_count_total",
		Help:      "Counter of response status codes.",
	}, []string{"zone", "rcode"})
)

const (
	subsystem = "dns"

	// Dropped indicates we dropped the query before any handling. It has no closing dot, so it can not be a valid zone.
	Dropped = "dropped"
)
