package metrics

import (
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/miekg/coredns/middleware"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	requestCount    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.HistogramVec
	requestDo       *prometheus.CounterVec
	requestType     *prometheus.CounterVec

	responseSize  *prometheus.HistogramVec
	responseRcode *prometheus.CounterVec
)

// Metrics holds the prometheus configuration. The metrics' path is fixed to be /metrics
type Metrics struct {
	Next      middleware.Handler
	Addr      string
	ln        net.Listener
	mux       *http.ServeMux
	Once      sync.Once
	ZoneNames []string
}

func (m *Metrics) Start() error {
	m.Once.Do(func() {
		define()

		if ln, err := net.Listen("tcp", m.Addr); err != nil {
			log.Printf("[ERROR] Failed to start metrics handler: %s", err)
			return
		} else {
			m.ln = ln
		}
		m.mux = http.NewServeMux()

		prometheus.MustRegister(requestCount)
		prometheus.MustRegister(requestDuration)
		prometheus.MustRegister(requestSize)
		prometheus.MustRegister(requestDo)
		prometheus.MustRegister(requestType)

		prometheus.MustRegister(responseSize)
		prometheus.MustRegister(responseRcode)

		m.mux.Handle(path, prometheus.Handler())

		go func() {
			http.Serve(m.ln, m.mux)
		}()
	})
	return nil
}

func (m *Metrics) Shutdown() error {
	if m.ln != nil {
		return m.ln.Close()
	}
	return nil
}

func define() {
	requestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_count_total",
		Help:      "Counter of DNS requests made per zone, protocol and family.",
	}, []string{"zone", "proto", "family"})

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_duration_seconds",
		Buckets:   append([]float64{.0001, .0005, .001, .0025}, prometheus.DefBuckets...),
		Help:      "Histogram of the time (in seconds) each request took.",
	}, []string{"zone"})

	requestSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_size_bytes",
		Help:      "Size of the EDNS0 UDP buffer in bytes (64K for TCP).",
		Buckets:   []float64{0, 100, 200, 300, 400, 511, 1023, 2047, 4095, 8291, 16e3, 32e3, 48e3, 64e3},
	}, []string{"zone", "proto"})

	requestDo = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_do_count_total",
		Help:      "Counter of DNS requests with DO bit set per zone.",
	}, []string{"zone"})

	requestType = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "request_type_count_total",
		Help:      "Counter of DNS requests per type, per zone.",
	}, []string{"zone", "type"})

	responseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "response_size_bytes",
		Help:      "Size of the returns response in bytes.",
		Buckets:   []float64{0, 100, 200, 300, 400, 511, 1023, 2047, 4095, 8291, 16e3, 32e3, 48e3, 64e3},
	}, []string{"zone", "proto"})

	responseRcode = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: middleware.Namespace,
		Subsystem: subsystem,
		Name:      "response_rcode_count_total",
		Help:      "Counter of response status codes.",
	}, []string{"zone", "rcode"})
}

const (
	// Dropped indicates we dropped the query before any handling. It has no closing dot, so it can not be a valid zone.
	Dropped   = "dropped"
	subsystem = "dns"
	path      = "/metrics"
)
