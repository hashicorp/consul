// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package acl

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto-public/pbacl"
)

type Config struct {
	ACLsEnabled               bool
	Logger                    hclog.Logger
	LoadAuthMethod            func(authMethod string, entMeta *acl.EnterpriseMeta) (*structs.ACLAuthMethod, Validator, error)
	NewLogin                  func() Login
	ForwardRPC                func(structs.RPCInfo, func(*grpc.ClientConn) error) (bool, error)
	ValidateEnterpriseRequest func(*acl.EnterpriseMeta, bool) error
	LocalTokensEnabled        func() bool
	InPrimaryDatacenter       bool
	PrimaryDatacenter         string
	NewTokenWriter            func() TokenWriter
}

//go:generate mockery --name Login --inpackage
type Login interface {
	TokenForVerifiedIdentity(identity *authmethod.Identity, authMethod *structs.ACLAuthMethod, description string) (*structs.ACLToken, error)
}

//go:generate mockery --name Validator --inpackage
type Validator interface {
	ValidateLogin(ctx context.Context, loginToken string) (*authmethod.Identity, error)
}

//go:generate mockery --name TokenWriter --inpackage
type TokenWriter interface {
	Delete(secretID string, fromLogout bool) error
}

type Server struct {
	Config
}

func NewServer(cfg Config) *Server {
	return &Server{cfg}
}

func (s *Server) Register(grpcServer *grpc.Server) {
	pbacl.RegisterACLServiceServer(grpcServer, s)
}

func (s *Server) requireACLsEnabled(logger hclog.Logger) error {
	if s.ACLsEnabled {
		return nil
	}
	logger.Warn("request blocked ACLs are disabled")
	return status.Error(codes.FailedPrecondition, acl.ErrDisabled.Error())
}

func (s *Server) requireLocalTokens(logger hclog.Logger) error {
	if s.LocalTokensEnabled() {
		return nil
	}
	logger.Warn("request blocked because we're in a non-primary datacenter and token replication is disabled")
	return status.Error(codes.FailedPrecondition, "token replication is required for auth methods to function")
}

func (s *Server) forwardWriteDC(dc string, fn func(*grpc.ClientConn) error, logger hclog.Logger) (bool, error) {
	// For private/internal gRPC handlers, protoc-gen-rpc-glue generates the
	// requisite methods to satisfy the structs.RPCInfo interface using fields
	// from the pbcommon package. This service is public, so we can't use those
	// fields in our proto definition. Instead, we construct our RPCInfo manually.
	var rpcInfo struct {
		structs.WriteRequest      // Ensure RPCs are forwarded to the leader.
		structs.DCSpecificRequest // Ensure RPCs are forwarded to the correct datacenter.
	}
	rpcInfo.Datacenter = dc

	return s.ForwardRPC(&rpcInfo, func(conn *grpc.ClientConn) error {
		logger.Trace("forwarding RPC", "datacenter", dc)
		return fn(conn)
	})
}
