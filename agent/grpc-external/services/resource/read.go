// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) Read(ctx context.Context, req *pbresource.ReadRequest) (*pbresource.ReadResponse, error) {
	// Light first pass validation based on what user passed in and not much more.
	reg, err := s.validateReadRequest(req)
	if err != nil {
		return nil, err
	}

	// acl.EnterpriseMeta acl.AuthorizerContext follow rules for V1 resources since they integrate with the V1 acl subsystem.
	// pbresource.Tenacy follows rules for V2 resources and the Resource service.
	// Example:
	//
	//    An OSS namespace scoped resource:
	//      V1: EnterpriseMeta{}
	//      V2: Tenancy {Partition: "default", Namespace: "default"}
	//
	//   An ENT namespace scoped resource:
	//      V1: EnterpriseMeta{Partition: "default", Namespace: "default"}
	//      V2: Tenancy {Partition: "default", Namespace: "default"}
	//
	// It is necessary to convert back and forth depending on which component supports which version, V1 or V2.
	entMeta := v2TenancyToV1EntMeta(req.Id.Tenancy)
	authz, authzContext, err := s.getAuthorizer(tokenFromContext(ctx), entMeta)
	if err != nil {
		return nil, err
	}

	v1EntMetaToV2Tenancy(reg, entMeta, req.Id.Tenancy)

	// ACL check comes before tenancy existence checks to not leak tenancy "existence".
	err = reg.ACLs.Read(authz, authzContext, req.Id)
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed read acl: %v", err)
	}

	// Check V1 tenancy exists for the V2 resource.
	if err = v1TenancyExists(reg, s.V1TenancyBridge, req.Id.Tenancy, codes.NotFound); err != nil {
		return nil, err
	}

	resource, err := s.Backend.Read(ctx, readConsistencyFrom(ctx), req.Id)
	switch {
	case err == nil:
		return &pbresource.ReadResponse{Resource: resource}, nil
	case errors.Is(err, storage.ErrNotFound):
		return nil, status.Error(codes.NotFound, err.Error())
	case errors.As(err, &storage.GroupVersionMismatchError{}):
		return nil, status.Error(codes.InvalidArgument, err.Error())
	default:
		return nil, status.Errorf(codes.Internal, "failed read: %v", err)
	}
}

func (s *Server) validateReadRequest(req *pbresource.ReadRequest) (*resource.Registration, error) {
	if req.Id == nil {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}

	if err := validateId(req.Id, "id"); err != nil {
		return nil, err
	}

	// Check type exists.
	reg, err := s.resolveType(req.Id.Type)
	if err != nil {
		return nil, err
	}

	// Check scope
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
