package health

import (
	"io"
	"log"
	"net/http"
	"sync"
)

var once sync.Once

type Health struct {
	Addr string
}

func health(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, ok)
}

func (h Health) ListenAndServe() error {
	if h.Addr == "" {
		h.Addr = defAddr
	}
	once.Do(func() {
		http.HandleFunc("/health", health)
		go func() {
			if err := http.ListenAndServe(h.Addr, nil); err != nil {
				log.Printf("[ERROR] Failed to start health handler: %s", err)
			}
		}()
	})
	return nil
}

const (
	ok      = "OK"
	defAddr = ":8080"
)
