package dnsserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/miekg/dns"
)

// serverTLS represents an instance of a TLS-over-DNS-server.
type serverTLS struct {
	*Server
}

// NewTLSServer returns a new CoreDNS TLS server and compiles all middleware in to it.
func NewServerTLS(addr string, group []*Config) (*serverTLS, error) {

	s, err := NewServer(addr, group)
	if err != nil {
		return nil, err
	}

	return &serverTLS{Server: s}, nil
}

// Serve implements caddy.TCPServer interface.
func (s *serverTLS) Serve(l net.Listener) error {
	s.m.Lock()

	// Only fill out the TCP server for this one.
	s.server[tcp] = &dns.Server{Listener: l, Net: "tcp-tls", Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		ctx := context.Background()
		s.ServeDNS(ctx, w, r)
	})}
	s.m.Unlock()

	return s.server[tcp].ActivateAndServe()
}

// ServePacket implements caddy.UDPServer interface.
func (s *serverTLS) ServePacket(p net.PacketConn) error { return nil }

// Listen implements caddy.TCPServer interface.
func (s *serverTLS) Listen() (net.Listener, error) {
	// The *tls* middleware must make sure that multiple conflicting
	// TLS configuration return an error: it can only be specified once.
	tlsConfig := new(tls.Config)
	for _, conf := range s.zones {
		// Should we error if some configs *don't* have TLS?
		tlsConfig = conf.TLSConfig
	}

	var (
		l   net.Listener
		err error
	)

	if tlsConfig == nil {
		l, err = net.Listen("tcp", s.Addr[len(TransportTLS+"://"):])
	} else {
		l, err = tls.Listen("tcp", s.Addr[len(TransportTLS+"://"):], tlsConfig)
	}

	if err != nil {
		return nil, err
	}
	return l, nil
}

// ListenPacket implements caddy.UDPServer interface.
func (s *serverTLS) ListenPacket() (net.PacketConn, error) { return nil, nil }

// OnStartupComplete lists the sites served by this server
// and any relevant information, assuming Quiet is false.
func (s *serverTLS) OnStartupComplete() {
	if Quiet {
		return
	}

	for zone, config := range s.zones {
		fmt.Println(TransportTLS + "://" + zone + ":" + config.Port)
	}
}
