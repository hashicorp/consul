// Package pprof implements a debug endpoint for getting profiles using the
// go pprof tooling.
package pprof

import (
	"net"
	"net/http"
	pp "net/http/pprof"
	"runtime"

	"github.com/coredns/coredns/plugin/pkg/reuseport"
)

type handler struct {
	addr     string
	rateBloc int
	ln       net.Listener
	mux      *http.ServeMux
}

func (h *handler) Startup() error {
	// Reloading the plugin without changing the listening address results
	// in an error unless we reuse the port because Startup is called for
	// new handlers before Shutdown is called for the old ones.
	ln, err := reuseport.Listen("tcp", h.addr)
	if err != nil {
		log.Errorf("Failed to start pprof handler: %s", err)
		return err
	}

	h.ln = ln

	h.mux = http.NewServeMux()
	h.mux.HandleFunc(path, func(rw http.ResponseWriter, req *http.Request) {
		http.Redirect(rw, req, path+"/", http.StatusFound)
	})
	h.mux.HandleFunc(path+"/", pp.Index)
	h.mux.HandleFunc(path+"/cmdline", pp.Cmdline)
	h.mux.HandleFunc(path+"/profile", pp.Profile)
	h.mux.HandleFunc(path+"/symbol", pp.Symbol)
	h.mux.HandleFunc(path+"/trace", pp.Trace)

	runtime.SetBlockProfileRate(h.rateBloc)

	go func() {
		http.Serve(h.ln, h.mux)
	}()
	return nil
}

func (h *handler) Shutdown() error {
	if h.ln != nil {
		return h.ln.Close()
	}
	return nil
}

const (
	path = "/debug/pprof"
)
