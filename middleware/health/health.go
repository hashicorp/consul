package health

import (
	"io"
	"log"
	"net"
	"net/http"
	"sync"
)

var once sync.Once

type Health struct {
	Addr string
	ln   net.Listener
	mux  *http.ServeMux
}

func health(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, ok)
}

func (h *Health) Start() error {
	if h.Addr == "" {
		h.Addr = defAddr
	}

	once.Do(func() {
		if ln, err := net.Listen("tcp", h.Addr); err != nil {
			log.Printf("[ERROR] Failed to start health handler: %s", err)
			return
		} else {
			h.ln = ln
		}
		h.mux = http.NewServeMux()

		h.mux.HandleFunc(path, health)
		go func() {
			http.Serve(h.ln, h.mux)
		}()
	})
	return nil
}

func (h *Health) Shutdown() error {
	if h.ln != nil {
		return h.ln.Close()
	}
	return nil
}

const (
	ok      = "OK"
	defAddr = ":8080"
	path    = "/health"
)
