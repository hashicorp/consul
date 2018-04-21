// Package health implements an HTTP handler that responds to health checks.
package health

import (
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin/pkg/log"
)

// Health implements healthchecks by polling plugins.
type health struct {
	Addr     string
	lameduck time.Duration

	ln      net.Listener
	nlSetup bool
	mux     *http.ServeMux

	// A slice of Healthers that the health plugin will poll every second for their health status.
	h []Healther
	sync.RWMutex
	ok bool // ok is the global boolean indicating an all healthy plugin stack

	stop     chan bool
	pollstop chan bool
}

// newHealth returns a new initialized health.
func newHealth(addr string) *health {
	return &health{Addr: addr, stop: make(chan bool), pollstop: make(chan bool)}
}

func (h *health) OnStartup() error {
	if h.Addr == "" {
		h.Addr = defAddr
	}

	ln, err := net.Listen("tcp", h.Addr)
	if err != nil {
		return err
	}

	h.ln = ln
	h.mux = http.NewServeMux()
	h.nlSetup = true

	h.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if h.Ok() {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, ok)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	go func() { http.Serve(h.ln, h.mux) }()
	go func() { h.overloaded() }()

	return nil
}

func (h *health) OnRestart() error { return h.OnFinalShutdown() }

func (h *health) OnFinalShutdown() error {
	if !h.nlSetup {
		return nil
	}

	// Stop polling plugins
	h.pollstop <- true
	// NACK health
	h.SetOk(false)

	if h.lameduck > 0 {
		log.Infof("Going into lameduck mode for %s", h.lameduck)
		time.Sleep(h.lameduck)
	}

	h.ln.Close()

	h.stop <- true
	h.nlSetup = false
	return nil
}

const (
	ok      = "OK"
	defAddr = ":8080"
	path    = "/health"
)
