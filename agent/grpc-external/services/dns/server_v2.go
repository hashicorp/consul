// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"context"
	"fmt"
	"net"

	"github.com/miekg/dns"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/go-hclog"

	agentdns "github.com/hashicorp/consul/agent/dns"
	"github.com/hashicorp/consul/proto-public/pbdns"
)

type ConfigV2 struct {
	DNSRouter agentdns.DNSRouter
	Logger    hclog.Logger
	TokenFunc func() string
}

var _ pbdns.DNSServiceServer = (*ServerV2)(nil)

// ServerV2 is a gRPC server that implements pbdns.DNSServiceServer.
// It is compatible with the refactored V2 DNS server and suitable for
// passing additional metadata along the grpc connection to catalog queries.
type ServerV2 struct {
	ConfigV2
}

func NewServerV2(cfg ConfigV2) *ServerV2 {
	return &ServerV2{cfg}
}

func (s *ServerV2) Register(registrar grpc.ServiceRegistrar) {
	pbdns.RegisterDNSServiceServer(registrar, s)
}

// Query is a gRPC endpoint that will serve dns requests. It will be consumed primarily by the
// consul dataplane to proxy dns requests to consul.
func (s *ServerV2) Query(ctx context.Context, req *pbdns.QueryRequest) (*pbdns.QueryResponse, error) {
	pr, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("error retrieving peer information from context")
	}

	var remote net.Addr
	// We do this so that we switch to udp/tcp when handling the request since it will be proxied
	// through consul through gRPC, and we need to 'fake' the protocol so that the message is trimmed
	// according to whether it is UDP or TCP.
	switch req.GetProtocol() {
	case pbdns.Protocol_PROTOCOL_TCP:
		remote = pr.Addr
	case pbdns.Protocol_PROTOCOL_UDP:
		remoteAddr := pr.Addr.(*net.TCPAddr)
		remote = &net.UDPAddr{IP: remoteAddr.IP, Port: remoteAddr.Port}
	default:
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("error protocol type not set: %v", req.GetProtocol()))
	}

	msg := &dns.Msg{}
	err := msg.Unpack(req.Msg)
	if err != nil {
		s.Logger.Error("error unpacking message", "err", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failure decoding dns request: %s", err.Error()))
	}

	reqCtx, err := agentdns.NewContextFromGRPCContext(ctx)
	if err != nil {
		s.Logger.Error("error parsing DNS context from grpc metadata", "err", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("error parsing DNS context from grpc metadata: %s", err.Error()))
	}

	resp := s.DNSRouter.HandleRequest(msg, reqCtx, remote)
	data, err := resp.Pack()
	if err != nil {
		s.Logger.Error("error packing message", "err", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failure encoding dns request: %s", err.Error()))
	}

	queryResponse := &pbdns.QueryResponse{Msg: data}
	return queryResponse, nil
}
