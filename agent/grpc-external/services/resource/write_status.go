// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"errors"
	"fmt"

	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) WriteStatus(ctx context.Context, req *pbresource.WriteStatusRequest) (*pbresource.WriteStatusResponse, error) {
	reg, err := s.validateWriteStatusRequest(req)
	if err != nil {
		return nil, err
	}

	entMeta := v2TenancyToV1EntMeta(req.Id.Tenancy)
	authz, authzContext, err := s.getAuthorizer(tokenFromContext(ctx), entMeta)
	if err != nil {
		return nil, err
	}

	// Apply defaults when tenancy units empty.
	v1EntMetaToV2Tenancy(reg, entMeta, req.Id.Tenancy)

	// Check tenancy exists for the V2 resource. Ignore "marked for deletion" since status updates
	// should still work regardless.
	if err = tenancyExists(reg, s.TenancyBridge, req.Id.Tenancy, codes.InvalidArgument); err != nil {
		return nil, err
	}

	// Retrieve resource since ACL hook requires it.
	existing, err := s.Backend.Read(ctx, storage.EventualConsistency, req.Id)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		return nil, status.Errorf(codes.NotFound, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed read: %v", err)
	}

	// Check write ACL.
	err = reg.ACLs.Write(authz, authzContext, existing)
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed operator:write allowed acl: %v", err)
	}

	// At the storage backend layer, all writes are CAS operations.
	//
	// See comment in write.go for more information.
	//
	// Most controllers *won't* do an explicit CAS write of the status because it
	// doesn't provide much value, and conflicts are fairly likely in the flurry
	// of activity after a resource is updated.
	//
	// Here's why that's okay:
	//
	//	- Controllers should only update their own status (identified by its key)
	//	  and updating separate statuses is commutative.
	//
	//	- Controllers that make writes should be leader-elected singletons (i.e.
	//	  there should only be one instance of the controller running) so we don't
	//	  need to worry about multiple instances racing with each other.
	//
	//	- Only controllers are supposed to write statuses, so you should never be
	//	  racing with a user's write of the same status.
	var result *pbresource.Resource
	err = s.retryCAS(ctx, req.Version, func() error {
		resource, err := s.Backend.Read(ctx, storage.EventualConsistency, req.Id)
		if err != nil {
			return err
		}

		if req.Version != "" && req.Version != resource.Version {
			return storage.ErrCASFailure
		}

		resource = clone(resource)
		if resource.Status == nil {
			resource.Status = make(map[string]*pbresource.Status)
		}

		status := clone(req.Status)
		status.UpdatedAt = timestamppb.Now()
		resource.Status[req.Key] = status

		result, err = s.Backend.WriteCAS(ctx, resource)
		return err
	})

	switch {
	case errors.Is(err, storage.ErrNotFound):
		return nil, status.Error(codes.NotFound, err.Error())
	case errors.Is(err, storage.ErrCASFailure):
		return nil, status.Error(codes.Aborted, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed to write resource: %v", err.Error())
	}

	return &pbresource.WriteStatusResponse{Resource: result}, nil
}

func (s *Server) validateWriteStatusRequest(req *pbresource.WriteStatusRequest) (*resource.Registration, error) {
	var field string
	switch {
	case req.Id == nil:
		field = "id"
	case req.Id.Type == nil:
		field = "id.type"
	case req.Id.Name == "":
		field = "id.name"
	case req.Id.Uid == "":
		// We require Uid because only controllers should write statuses and
		// controllers should *always* refer to a specific incarnation of a
		// resource using its Uid.
		field = "id.uid"
	case req.Key == "":
		field = "key"
	case req.Status == nil:
		field = "status"
	case req.Status.ObservedGeneration == "":
		field = "status.observed_generation"
	default:
		for i, condition := range req.Status.Conditions {
			if condition.Type == "" {
				field = fmt.Sprintf("status.conditions[%d].type", i)
				break
			}

			if condition.Resource != nil {
				switch {
				case condition.Resource.Type == nil:
					field = fmt.Sprintf("status.conditions[%d].resource.type", i)
					break
				case condition.Resource.Tenancy == nil:
					field = fmt.Sprintf("status.conditions[%d].resource.tenancy", i)
					break
				case condition.Resource.Name == "":
					field = fmt.Sprintf("status.conditions[%d].resource.name", i)
					break
				}
			}
		}
	}
	if field != "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s is required", field)
	}

	if req.Status.UpdatedAt != nil {
		return nil, status.Error(codes.InvalidArgument, "status.updated_at is automatically set and cannot be provided")
	}

	if _, err := ulid.ParseStrict(req.Status.ObservedGeneration); err != nil {
		return nil, status.Error(codes.InvalidArgument, "status.observed_generation is not valid")
	}

	// Better UX: Allow callers to pass in nil tenancy.  Defaulting and inheritance of tenancy
	// from the request token will take place further down in the call flow.
	if req.Id.Tenancy == nil {
		req.Id.Tenancy = &pbresource.Tenancy{
			Partition: "",
			Namespace: "",
		}
	}

	if err := validateId(req.Id, "id"); err != nil {
		return nil, err
	}

	for i, condition := range req.Status.Conditions {
		if condition.Resource != nil {
			if err := validateRef(condition.Resource, fmt.Sprintf("status.conditions[%d].resource", i)); err != nil {
				return nil, err
			}
		}
	}

	// Check type exists.
	reg, err := s.resolveType(req.Id.Type)
	if err != nil {
		return nil, err
	}

	// Check scope.
	if reg.Scope == resource.ScopePartition && req.Id.Tenancy.Namespace != "" {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"partition scoped resource %s cannot have a namespace. got: %s",
			resource.ToGVK(req.Id.Type),
			req.Id.Tenancy.Namespace,
		)
	}

	return reg, nil
}
