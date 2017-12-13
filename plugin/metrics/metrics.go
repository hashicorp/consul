// Package metrics implement a handler and plugin that provides Prometheus metrics.
package metrics

import (
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics/vars"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	prometheus.MustRegister(vars.RequestCount)
	prometheus.MustRegister(vars.RequestDuration)
	prometheus.MustRegister(vars.RequestSize)
	prometheus.MustRegister(vars.RequestDo)
	prometheus.MustRegister(vars.RequestType)

	prometheus.MustRegister(vars.ResponseSize)
	prometheus.MustRegister(vars.ResponseRcode)
}

// Metrics holds the prometheus configuration. The metrics' path is fixed to be /metrics
type Metrics struct {
	Next plugin.Handler
	Addr string
	ln   net.Listener
	mux  *http.ServeMux

	zoneNames []string
	zoneMap   map[string]bool
	zoneMu    sync.RWMutex
}

// New returns a new instance of Metrics with the given address
func New(addr string) *Metrics {
	return &Metrics{Addr: addr, zoneMap: make(map[string]bool)}
}

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
		log.Printf("[ERROR] Failed to start metrics handler: %s", err)
		return err
	}

	m.ln = ln
	ListenAddr = m.ln.Addr().String()

	m.mux = http.NewServeMux()
	m.mux.Handle("/metrics", prometheus.Handler())

	go func() {
		http.Serve(m.ln, m.mux)
	}()
	return nil
}

// OnShutdown tears down the metrics on shutdown and restart.
func (m *Metrics) OnShutdown() error {
	if m.ln != nil {
		return m.ln.Close()
	}
	return nil
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
