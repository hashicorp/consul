// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	// V1TenancyBridge temporarily allows us to use V1 implementations of
	// partitions and namespaces until V2 implementations are available.
	V1TenancyBridge TenancyBridge
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

//go:generate mockery --name TenancyBridge --inpackage
type TenancyBridge interface {
	PartitionExists(partition string) (bool, error)
	IsPartitionMarkedForDeletion(partition string) (bool, error)
	NamespaceExists(partition, namespace string) (bool, error)
	IsNamespaceMarkedForDeletion(partition, namespace string) (bool, error)
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

func (s *Server) getAuthorizer(token string, entMeta *acl.EnterpriseMeta) (acl.Authorizer, *acl.AuthorizerContext, error) {
	authzContext := &acl.AuthorizerContext{}
	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(token, entMeta, authzContext)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "failed getting authorizer: %v", err)
	}
	return authz, authzContext, nil
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
	case id.Name == "":
		field = "name"
	}

	if field != "" {
		return status.Errorf(codes.InvalidArgument, "%s.%s is required", errorPrefix, field)
	}

	// Better UX: Allow callers to pass in nil tenancy.  Defaulting and inheritance of tenancy
	// from the request token will take place further down in the call flow.
	if id.Tenancy == nil {
		id.Tenancy = &pbresource.Tenancy{
			Partition: "",
			Namespace: "",
			// TODO(spatel): Remove when peerTenancy introduced.
			PeerName: "local",
		}
	}

	resource.Normalize(id.Tenancy)

	return nil
}

// v1TenancyExists return an error with the passed in gRPC status code when tenancy partition or namespace do not exist.
func v1TenancyExists(reg *resource.Registration, v1Bridge TenancyBridge, tenancy *pbresource.Tenancy, errCode codes.Code) error {
	if reg.Scope == resource.ScopePartition || reg.Scope == resource.ScopeNamespace {
		exists, err := v1Bridge.PartitionExists(tenancy.Partition)
		switch {
		case err != nil:
			return err
		case !exists:
			return status.Errorf(errCode, "partition resource not found: %v", tenancy.Partition)
		}
	}

	if reg.Scope == resource.ScopeNamespace {
		exists, err := v1Bridge.NamespaceExists(tenancy.Partition, tenancy.Namespace)
		switch {
		case err != nil:
			return err
		case !exists:
			return status.Errorf(errCode, "namespace resource not found: %v", tenancy.Namespace)
		}
	}
	return nil
}

// v1TenancyMarkedForDeletion returns a gRPC InvalidArgument when either partition or namespace is marked for deletion.
func v1TenancyMarkedForDeletion(reg *resource.Registration, v1Bridge TenancyBridge, tenancy *pbresource.Tenancy) error {
	if reg.Scope == resource.ScopePartition || reg.Scope == resource.ScopeNamespace {
		marked, err := v1Bridge.IsPartitionMarkedForDeletion(tenancy.Partition)
		switch {
		case err != nil:
			return err
		case marked:
			return status.Errorf(codes.InvalidArgument, "partition marked for deletion: %v", tenancy.Partition)
		}
	}

	if reg.Scope == resource.ScopeNamespace {
		marked, err := v1Bridge.IsNamespaceMarkedForDeletion(tenancy.Partition, tenancy.Namespace)
		switch {
		case err != nil:
			return err
		case marked:
			return status.Errorf(codes.InvalidArgument, "namespace marked for deletion: %v", tenancy.Namespace)
		}
	}
	return nil
}

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }
