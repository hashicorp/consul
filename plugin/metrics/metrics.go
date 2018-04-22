// Package metrics implement a handler and plugin that provides Prometheus metrics.
package metrics

import (
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics/vars"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds the prometheus configuration. The metrics' path is fixed to be /metrics
type Metrics struct {
	Next    plugin.Handler
	Addr    string
	Reg     *prometheus.Registry
	ln      net.Listener
	lnSetup bool
	mux     *http.ServeMux

	zoneNames []string
	zoneMap   map[string]bool
	zoneMu    sync.RWMutex
}

// New returns a new instance of Metrics with the given address
func New(addr string) *Metrics {
	met := &Metrics{
		Addr:    addr,
		Reg:     prometheus.NewRegistry(),
		zoneMap: make(map[string]bool),
	}
	// Add the default collectors
	met.MustRegister(prometheus.NewGoCollector())
	met.MustRegister(prometheus.NewProcessCollector(os.Getpid(), ""))

	// Add all of our collectors
	met.MustRegister(buildInfo)
	met.MustRegister(vars.RequestCount)
	met.MustRegister(vars.RequestDuration)
	met.MustRegister(vars.RequestSize)
	met.MustRegister(vars.RequestDo)
	met.MustRegister(vars.RequestType)
	met.MustRegister(vars.ResponseSize)
	met.MustRegister(vars.ResponseRcode)

	return met
}

// MustRegister wraps m.Reg.MustRegister.
func (m *Metrics) MustRegister(c prometheus.Collector) { m.Reg.MustRegister(c) }

// AddZone adds zone z to m.
func (m *Metrics) AddZone(z string) {
	m.zoneMu.Lock()
	m.zoneMap[z] = true
	m.zoneNames = keys(m.zoneMap)
	m.zoneMu.Unlock()
}

// RemoveZone remove zone z from m.
func (m *Metrics) RemoveZone(z string) {
	m.zoneMu.Lock()
	delete(m.zoneMap, z)
	m.zoneNames = keys(m.zoneMap)
	m.zoneMu.Unlock()
}

// ZoneNames returns the zones of m.
func (m *Metrics) ZoneNames() []string {
	m.zoneMu.RLock()
	s := m.zoneNames
	m.zoneMu.RUnlock()
	return s
}

// OnStartup sets up the metrics on startup.
func (m *Metrics) OnStartup() error {
	ln, err := net.Listen("tcp", m.Addr)
	if err != nil {
		log.Errorf("Failed to start metrics handler: %s", err)
		return err
	}

	m.ln = ln
	m.lnSetup = true
	ListenAddr = m.ln.Addr().String() // For tests

	m.mux = http.NewServeMux()
	m.mux.Handle("/metrics", promhttp.HandlerFor(m.Reg, promhttp.HandlerOpts{}))

	go func() {
		http.Serve(m.ln, m.mux)
	}()
	return nil
}

// OnRestart stops the listener on reload.
func (m *Metrics) OnRestart() error {
	if !m.lnSetup {
		return nil
	}

	uniqAddr.setAddressTodo(m.Addr)

	m.ln.Close()
	m.lnSetup = false
	return nil
}

// OnFinalShutdown tears down the metrics listener on shutdown and restart.
func (m *Metrics) OnFinalShutdown() error {
	// We allow prometheus statements in multiple Server Blocks, but only the first
	// will open the listener, for the rest they are all nil; guard against that.
	if !m.lnSetup {
		return nil
	}

	m.lnSetup = false
	return m.ln.Close()
}

func keys(m map[string]bool) []string {
	sx := []string{}
	for k := range m {
		sx = append(sx, k)
	}
	return sx
}

// ListenAddr is assigned the address of the prometheus listener. Its use is mainly in tests where
// we listen on "localhost:0" and need to retrieve the actual address.
var ListenAddr string

var (
	buildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: plugin.Namespace,
		Name:      "build_info",
		Help:      "A metric with a constant '1' value labeled by version, revision, and goversion from which CoreDNS was built.",
	}, []string{"version", "revision", "goversion"})
)
