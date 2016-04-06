package metrics

import (
	"log"
	"net/http"
	"sync"

	"github.com/miekg/coredns/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "coredns"

var (
	requestCount    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
	responseRcode   *prometheus.CounterVec
)

const path = "/metrics"

// Metrics holds the prometheus configuration. The metrics' path is fixed to be /metrics
type Metrics struct {
	Next      middleware.Handler
	Addr      string // where to we listen
	Once      sync.Once
	ZoneNames []string
}

func (m *Metrics) Start() error {
	m.Once.Do(func() {
		define("")

		prometheus.MustRegister(requestCount)
		prometheus.MustRegister(requestDuration)
		prometheus.MustRegister(responseSize)
		prometheus.MustRegister(responseRcode)

		http.Handle(path, prometheus.Handler())
		go func() {
			if err := http.ListenAndServe(m.Addr, nil); err != nil {
				log.Printf("[ERROR] Failed to start prometheus handler: %s", err)
			}
		}()
	})
	return nil
}

func define(subsystem string) {
	if subsystem == "" {
		subsystem = "dns"
	}
	requestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "request_count_total",
		Help:      "Counter of DNS requests made per zone and type and opcode.",
	}, []string{"zone", "qtype"})

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "request_duration_seconds",
		Help:      "Histogram of the time (in seconds) each request took.",
	}, []string{"zone", "qtype"})

	responseSize = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "response_size_bytes",
		Help:      "Size of the returns response in bytes.",
		Buckets:   []float64{0, 100, 200, 300, 400, 511, 1023, 2047, 4095, 8291, 16e3, 32e3, 48e3, 64e3},
	}, []string{"zone", "qtype"})

	responseRcode = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "response_rcode_count_total",
		Help:      "Counter of response status codes.",
	}, []string{"zone", "rcode", "qtype"})
}
