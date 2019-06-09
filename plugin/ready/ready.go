// Package ready is used to signal readiness of the CoreDNS process. Once all
// plugins have called in the plugin will signal readiness by returning a 200
// OK on the HTTP handler (on port 8181). If not ready yet, the handler will
// return a 503.
package ready

import (
	"io"
	"net"
	"net/http"
	"sync"

	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/uniq"
)

var (
	log      = clog.NewWithPlugin("ready")
	plugins  = &list{}
	uniqAddr = uniq.New()
)

type ready struct {
	Addr string

	sync.RWMutex
	ln   net.Listener
	done bool
	mux  *http.ServeMux
}

func (rd *ready) onStartup() error {
	ln, err := net.Listen("tcp", rd.Addr)
	if err != nil {
		return err
	}

	rd.Lock()
	rd.ln = ln
	rd.mux = http.NewServeMux()
	rd.done = true
	rd.Unlock()

	rd.mux.HandleFunc("/ready", func(w http.ResponseWriter, _ *http.Request) {
		ok, todo := plugins.Ready()
		if ok {
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "OK")
			return
		}
		log.Infof("Still waiting on: %q", todo)
		w.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(w, todo)
	})

	go func() { http.Serve(rd.ln, rd.mux) }()

	return nil
}

func (rd *ready) onFinalShutdown() error {
	rd.Lock()
	defer rd.Unlock()
	if !rd.done {
		return nil
	}

	uniqAddr.Unset(rd.Addr)

	rd.ln.Close()
	rd.done = false
	return nil
}
