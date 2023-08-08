// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/oklog/ulid/v2"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) WriteStatus(ctx context.Context, req *pbresource.WriteStatusRequest) (*pbresource.WriteStatusResponse, error) {
	// TODO(spatel): Refactor _ and entMeta as part of NET-4912
	authz, _, err := s.getAuthorizer(tokenFromContext(ctx), acl.DefaultEnterpriseMeta())
	if err != nil {
		return nil, err
	}

	// check acls
	err = authz.ToAllowAuthorizer().OperatorWriteAllowed(&acl.AuthorizerContext{})
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed operator:write allowed acl: %v", err)
	}

	if err := validateWriteStatusRequest(req); err != nil {
		return nil, err
	}

	_, err = s.resolveType(req.Id.Type)
	if err != nil {
		return nil, err
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

func validateWriteStatusRequest(req *pbresource.WriteStatusRequest) error {
	var field string
	switch {
	case req.Id == nil:
		field = "id"
	case req.Id.Type == nil:
		field = "id.type"
	case req.Id.Tenancy == nil:
		field = "id.tenancy"
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
		return status.Errorf(codes.InvalidArgument, "%s is required", field)
	}

	if req.Status.UpdatedAt != nil {
		return status.Error(codes.InvalidArgument, "status.updated_at is automatically set and cannot be provided")
	}

	if _, err := ulid.ParseStrict(req.Status.ObservedGeneration); err != nil {
		return status.Error(codes.InvalidArgument, "status.observed_generation is not valid")
	}

	return nil
}
