package dnstest

import (
	"net"

	"github.com/coredns/coredns/plugin/pkg/reuseport"

	"github.com/miekg/dns"
)

// A Server is an DNS server listening on a system-chosen port on the local
// loopback interface, for use in end-to-end DNS tests.
type Server struct {
	Addr string // Address where the server listening.

	s1 *dns.Server // udp
	s2 *dns.Server // tcp
}

// NewServer starts and returns a new Server. The caller should call Close when
// finished, to shut it down.
func NewServer(f dns.HandlerFunc) *Server {
	dns.HandleFunc(".", f)

	ch1 := make(chan bool)
	ch2 := make(chan bool)

	s1 := &dns.Server{} // udp
	s2 := &dns.Server{} // tcp

	for i := 0; i < 5; i++ { // 5 attempts
		s2.Listener, _ = reuseport.Listen("tcp", ":0")
		if s2.Listener == nil {
			continue
		}

		s1.PacketConn, _ = net.ListenPacket("udp", s2.Listener.Addr().String())
		if s1.PacketConn != nil {
			break
		}

		// perhaps UPD port is in use, try again
		s2.Listener.Close()
		s2.Listener = nil
	}
	if s2.Listener == nil {
		panic("dnstest.NewServer(): failed to create new server")
	}

	s1.NotifyStartedFunc = func() { close(ch1) }
	s2.NotifyStartedFunc = func() { close(ch2) }
	go s1.ActivateAndServe()
	go s2.ActivateAndServe()

	<-ch1
	<-ch2

	return &Server{s1: s1, s2: s2, Addr: s2.Listener.Addr().String()}
}

// Close shuts down the server.
func (s *Server) Close() {
	s.s1.Shutdown()
	s.s2.Shutdown()
}
