// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"context"
	"fmt"
	agentdns "github.com/hashicorp/consul/agent/dns"
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

func (s *Server) Register(registrar grpc.ServiceRegistrar) {
	pbdns.RegisterDNSServiceServer(registrar, s)
}

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

	reqCtx, err := agentdns.NewContextFromGRPCContext(ctx)
	if err != nil {
		s.Logger.Error("error parsing DNS context from grpc metadata", "err", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("error parsing DNS context from grpc metadata: %s", err.Error()))
	}

	respWriter := &agentdns.BufferResponseWriter{
		LocalAddress:   local,
		RemoteAddress:  remote,
		Logger:         s.Logger,
		RequestContext: reqCtx,
	}

	msg := &dns.Msg{}
	err = msg.Unpack(req.Msg)
	if err != nil {
		s.Logger.Error("error unpacking message", "err", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failure decoding dns request: %s", err.Error()))
	}

	s.DNSServeMux.ServeDNS(respWriter, msg)

	queryResponse := &pbdns.QueryResponse{Msg: respWriter.ResponseBuffer()}

	return queryResponse, nil
}
