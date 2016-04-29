package pprof

import (
	"log"
	"net"
	"net/http"
	pp "net/http/pprof"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type Handler struct {
	Next middleware.Handler
	ln   net.Listener
	mux  *http.ServeMux
}

// ServeDNS passes all other requests up the chain.
func (h *Handler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return h.Next.ServeDNS(ctx, w, r)
}

func (h *Handler) Start() error {
	if ln, err := net.Listen("tcp", addr); err != nil {
		log.Printf("[ERROR] Failed to start pprof handler: %s", err)
		return err
	} else {
		h.ln = ln
	}

	h.mux = http.NewServeMux()
	h.mux.HandleFunc(path+"/", pp.Index)
	h.mux.HandleFunc(path+"/cmdline", pp.Cmdline)
	h.mux.HandleFunc(path+"/profile", pp.Profile)
	h.mux.HandleFunc(path+"/symbol", pp.Symbol)
	h.mux.HandleFunc(path+"/trace", pp.Trace)

	go func() {
		http.Serve(h.ln, h.mux)
	}()
	return nil
}

func (h *Handler) Shutdown() error {
	if h.ln != nil {
		return h.ln.Close()
	}
	return nil
}

const (
	addr = "localhost:6053"
	path = "/debug/pprof"
)
