package dnsserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"

	"github.com/coredns/coredns/pb"
	"github.com/coredns/coredns/plugin/pkg/reuseport"
	"github.com/coredns/coredns/plugin/pkg/transport"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/miekg/dns"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// ServergRPC represents an instance of a DNS-over-gRPC server.
type ServergRPC struct {
	*Server
	grpcServer *grpc.Server
	listenAddr net.Addr
	tlsConfig  *tls.Config
}

// NewServergRPC returns a new CoreDNS GRPC server and compiles all plugin in to it.
func NewServergRPC(addr string, group []*Config) (*ServergRPC, error) {
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

	return &ServergRPC{Server: s, tlsConfig: tlsConfig}, nil
}

// Serve implements caddy.TCPServer interface.
func (s *ServergRPC) Serve(l net.Listener) error {
	s.m.Lock()
	s.listenAddr = l.Addr()
	s.m.Unlock()

	if s.Tracer() != nil {
		onlyIfParent := func(parentSpanCtx opentracing.SpanContext, method string, req, resp interface{}) bool {
			return parentSpanCtx != nil
		}
		intercept := otgrpc.OpenTracingServerInterceptor(s.Tracer(), otgrpc.IncludingSpans(onlyIfParent))
		s.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(intercept))
	} else {
		s.grpcServer = grpc.NewServer()
	}

	pb.RegisterDnsServiceServer(s.grpcServer, s)

	if s.tlsConfig != nil {
		l = tls.NewListener(l, s.tlsConfig)
	}
	return s.grpcServer.Serve(l)
}

// ServePacket implements caddy.UDPServer interface.
func (s *ServergRPC) ServePacket(p net.PacketConn) error { return nil }

// Listen implements caddy.TCPServer interface.
func (s *ServergRPC) Listen() (net.Listener, error) {

	l, err := reuseport.Listen("tcp", s.Addr[len(transport.GRPC+"://"):])
	if err != nil {
		return nil, err
	}
	return l, nil
}

// ListenPacket implements caddy.UDPServer interface.
func (s *ServergRPC) ListenPacket() (net.PacketConn, error) { return nil, nil }

// OnStartupComplete lists the sites served by this server
// and any relevant information, assuming Quiet is false.
func (s *ServergRPC) OnStartupComplete() {
	if Quiet {
		return
	}

	out := startUpZones(transport.GRPC+"://", s.Addr, s.zones)
	if out != "" {
		fmt.Print(out)
	}
}

// Stop stops the server. It blocks until the server is
// totally stopped.
func (s *ServergRPC) Stop() (err error) {
	s.m.Lock()
	defer s.m.Unlock()
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	return
}

// Query is the main entry-point into the gRPC server. From here we call ServeDNS like
// any normal server. We use a custom responseWriter to pick up the bytes we need to write
// back to the client as a protobuf.
func (s *ServergRPC) Query(ctx context.Context, in *pb.DnsPacket) (*pb.DnsPacket, error) {
	msg := new(dns.Msg)
	err := msg.Unpack(in.Msg)
	if err != nil {
		return nil, err
	}

	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, errors.New("no peer in gRPC context")
	}

	a, ok := p.Addr.(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("no TCP peer in gRPC context: %v", p.Addr)
	}

	w := &gRPCresponse{localAddr: s.listenAddr, remoteAddr: a, Msg: msg}

	dnsCtx := context.WithValue(ctx, Key{}, s.Server)
	s.ServeDNS(dnsCtx, w, msg)

	packed, err := w.Msg.Pack()
	if err != nil {
		return nil, err
	}

	return &pb.DnsPacket{Msg: packed}, nil
}

// Shutdown stops the server (non gracefully).
func (s *ServergRPC) Shutdown() error {
	if s.grpcServer != nil {
		s.grpcServer.Stop()
	}
	return nil
}

type gRPCresponse struct {
	localAddr  net.Addr
	remoteAddr net.Addr
	Msg        *dns.Msg
}

// Write is the hack that makes this work. It does not actually write the message
// but returns the bytes we need to write in r. We can then pick this up in Query
// and write a proper protobuf back to the client.
func (r *gRPCresponse) Write(b []byte) (int, error) {
	r.Msg = new(dns.Msg)
	return len(b), r.Msg.Unpack(b)
}

// These methods implement the dns.ResponseWriter interface from Go DNS.
func (r *gRPCresponse) Close() error              { return nil }
func (r *gRPCresponse) TsigStatus() error         { return nil }
func (r *gRPCresponse) TsigTimersOnly(b bool)     {}
func (r *gRPCresponse) Hijack()                   {}
func (r *gRPCresponse) LocalAddr() net.Addr       { return r.localAddr }
func (r *gRPCresponse) RemoteAddr() net.Addr      { return r.remoteAddr }
func (r *gRPCresponse) WriteMsg(m *dns.Msg) error { r.Msg = m; return nil }
