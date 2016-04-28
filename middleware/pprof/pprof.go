package pprof

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/miekg/coredns/middleware"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

const addr = "localhost:8053"

type Handler struct {
	Next middleware.Handler
}

// ServeDNS passes all other requests up the chain.
func (h *Handler) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return h.Next.ServeDNS(ctx, w, r)
}

func (h *Handler) Start() error {
	go func() {
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Printf("[ERROR] Failed to start pprof handler: %s", err)
		}
	}()
	return nil
}
