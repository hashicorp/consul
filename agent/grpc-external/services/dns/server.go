// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package dns

import (
	"context"
	"fmt"
	"net"

	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/proto-public/pbdns"
)

type LocalAddr struct {
	IP   net.IP
	Port int
}

type Config struct {
	Logger      hclog.Logger
	DNSServeMux *dns.ServeMux
	LocalAddr   LocalAddr
}

type Server struct {
	Config
}

func NewServer(cfg Config) *Server {
	return &Server{cfg}
}

func (s *Server) Register(grpcServer *grpc.Server) {
	pbdns.RegisterDNSServiceServer(grpcServer, s)
}

// BufferResponseWriter writes a DNS response to a byte buffer.
type BufferResponseWriter struct {
	responseBuffer []byte
	LocalAddress   net.Addr
	RemoteAddress  net.Addr
	Logger         hclog.Logger
}

// LocalAddr returns the net.Addr of the server
func (b *BufferResponseWriter) LocalAddr() net.Addr {
	return b.LocalAddress
}

// RemoteAddr returns the net.Addr of the client that sent the current request.
func (b *BufferResponseWriter) RemoteAddr() net.Addr {
	return b.RemoteAddress
}

// WriteMsg writes a reply back to the client.
func (b *BufferResponseWriter) WriteMsg(m *dns.Msg) error {
	// Pack message to bytes first.
	msgBytes, err := m.Pack()
	if err != nil {
		b.Logger.Error("error packing message", "err", err)
		return err
	}
	b.responseBuffer = msgBytes
	return nil
}

// Write writes a raw buffer back to the client.
func (b *BufferResponseWriter) Write(m []byte) (int, error) {
	b.Logger.Debug("Write was called")
	return copy(b.responseBuffer, m), nil
}

// Close closes the connection.
func (b *BufferResponseWriter) Close() error {
	// There's nothing for us to do here as we don't handle the connection.
	return nil
}

// TsigStatus returns the status of the Tsig.
func (b *BufferResponseWriter) TsigStatus() error {
	// TSIG doesn't apply to this response writer.
	return nil
}

// TsigTimersOnly sets the tsig timers only boolean.
func (b *BufferResponseWriter) TsigTimersOnly(bool) {}

// Hijack lets the caller take over the connection.
// After a call to Hijack(), the DNS package will not do anything with the connection. {
func (b *BufferResponseWriter) Hijack() {}

// Query is a gRPC endpoint that will serve dns requests. It will be consumed primarily by the
// consul dataplane to proxy dns requests to consul.
func (s *Server) Query(ctx context.Context, req *pbdns.QueryRequest) (*pbdns.QueryResponse, error) {
	pr, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("error retrieving peer information from context")
	}

	var local net.Addr
	var remote net.Addr
	// We do this so that we switch to udp/tcp when handling the request since it will be proxied
	// through consul through gRPC and we need to 'fake' the protocol so that the message is trimmed
	// according to wether it is UDP or TCP.
	switch req.GetProtocol() {
	case pbdns.Protocol_PROTOCOL_TCP:
		remote = pr.Addr
		local = &net.TCPAddr{IP: s.LocalAddr.IP, Port: s.LocalAddr.Port}
	case pbdns.Protocol_PROTOCOL_UDP:
		remoteAddr := pr.Addr.(*net.TCPAddr)
		remote = &net.UDPAddr{IP: remoteAddr.IP, Port: remoteAddr.Port}
		local = &net.UDPAddr{IP: s.LocalAddr.IP, Port: s.LocalAddr.Port}
	default:
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("error protocol type not set: %v", req.GetProtocol()))
	}

	respWriter := &BufferResponseWriter{
		LocalAddress:  local,
		RemoteAddress: remote,
		Logger:        s.Logger,
	}

	msg := &dns.Msg{}
	err := msg.Unpack(req.Msg)
	if err != nil {
		s.Logger.Error("error unpacking message", "err", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failure decoding dns request: %s", err.Error()))
	}
	s.DNSServeMux.ServeDNS(respWriter, msg)

	queryResponse := &pbdns.QueryResponse{Msg: respWriter.responseBuffer}

	return queryResponse, nil
}
