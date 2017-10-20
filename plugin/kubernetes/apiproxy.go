package kubernetes

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/coredns/coredns/plugin/pkg/healthcheck"
)

type proxyHandler struct {
	healthcheck.HealthCheck
}

type apiProxy struct {
	http.Server
	listener net.Listener
	handler  proxyHandler
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upstream := p.Select()
	network := "tcp"
	address := upstream.Name

	d, err := net.Dial(network, address)
	if err != nil {
		log.Printf("[ERROR] Unable to establish connection to upstream %s://%s: %s", network, address, err)
		http.Error(w, fmt.Sprintf("Unable to establish connection to upstream %s://%s: %s", network, address, err), 500)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Printf("[ERROR] Unable to establish connection: no hijacker")
		http.Error(w, "Unable to establish connection: no hijacker", 500)
		return
	}
	nc, _, err := hj.Hijack()
	if err != nil {
		log.Printf("[ERROR] Unable to hijack connection: %s", err)
		http.Error(w, fmt.Sprintf("Unable to hijack connection: %s", err), 500)
		return
	}
	defer nc.Close()
	defer d.Close()

	err = r.Write(d)
	if err != nil {
		log.Printf("[ERROR] Unable to copy connection to upstream %s://%s: %s", network, address, err)
		http.Error(w, fmt.Sprintf("Unable to copy connection to upstream %s://%s: %s", network, address, err), 500)
		return
	}

	errChan := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errChan <- err
	}
	go cp(d, nc)
	go cp(nc, d)
	<-errChan
}

func (p *apiProxy) Run() {
	p.handler.Start()
	go func() {
		p.Serve(p.listener)
	}()
}

func (p *apiProxy) Stop() {
	p.handler.Stop()
	p.listener.Close()
}
