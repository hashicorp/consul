// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Server struct {
	Config
}

type Config struct {
	Logger   hclog.Logger
	Registry Registry

	// Backend is the storage backend that will be used for resource persistence.
	Backend     Backend
	ACLResolver ACLResolver
}

//go:generate mockery --name Registry --inpackage
type Registry interface {
	resource.Registry
}

//go:generate mockery --name Backend --inpackage
type Backend interface {
	storage.Backend
}

//go:generate mockery --name ACLResolver --inpackage
type ACLResolver interface {
	ResolveTokenAndDefaultMeta(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) (resolver.Result, error)
}

func NewServer(cfg Config) *Server {
	return &Server{cfg}
}

var _ pbresource.ResourceServiceServer = (*Server)(nil)

func (s *Server) Register(grpcServer *grpc.Server) {
	pbresource.RegisterResourceServiceServer(grpcServer, s)
}

// Get token from grpc metadata or AnonymounsTokenId if not found
func tokenFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return acl.AnonymousTokenID
	}

	vals := md.Get("x-consul-token")
	if len(vals) == 0 {
		return acl.AnonymousTokenID
	}
	return vals[0]
}

func (s *Server) resolveType(typ *pbresource.Type) (*resource.Registration, error) {
	v, ok := s.Registry.Resolve(typ)
	if ok {
		return &v, nil
	}
	return nil, status.Errorf(
		codes.InvalidArgument,
		"resource type %s not registered", resource.ToGVK(typ),
	)
}

func readConsistencyFrom(ctx context.Context) storage.ReadConsistency {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return storage.EventualConsistency
	}

	vals := md.Get("x-consul-consistency-mode")
	if len(vals) == 0 {
		return storage.EventualConsistency
	}

	if vals[0] == "consistent" {
		return storage.StrongConsistency
	}
	return storage.EventualConsistency
}

func (s *Server) getAuthorizer(token string) (acl.Authorizer, error) {
	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(token, nil, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed getting authorizer: %v", err)
	}
	return authz, nil
}

func isGRPCStatusError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := status.FromError(err)
	return ok
}

func validateId(id *pbresource.ID, errorPrefix string) error {
	var field string
	switch {
	case id.Type == nil:
		field = "type"
	case id.Tenancy == nil:
		field = "tenancy"
	case id.Name == "":
		field = "name"
	}

	if field != "" {
		return status.Errorf(codes.InvalidArgument, "%s.%s is required", errorPrefix, field)
	}

	// Revisit defaulting and non-namespaced resources post-1.16
	var expected string
	switch {
	case id.Tenancy.Partition != "default":
		field, expected = "partition", "default"
	case id.Tenancy.Namespace != "default":
		field, expected = "namespace", "default"
	case id.Tenancy.PeerName != "local":
		field, expected = "peername", "local"
	}

	if field != "" {
		return status.Errorf(codes.InvalidArgument, "%s.tenancy.%s must be %s", errorPrefix, field, expected)
	}
	return nil
}

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }
