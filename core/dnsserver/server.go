// Package dnsserver implements all the interfaces from Caddy, so that CoreDNS can be a servertype plugin.
package dnsserver

import (
	"fmt"
	"log"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics/vars"
	"github.com/coredns/coredns/plugin/pkg/edns"
	"github.com/coredns/coredns/plugin/pkg/rcode"
	"github.com/coredns/coredns/plugin/pkg/trace"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
	ot "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
)

// Server represents an instance of a server, which serves
// DNS requests at a particular address (host and port). A
// server is capable of serving numerous zones on
// the same address and the listener may be stopped for
// graceful termination (POSIX only).
type Server struct {
	Addr string // Address we listen on

	server [2]*dns.Server // 0 is a net.Listener, 1 is a net.PacketConn (a *UDPConn) in our case.
	m      sync.Mutex     // protects the servers

	zones       map[string]*Config // zones keyed by their address
	dnsWg       sync.WaitGroup     // used to wait on outstanding connections
	connTimeout time.Duration      // the maximum duration of a graceful shutdown
	trace       trace.Trace        // the trace plugin for the server
	debug       bool               // disable recover()
	classChaos  bool               // allow non-INET class queries
}

// NewServer returns a new CoreDNS server and compiles all plugins in to it. By default CH class
// queries are blocked unless the chaos or proxy is loaded.
func NewServer(addr string, group []*Config) (*Server, error) {

	s := &Server{
		Addr:        addr,
		zones:       make(map[string]*Config),
		connTimeout: 5 * time.Second, // TODO(miek): was configurable
	}

	// We have to bound our wg with one increment
	// to prevent a "race condition" that is hard-coded
	// into sync.WaitGroup.Wait() - basically, an add
	// with a positive delta must be guaranteed to
	// occur before Wait() is called on the wg.
	// In a way, this kind of acts as a safety barrier.
	s.dnsWg.Add(1)

	for _, site := range group {
		if site.Debug {
			s.debug = true
		}
		// set the config per zone
		s.zones[site.Zone] = site
		// compile custom plugin for everything
		var stack plugin.Handler
		for i := len(site.Plugin) - 1; i >= 0; i-- {
			stack = site.Plugin[i](stack)

			// register the *handler* also
			site.registerHandler(stack)

			if s.trace == nil && stack.Name() == "trace" {
				// we have to stash away the plugin, not the
				// Tracer object, because the Tracer won't be initialized yet
				if t, ok := stack.(trace.Trace); ok {
					s.trace = t
				}
			}
			if stack.Name() == "chaos" || stack.Name() == "proxy" {
				s.classChaos = true
			}
		}
		site.pluginChain = stack
	}

	return s, nil
}

// Serve starts the server with an existing listener. It blocks until the server stops.
// This implements caddy.TCPServer interface.
func (s *Server) Serve(l net.Listener) error {
	s.m.Lock()
	s.server[tcp] = &dns.Server{Listener: l, Net: "tcp", Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		ctx := context.Background()
		s.ServeDNS(ctx, w, r)
	})}
	s.m.Unlock()

	return s.server[tcp].ActivateAndServe()
}

// ServePacket starts the server with an existing packetconn. It blocks until the server stops.
// This implements caddy.UDPServer interface.
func (s *Server) ServePacket(p net.PacketConn) error {
	s.m.Lock()
	s.server[udp] = &dns.Server{PacketConn: p, Net: "udp", Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		ctx := context.Background()
		s.ServeDNS(ctx, w, r)
	})}
	s.m.Unlock()

	return s.server[udp].ActivateAndServe()
}

// Listen implements caddy.TCPServer interface.
func (s *Server) Listen() (net.Listener, error) {
	l, err := net.Listen("tcp", s.Addr[len(TransportDNS+"://"):])
	if err != nil {
		return nil, err
	}
	return l, nil
}

// ListenPacket implements caddy.UDPServer interface.
func (s *Server) ListenPacket() (net.PacketConn, error) {
	p, err := net.ListenPacket("udp", s.Addr[len(TransportDNS+"://"):])
	if err != nil {
		return nil, err
	}

	return p, nil
}

// Stop stops the server. It blocks until the server is
// totally stopped. On POSIX systems, it will wait for
// connections to close (up to a max timeout of a few
// seconds); on Windows it will close the listener
// immediately.
// This implements Caddy.Stopper interface.
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
	for _, s1 := range s.server {
		// We might not have started and initialized the full set of servers
		if s1 != nil {
			err = s1.Shutdown()
		}
	}
	s.m.Unlock()
	return
}

// Address together with Stop() implement caddy.GracefulServer.
func (s *Server) Address() string { return s.Addr }

// ServeDNS is the entry point for every request to the address that s
// is bound to. It acts as a multiplexer for the requests zonename as
// defined in the request so that the correct zone
// (configuration and plugin stack) will handle the request.
func (s *Server) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
	// The default dns.Mux checks the question section size, but we have our
	// own mux here. Check if we have a question section. If not drop them here.
	if r == nil || len(r.Question) == 0 {
		DefaultErrorFunc(w, r, dns.RcodeServerFailure)
		return
	}

	if !s.debug {
		defer func() {
			// In case the user doesn't enable error plugin, we still
			// need to make sure that we stay alive up here
			if rec := recover(); rec != nil {
				DefaultErrorFunc(w, r, dns.RcodeServerFailure)
			}
		}()
	}

	if !s.classChaos && r.Question[0].Qclass != dns.ClassINET {
		DefaultErrorFunc(w, r, dns.RcodeRefused)
		return
	}

	if m, err := edns.Version(r); err != nil { // Wrong EDNS version, return at once.
		w.WriteMsg(m)
		return
	}

	q := r.Question[0].Name
	b := make([]byte, len(q))
	var off int
	var end bool

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
				if h.FilterFunc == nil {
					rcode, _ := h.pluginChain.ServeDNS(ctx, w, r)
					if !plugin.ClientWrite(rcode) {
						DefaultErrorFunc(w, r, rcode)
					}
					return
				}
				// FilterFunc is set, call it to see if we should use this handler.
				// This is given to full query name.
				if h.FilterFunc(q) {
					rcode, _ := h.pluginChain.ServeDNS(ctx, w, r)
					if !plugin.ClientWrite(rcode) {
						DefaultErrorFunc(w, r, rcode)
					}
					return
				}
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

	if r.Question[0].Qtype == dns.TypeDS && dshandler != nil && dshandler.pluginChain != nil {
		// DS request, and we found a zone, use the handler for the query.
		rcode, _ := dshandler.pluginChain.ServeDNS(ctx, w, r)
		if !plugin.ClientWrite(rcode) {
			DefaultErrorFunc(w, r, rcode)
		}
		return
	}

	// Wildcard match, if we have found nothing try the root zone as a last resort.
	if h, ok := s.zones["."]; ok && h.pluginChain != nil {
		rcode, _ := h.pluginChain.ServeDNS(ctx, w, r)
		if !plugin.ClientWrite(rcode) {
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

// Tracer returns the tracer in the server if defined.
func (s *Server) Tracer() ot.Tracer {
	if s.trace == nil {
		return nil
	}

	return s.trace.Tracer()
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

const (
	tcp = 0
	udp = 1
)

// Quiet mode will not show any informative output on initialization.
var Quiet bool
