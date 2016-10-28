// Package metrics implement a handler and middleware that provides Prometheus metrics.
package metrics

import (
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/metrics/vars"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds the prometheus configuration. The metrics' path is fixed to be /metrics
type Metrics struct {
	Next middleware.Handler
	Addr string
	ln   net.Listener
	mux  *http.ServeMux
	Once sync.Once

	zoneNames []string
	zoneMap   map[string]bool
	zoneMu    sync.RWMutex
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
	m.Once.Do(func() {

		ln, err := net.Listen("tcp", m.Addr)
		if err != nil {
			log.Printf("[ERROR] Failed to start metrics handler: %s", err)
			return
		}

		m.ln = ln
		ListenAddr = m.ln.Addr().String()

		m.mux = http.NewServeMux()

		prometheus.MustRegister(vars.RequestCount)
		prometheus.MustRegister(vars.RequestDuration)
		prometheus.MustRegister(vars.RequestSize)
		prometheus.MustRegister(vars.RequestDo)
		prometheus.MustRegister(vars.RequestType)

		prometheus.MustRegister(vars.ResponseSize)
		prometheus.MustRegister(vars.ResponseRcode)

		m.mux.Handle("/metrics", prometheus.Handler())

		go func() {
			http.Serve(m.ln, m.mux)
		}()
	})
	return nil
}

// OnShutdown tears down the metrics on shutdown.
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
