// Package health implements an HTTP handler that responds to health checks.
package health

import (
	"io"
	"net"
	"net/http"
	"time"

	clog "github.com/coredns/coredns/plugin/pkg/log"
)

var log = clog.NewWithPlugin("health")

// Health implements healthchecks by polling plugins.
type health struct {
	Addr     string
	lameduck time.Duration

	ln      net.Listener
	nlSetup bool
	mux     *http.ServeMux

	stop chan bool
}

// newHealth returns a new initialized health.
func newHealth(addr string) *health {
	return &health{Addr: addr, stop: make(chan bool)}
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
		// We're always healthy.
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, ok)
		return
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

	if h.lameduck > 0 {
		log.Infof("Going into lameduck mode for %s", h.lameduck)
		time.Sleep(h.lameduck)
	}

	h.ln.Close()

	h.nlSetup = false
	close(h.stop)
	return nil
}

const (
	ok      = "OK"
	defAddr = ":8080"
	path    = "/health"
)
