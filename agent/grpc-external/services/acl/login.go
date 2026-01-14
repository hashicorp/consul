// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package acl

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	fmt.Println(time.Now().String()+" ===================>  grpc login called by Login with bearer token = ", req.BearerToken)

	logger := s.Logger.Named("login").With("request_id", external.TraceID())
	logger.Trace("request received")

	if err := s.requireACLsEnabled(logger); err != nil {
		return nil, err
	}
	fmt.Println(time.Now().String()+" ===================>  grpc login called by Login with bearer token 1 = ", req.BearerToken)

	entMeta := acl.NewEnterpriseMetaWithPartition(req.Partition, req.Namespace)

	if err := s.ValidateEnterpriseRequest(&entMeta, true); err != nil {
		logger.Error("error during enterprise request validation", "error", err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	fmt.Println(time.Now().String()+" ===================>  grpc login called by Login with bearer token 3 = ", req.BearerToken)

	// Forward request to leader in the correct datacenter.
	var rsp *pbacl.LoginResponse
	handled, err := s.forwardWriteDC(req.Datacenter, func(conn *grpc.ClientConn) error {
		var err error
		rsp, err = pbacl.NewACLServiceClient(conn).Login(ctx, req)
		return err
	}, logger)
	if handled || err != nil {
		return rsp, err
	}
	fmt.Println(time.Now().String()+" ===================>  grpc login called by Login with bearer token 4 = ", req.BearerToken)

	// This is also validated by the TokenWriter, but doing it early saves any
	// work done by the validator (e.g. roundtrip to the Kubernetes API server).
	if err := s.requireLocalTokens(logger); err != nil {
		return nil, err
	}
	fmt.Println(time.Now().String()+" ===================>  grpc login called by Login with bearer token 5 = ", req.BearerToken)

	authMethod, validator, err := s.LoadAuthMethod(req.AuthMethod, &entMeta)
	switch {
	case errors.Is(err, acl.ErrNotFound):
		return nil, status.Errorf(codes.InvalidArgument, "auth method %q not found", req.AuthMethod)
	case err != nil:
		logger.Error("failed to load auth method", "error", err.Error())
		return nil, status.Error(codes.Internal, "failed to load auth method")
	}
	fmt.Println(time.Now().String()+" ===================>  grpc login called by Login with bearer token 6 = ", req.BearerToken)

	verifiedIdentity, err := validator.ValidateLogin(ctx, req.BearerToken)
	if err != nil {
		// TODO(agentless): errors returned from validators aren't standardized so
		// it's hard to tell whether validation failed because of an invalid bearer
		// token or something internal/transient. We currently return Unauthenticated
		// for all errors because it's the most likely, but we should make validators
		// return a typed or sentinel error instead.
		logger.Error("failed to validate login", "error", err.Error())
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	fmt.Println(time.Now().String()+" ===================>  grpc login called by Login with bearer token 7 = ", req.BearerToken)

	description, err := auth.BuildTokenDescription("token created via login", req.Meta)
	if err != nil {
		logger.Error("failed to build token description", "error", err.Error())
		return nil, status.Error(codes.Internal, err.Error())
	}
	fmt.Println(time.Now().String()+" ===================>  grpc login called by Login with bearer token 8 = ", req.BearerToken)

	token, err := s.NewLogin().TokenForVerifiedIdentity(verifiedIdentity, authMethod, description)
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		logger.Error("failed to create token", "error", err.Error())
		return nil, status.Error(codes.Internal, "failed to create token")
	}
	fmt.Println(time.Now().String()+" ===================>  grpc login called by Login with bearer token 9 = ", req.BearerToken, "token", token)

	return &pbacl.LoginResponse{
		Token: &pbacl.LoginToken{
			AccessorId: token.AccessorID,
			SecretId:   token.SecretID,
		},
	}, nil
}
