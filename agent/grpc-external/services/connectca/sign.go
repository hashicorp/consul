// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connectca

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
)

// Sign a leaf certificate for the service or agent identified by the SPIFFE
// ID in the given CSR's SAN.
func (s *Server) Sign(ctx context.Context, req *pbconnectca.SignRequest) (*pbconnectca.SignResponse, error) {
	if err := s.requireConnect(); err != nil {
		return nil, err
	}

	logger := s.Logger.Named("sign").With("request_id", external.TraceID())
	logger.Trace("request received")

	options, err := external.QueryOptionsFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.Csr == "" {
		return nil, status.Error(codes.InvalidArgument, "CSR is required")
	}

	// For private/internal gRPC handlers, protoc-gen-rpc-glue generates the
	// requisite methods to satisfy the structs.RPCInfo interface using fields
	// from the pbcommon package. This service is public, so we can't use those
	// fields in our proto definition. Instead, we construct our RPCInfo manually.
	//
	// Embedding WriteRequest ensures RPCs are forwarded to the leader, embedding
	// DCSpecificRequest adds the RequestDatacenter method (but as we're not
	// setting Datacenter it has the effect of *not* doing DC forwarding).
	var rpcInfo struct {
		structs.WriteRequest
		structs.DCSpecificRequest
	}
	rpcInfo.Token = options.Token

	var rsp *pbconnectca.SignResponse
	handled, err := s.ForwardRPC(&rpcInfo, func(conn *grpc.ClientConn) error {
		logger.Trace("forwarding RPC")
		ctx := external.ForwardMetadataContext(ctx)
		var err error
		rsp, err = pbconnectca.NewConnectCAServiceClient(conn).Sign(ctx, req)
		return err
	})
	if handled || err != nil {
		return rsp, err
	}

	csr, err := connect.ParseCSR(req.Csr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(options.Token, nil, nil)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	cert, err := s.CAManager.AuthorizeAndSignCertificate(csr, authz)
	switch {
	case connect.IsInvalidCSRError(err):
		return nil, status.Error(codes.InvalidArgument, err.Error())
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case isRateLimitError(err):
		return nil, status.Error(codes.ResourceExhausted, err.Error())
	case err != nil:
		logger.Error("failed to sign leaf certificate", "error", err.Error())
		return nil, status.Error(codes.Internal, "failed to sign leaf certificate")
	}

	return &pbconnectca.SignResponse{
		CertPem: cert.CertPEM,
	}, nil
}

// TODO(agentless): CAManager currently lives in the `agent/consul` package and
// returns ErrRateLimited which we can't reference directly here because it'd
// create an import cycle. Checking the error message like this is fragile, but
// because of net/rpc's limited error handling support it's what we already do
// on the client. We should either move the error constant so that can use it
// here, or perhaps make it a typed error?
func isRateLimitError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "limit reached")
}
