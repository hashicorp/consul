// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/auth"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/proto-public/pbacl"
)

// Login exchanges the presented bearer token for a Consul ACL token using a
// configured auth method.
func (s *Server) Login(ctx context.Context, req *pbacl.LoginRequest) (*pbacl.LoginResponse, error) {
	fmt.Println("============================> Login called ", req)

	logger := s.Logger.Named("login").With("request_id", external.TraceID())
	logger.Trace("request received")
	fmt.Println("============================> Login 11111111 ")

	if err := s.requireACLsEnabled(logger); err != nil {
		return nil, err
	}
	fmt.Println("============================> Login 2222 ")

	entMeta := acl.NewEnterpriseMetaWithPartition(req.Partition, req.Namespace)

	if err := s.ValidateEnterpriseRequest(&entMeta, true); err != nil {
		logger.Error("error during enterprise request validation", "error", err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	fmt.Println("============================> Login 3333333 ")

	// Forward request to leader in the correct datacenter.
	var rsp *pbacl.LoginResponse
	handled, err := s.forwardWriteDC(req.Datacenter, func(conn *grpc.ClientConn) error {
		fmt.Println("============================> Login 3.1111111111 ")

		var err error
		rsp, err = pbacl.NewACLServiceClient(conn).Login(ctx, req)
		fmt.Println("============================> Login 3.22222222 ", rsp, err)
		return err
	}, logger)
	if handled || err != nil {
		fmt.Println("============================> Login 3.3333333 ", rsp, err)
		return rsp, err
	}
	fmt.Println("============================> Login 44444444 ", req)

	// This is also validated by the TokenWriter, but doing it early saves any
	// work done by the validator (e.g. roundtrip to the Kubernetes API server).
	if err := s.requireLocalTokens(logger); err != nil {
		return nil, err
	}
	fmt.Println("============================> Login 555555 ", req)

	authMethod, validator, err := s.LoadAuthMethod(req.AuthMethod, &entMeta)
	fmt.Println("============================> Login 666666 ", authMethod, validator, err)

	switch {
	case errors.Is(err, acl.ErrACLNotFound):
		fmt.Println("============================> Login 66666666 ", acl.ErrACLNotFound)
		return nil, status.Errorf(codes.InvalidArgument, "auth method %q not found", req.AuthMethod)
	case err != nil:
		fmt.Println("============================> Login 7777777 ", err)

		logger.Error("failed to load auth method", "error", err.Error())
		return nil, status.Error(codes.Internal, "failed to load auth method")
	}

	verifiedIdentity, err := validator.ValidateLogin(ctx, req.BearerToken)
	fmt.Println("============================> Login 8888888 ", verifiedIdentity, err)

	if err != nil {

		// TODO(agentless): errors returned from validators aren't standardized so
		// it's hard to tell whether validation failed because of an invalid bearer
		// token or something internal/transient. We currently return Unauthenticated
		// for all errors because it's the most likely, but we should make validators
		// return a typed or sentinel error instead.
		logger.Error("failed to validate login", "error", err.Error())
		fmt.Println("============================> Login 99999999 ", verifiedIdentity, err)

		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	description, err := auth.BuildTokenDescription("token created via login", req.Meta)
	fmt.Println("============================> Login 10000000000 ", description, err)

	if err != nil {
		logger.Error("failed to build token description", "error", err.Error())
		fmt.Println("============================> Login 11111111111 ", description, err)

		return nil, status.Error(codes.Internal, err.Error())
	}

	token, err := s.NewLogin().TokenForVerifiedIdentity(verifiedIdentity, authMethod, description)
	fmt.Println("============================> Login 121212121212 ", token, err)

	switch {
	case acl.IsErrPermissionDenied(err):
		fmt.Println("============================> Login 13131313131313 ", token, err)

		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		logger.Error("failed to create token", "error", err.Error())
		fmt.Println("============================> Login 14141414141414 ", token, err)
		return nil, status.Error(codes.Internal, "failed to create token")
	}

	return &pbacl.LoginResponse{
		Token: &pbacl.LoginToken{
			AccessorId: token.AccessorID,
			SecretId:   token.SecretID,
		},
	}, nil
}
