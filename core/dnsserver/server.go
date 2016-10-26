// Package dnsserver implements all the interfaces from Caddy, so that CoreDNS can be a servertype plugin.
package dnsserver

import (
	"fmt"
	"log"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/miekg/coredns/middleware"
	"github.com/miekg/coredns/middleware/metrics/vars"
	"github.com/miekg/coredns/middleware/pkg/edns"
	"github.com/miekg/coredns/middleware/pkg/rcode"
	"github.com/miekg/coredns/request"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

// Server represents an instance of a server, which serves
// DNS requests at a particular address (host and port). A
// server is capable of serving numerous zones on
// the same address and the listener may be stopped for
// graceful termination (POSIX only).
type Server struct {
	Addr   string // Address we listen on
	mux    *dns.ServeMux
	server [2]*dns.Server // 0 is a net.Listener, 1 is a net.PacketConn (a *UDPConn) in our case.

	l net.Listener
	p net.PacketConn
	m sync.Mutex // protects listener and packetconn

	zones       map[string]*Config // zones keyed by their address
	dnsWg       sync.WaitGroup     // used to wait on outstanding connections
	connTimeout time.Duration      // the maximum duration of a graceful shutdown
}

// NewServer returns a new CoreDNS server and compiles all middleware in to it.
func NewServer(addr string, group []*Config) (*Server, error) {

	s := &Server{
		Addr:        addr,
		zones:       make(map[string]*Config),
		connTimeout: 5 * time.Second, // TODO(miek): was configurable
	}
	mux := dns.NewServeMux()
	mux.Handle(".", s) // wildcard handler, everything will go through here
	s.mux = mux

	// We have to bound our wg with one increment
	// to prevent a "race condition" that is hard-coded
	// into sync.WaitGroup.Wait() - basically, an add
	// with a positive delta must be guaranteed to
	// occur before Wait() is called on the wg.
	// In a way, this kind of acts as a safety barrier.
	s.dnsWg.Add(1)

	for _, site := range group {
		// set the config per zone
		s.zones[site.Zone] = site
		// compile custom middleware for everything
		var stack middleware.Handler
		for i := len(site.Middleware) - 1; i >= 0; i-- {
			stack = site.Middleware[i](stack)
		}
		site.middlewareChain = stack
	}

	return s, nil
}

// Serve starts the server with an existing listener. It blocks until the server stops.
func (s *Server) Serve(l net.Listener) error {
	s.m.Lock()
	s.server[tcp] = &dns.Server{Listener: l, Net: "tcp", Handler: s.mux}
	s.m.Unlock()

	return s.server[tcp].ActivateAndServe()
}

// ServePacket starts the server with an existing packetconn. It blocks until the server stops.
func (s *Server) ServePacket(p net.PacketConn) error {
	s.m.Lock()
	s.server[udp] = &dns.Server{PacketConn: p, Net: "udp", Handler: s.mux}
	s.m.Unlock()

	return s.server[udp].ActivateAndServe()
}

// Listen implements caddy.TCPServer interface.
func (s *Server) Listen() (net.Listener, error) {
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return nil, err
	}
	s.m.Lock()
	s.l = l
	s.m.Unlock()
	return l, nil
}

// ListenPacket implements caddy.UDPServer interface.
func (s *Server) ListenPacket() (net.PacketConn, error) {
	p, err := net.ListenPacket("udp", s.Addr)
	if err != nil {
		return nil, err
	}

	s.m.Lock()
	s.p = p
	s.m.Unlock()
	return p, nil
}

// Stop stops the server. It blocks until the server is
// totally stopped. On POSIX systems, it will wait for
// connections to close (up to a max timeout of a few
// seconds); on Windows it will close the listener
// immediately.
func (s *Server) Stop() (err error) {

	if runtime.GOOS != "windows" {
		// force connections to close after timeout
		done := make(chan struct{})
		go func() {
			s.dnsWg.Done() // decrement our initial increment used as a barrier
			s.dnsWg.Wait()
			close(done)
		}()

		// Wait for remaining connections to finish or
		// force them all to close after timeout
		select {
		case <-time.After(s.connTimeout):
		case <-done:
		}
	}

	// Close the listener now; this stops the server without delay
	s.m.Lock()
	if s.l != nil {
		err = s.l.Close()
	}
	if s.p != nil {
		err = s.p.Close()
	}

	for _, s1 := range s.server {
		err = s1.Shutdown()
	}
	s.m.Unlock()
	return
}

