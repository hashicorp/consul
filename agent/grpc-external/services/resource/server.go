// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"errors"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type Server struct {
	Config
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

func (s *Server) Register(registrar grpc.ServiceRegistrar) {
	pbresource.RegisterResourceServiceServer(registrar, s)
}

// Get token from grpc metadata or AnonymousTokenId if not found
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
	if id.Type == nil {
		return status.Errorf(codes.InvalidArgument, "%s.type is required", errorPrefix)
	}

	if err := resource.ValidateName(id.Name); err != nil {
		return status.Errorf(codes.InvalidArgument, "%s.name invalid: %v", errorPrefix, err)
	}

	// Better UX: Allow callers to pass in nil tenancy.  Defaulting and inheritance of tenancy
	// from the request token will take place further down in the call flow.
	if id.Tenancy == nil {
		id.Tenancy = &pbresource.Tenancy{
			Partition: "",
			Namespace: "",
		}
	}

	if id.Tenancy.Partition != "" {
		if err := resource.ValidateName(id.Tenancy.Partition); err != nil {
			return status.Errorf(codes.InvalidArgument, "%s.tenancy.partition invalid: %v", errorPrefix, err)
		}
	}
	if id.Tenancy.Namespace != "" {
		if err := resource.ValidateName(id.Tenancy.Namespace); err != nil {
			return status.Errorf(codes.InvalidArgument, "%s.tenancy.namespace invalid: %v", errorPrefix, err)
		}
	}

	return nil
}

func validateRef(ref *pbresource.Reference, errorPrefix string) error {
	if ref.Type == nil {
		return status.Errorf(codes.InvalidArgument, "%s.type is required", errorPrefix)
	}
	if err := resource.ValidateName(ref.Name); err != nil {
		return status.Errorf(codes.InvalidArgument, "%s.name invalid: %v", errorPrefix, err)
	}
	if err := resource.ValidateName(ref.Tenancy.Partition); err != nil {
		return status.Errorf(codes.InvalidArgument, "%s.tenancy.partition invalid: %v", errorPrefix, err)
	}
	if err := resource.ValidateName(ref.Tenancy.Namespace); err != nil {
		return status.Errorf(codes.InvalidArgument, "%s.tenancy.namespace invalid: %v", errorPrefix, err)
	}
	return nil
}

func validateWildcardTenancy(tenancy *pbresource.Tenancy, namePrefix string) error {
	// Partition has to be a valid name if not wildcard or empty
	if tenancy.Partition != "" && tenancy.Partition != "*" {
		if err := resource.ValidateName(tenancy.Partition); err != nil {
			return status.Errorf(codes.InvalidArgument, "tenancy.partition invalid: %v", err)
		}
	}

	// Namespace has to be a valid name if not wildcard or empty
	if tenancy.Namespace != "" && tenancy.Namespace != "*" {
		if err := resource.ValidateName(tenancy.Namespace); err != nil {
			return status.Errorf(codes.InvalidArgument, "tenancy.namespace invalid: %v", err)
		}
	}

	// Not doing a strict resource name validation here because the prefix can be
	// something like "foo-" which is a valid prefix but not valid resource name.
	// relax validation to just check for lowercasing
	if namePrefix != strings.ToLower(namePrefix) {
		return status.Errorf(codes.InvalidArgument, "name_prefix invalid: must be lowercase alphanumeric, got: %v", namePrefix)
	}

	return nil
}

// tenancyExists return an error with the passed in gRPC status code when tenancy partition or namespace do not exist.
func tenancyExists(reg *resource.Registration, tenancyBridge TenancyBridge, tenancy *pbresource.Tenancy, errCode codes.Code) error {
	if reg.Scope == resource.ScopePartition || reg.Scope == resource.ScopeNamespace {
		exists, err := tenancyBridge.PartitionExists(tenancy.Partition)
		switch {
		case err != nil:
			return err
		case !exists:
			return status.Errorf(errCode, "partition not found: %v", tenancy.Partition)
		}
	}

	if reg.Scope == resource.ScopeNamespace {
		exists, err := tenancyBridge.NamespaceExists(tenancy.Partition, tenancy.Namespace)
		switch {
		case err != nil:
			return err
		case !exists:
			return status.Errorf(errCode, "namespace not found: %v", tenancy.Namespace)
		}
	}
	return nil
}

func validateScopedTenancy(scope resource.Scope, resourceType *pbresource.Type, tenancy *pbresource.Tenancy, allowWildcards bool) error {
	if scope == resource.ScopePartition && tenancy.Namespace != "" && (!allowWildcards || tenancy.Namespace != storage.Wildcard) {
		return status.Errorf(
			codes.InvalidArgument,
			"partition scoped resource %s cannot have a namespace. got: %s",
			resource.ToGVK(resourceType),
			tenancy.Namespace,
		)
	}

	if scope == resource.ScopeCluster {
		if tenancy.Partition != "" && (!allowWildcards || tenancy.Partition != storage.Wildcard) {
			return status.Errorf(
				codes.InvalidArgument,
				"cluster scoped resource %s cannot have a partition: %s",
				resource.ToGVK(resourceType),
				tenancy.Partition,
			)
		}
		if tenancy.Namespace != "" && (!allowWildcards || tenancy.Namespace != storage.Wildcard) {
			return status.Errorf(
				codes.InvalidArgument,
				"cluster scoped resource %s cannot have a namespace: %s",
				resource.ToGVK(resourceType),
				tenancy.Namespace,
			)
		}
	}
	return nil
}

func isTenancyMarkedForDeletion(reg *resource.Registration, tenancyBridge TenancyBridge, tenancy *pbresource.Tenancy) (bool, error) {
	if reg.Scope == resource.ScopePartition || reg.Scope == resource.ScopeNamespace {
		marked, err := tenancyBridge.IsPartitionMarkedForDeletion(tenancy.Partition)
		if err != nil {
			return false, err
		}
		if marked {
			return marked, nil
		}
	}

	if reg.Scope == resource.ScopeNamespace {
		marked, err := tenancyBridge.IsNamespaceMarkedForDeletion(tenancy.Partition, tenancy.Namespace)
		if err != nil {
			return false, err
		}
		return marked, nil
	}

	// Cluster scope has no tenancy so always return false
	return false, nil
}

// retryCAS retries the given operation with exponential backoff if the user
// didn't provide a version. This is intended to hide failures when the user
// isn't intentionally performing a CAS operation (all writes are, by design,
// CAS operations at the storage backend layer).
func (s *Server) retryCAS(ctx context.Context, vsn string, cas func() error) error {
	if vsn != "" {
		return cas()
	}

	const maxAttempts = 5

	// These parameters are fairly arbitrary, so if you find better ones then go
	// ahead and swap them out! In general, we want to wait long enough to smooth
	// over small amounts of storage replication lag, but not so long that we make
	// matters worse by holding onto load.
	backoff := &retry.Waiter{
		MinWait: 50 * time.Millisecond,
		MaxWait: 1 * time.Second,
		Jitter:  retry.NewJitter(50),
		Factor:  75 * time.Millisecond,
	}

	var err error
	for i := 1; i <= maxAttempts; i++ {
		if err = cas(); !errors.Is(err, storage.ErrCASFailure) {
			break
		}
		if backoff.Wait(ctx) != nil {
			break
		}
		s.Logger.Trace("retrying failed CAS operation", "failure_count", i)
	}
	return err
}

func clone[T proto.Message](v T) T { return proto.Clone(v).(T) }
