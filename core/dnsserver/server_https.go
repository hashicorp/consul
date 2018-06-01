package dnsserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/miekg/dns"
)

// ServerHTTPS represents an instance of a DNS-over-HTTPS server.
type ServerHTTPS struct {
	*Server
	httpsServer *http.Server
	listenAddr  net.Addr
	tlsConfig   *tls.Config
}

// NewServerHTTPS returns a new CoreDNS GRPC server and compiles all plugins in to it.
func NewServerHTTPS(addr string, group []*Config) (*ServerHTTPS, error) {
	s, err := NewServer(addr, group)
	if err != nil {
		return nil, err
	}
	// The *tls* plugin must make sure that multiple conflicting
	// TLS configuration return an error: it can only be specified once.
	var tlsConfig *tls.Config
	for _, conf := range s.zones {
		// Should we error if some configs *don't* have TLS?
		tlsConfig = conf.TLSConfig
	}

	sh := &ServerHTTPS{Server: s, tlsConfig: tlsConfig, httpsServer: new(http.Server)}
	sh.httpsServer.Handler = sh

	return sh, nil
}

// Serve implements caddy.TCPServer interface.
func (s *ServerHTTPS) Serve(l net.Listener) error {
	s.m.Lock()
	s.listenAddr = l.Addr()
	s.m.Unlock()

	if s.tlsConfig != nil {
		l = tls.NewListener(l, s.tlsConfig)
	}
	return s.httpsServer.Serve(l)
}

// ServePacket implements caddy.UDPServer interface.
func (s *ServerHTTPS) ServePacket(p net.PacketConn) error { return nil }

// Listen implements caddy.TCPServer interface.
func (s *ServerHTTPS) Listen() (net.Listener, error) {

	l, err := net.Listen("tcp", s.Addr[len(TransportHTTPS+"://"):])
	if err != nil {
		return nil, err
	}
	return l, nil
}

// ListenPacket implements caddy.UDPServer interface.
func (s *ServerHTTPS) ListenPacket() (net.PacketConn, error) { return nil, nil }

// OnStartupComplete lists the sites served by this server
// and any relevant information, assuming Quiet is false.
func (s *ServerHTTPS) OnStartupComplete() {
	if Quiet {
		return
	}

	out := startUpZones(TransportHTTPS+"://", s.Addr, s.zones)
	if out != "" {
		fmt.Print(out)
	}
	return
}

// Stop stops the server. It blocks until the server is totally stopped.
func (s *ServerHTTPS) Stop() error {
	s.m.Lock()
	defer s.m.Unlock()
	if s.httpsServer != nil {
		s.httpsServer.Shutdown(context.Background())
	}
	return nil
}

// ServeHTTP is the handler that gets the HTTP request and converts to the dns format, calls the plugin
// chain, converts it back and write it to the client.
func (s *ServerHTTPS) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	msg := new(dns.Msg)
	var err error

	if r.URL.Path != pathDOH {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPost:
		msg, err = postRequestToMsg(r)
	case http.MethodGet:
		msg, err = getRequestToMsg(r)
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create a DoHWriter with the correct addresses in it.
	h, p, _ := net.SplitHostPort(r.RemoteAddr)
	port, _ := strconv.Atoi(p)
	dw := &DoHWriter{laddr: s.listenAddr, raddr: &net.TCPAddr{IP: net.ParseIP(h), Port: port}}

	// We just call the normal chain handler - all error handling is done there.
	// We should expect a packet to be returned that we can send to the client.
	s.ServeDNS(context.Background(), dw, msg)

	buf, _ := dw.Msg.Pack()

	w.Header().Set("Content-Type", mimeTypeDOH)
	w.Header().Set("Cache-Control", "max-age=128") // TODO(issues/1823): implement proper fix.
	w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
	w.WriteHeader(http.StatusOK)

	w.Write(buf)
}

// Shutdown stops the server (non gracefully).
func (s *ServerHTTPS) Shutdown() error {
	if s.httpsServer != nil {
		s.httpsServer.Shutdown(context.Background())
	}
	return nil
}