// ServeDNS is the entry point for every request to the address that s
// is bound to. It acts as a multiplexer for the requests zonename as
// defined in the request so that the correct zone
// (configuration and middleware stack) will handle the request.
func (s *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	// TODO(miek): expensive to use defer
	defer func() {
		// In case the user doesn't enable error middleware, we still
		// need to make sure that we stay alive up here
		if rec := recover(); rec != nil {
			DefaultErrorFunc(w, r, dns.RcodeServerFailure)
		}
	}()

	if m, err := edns.Version(r); err != nil { // Wrong EDNS version, return at once.
		w.WriteMsg(m)
		return
	}

	q := r.Question[0].Name
	b := make([]byte, len(q))
	off, end := 0, false
	ctx := context.Background()

	var dshandler *Config

	for {
		l := len(q[off:])
		for i := 0; i < l; i++ {
			b[i] = q[off+i]
			// normalize the name for the lookup
			if b[i] >= 'A' && b[i] <= 'Z' {
				b[i] |= ('a' - 'A')
			}
		}

		if h, ok := s.zones[string(b[:l])]; ok {
			if r.Question[0].Qtype != dns.TypeDS {
				rcode, _ := h.middlewareChain.ServeDNS(ctx, w, r)
				if rcodeNoClientWrite(rcode) {
					DefaultErrorFunc(w, r, rcode)
				}
				return
			}
			// The type is DS, keep the handler, but keep on searching as maybe we are serving
			// the parent as well and the DS should be routed to it - this will probably *misroute* DS
			// queries to a possibly grand parent, but there is no way for us to know at this point
			// if there is an actually delegation from grandparent -> parent -> zone.
			// In all fairness: direct DS queries should not be needed.
			dshandler = h
		}
		off, end = dns.NextLabel(q, off)
		if end {
			break
		}
	}

	if dshandler != nil {
		// DS request, and we found a zone, use the handler for the query
		rcode, _ := dshandler.middlewareChain.ServeDNS(ctx, w, r)
		if rcodeNoClientWrite(rcode) {
			DefaultErrorFunc(w, r, rcode)
		}
		return
	}

	// Wildcard match, if we have found nothing try the root zone as a last resort.
	if h, ok := s.zones["."]; ok {
		rcode, _ := h.middlewareChain.ServeDNS(ctx, w, r)
		if rcodeNoClientWrite(rcode) {
			DefaultErrorFunc(w, r, rcode)
		}
		return
	}

	// Still here? Error out with REFUSED and some logging
	remoteHost := w.RemoteAddr().String()
	DefaultErrorFunc(w, r, dns.RcodeRefused)
	log.Printf("[INFO] \"%s %s %s\" - No such zone at %s (Remote: %s)", dns.Type(r.Question[0].Qtype), dns.Class(r.Question[0].Qclass), q, s.Addr, remoteHost)
}

// OnStartupComplete lists the sites served by this server
// and any relevant information, assuming Quiet is false.
func (s *Server) OnStartupComplete() {
	if Quiet {
		return
	}

	for zone, config := range s.zones {
		fmt.Println(zone + ":" + config.Port)
	}
}

// DefaultErrorFunc responds to an DNS request with an error.
func DefaultErrorFunc(w dns.ResponseWriter, r *dns.Msg, rc int) {
	state := request.Request{W: w, Req: r}

	answer := new(dns.Msg)
	answer.SetRcode(r, rc)

	state.SizeAndDo(answer)

	vars.Report(state, vars.Dropped, rcode.ToString(rc), answer.Len(), time.Now())

	w.WriteMsg(answer)
}

func rcodeNoClientWrite(rcode int) bool {
	switch rcode {
	case dns.RcodeServerFailure:
		fallthrough
	case dns.RcodeRefused:
		fallthrough
	case dns.RcodeFormatError:
		fallthrough
	case dns.RcodeNotImplemented:
		return true
	}
	return false
}

const (
	tcp = 0
	udp = 1
)

var (
	// Quiet mode will not show any informative output on initialization.
	Quiet bool
)
