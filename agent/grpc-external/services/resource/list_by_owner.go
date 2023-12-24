// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) ListByOwner(ctx context.Context, req *pbresource.ListByOwnerRequest) (*pbresource.ListByOwnerResponse, error) {
	reg, err := s.ensureListByOwnerRequestValid(req)
	if err != nil {
		return nil, err
	}

	// Convert v2 request tenancy to v1 for ACL subsystem.
	entMeta := v2TenancyToV1EntMeta(req.Owner.Tenancy)
	token := tokenFromContext(ctx)

	// Fill entMeta with token tenancy when empty.
	authz, authzContext, err := s.getAuthorizer(token, entMeta)
	if err != nil {
		return nil, err
	}

	// Handle defaulting empty tenancy units from request.
	v1EntMetaToV2Tenancy(reg, entMeta, req.Owner.Tenancy)

	// Check list ACL before verifying tenancy exists to not leak tenancy existence.
	err = reg.ACLs.List(authz, authzContext)
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed list acl: %v", err)
	}

	// Get owned resources.
	children, err := s.Backend.ListByOwner(ctx, req.Owner)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed list by owner: %v", err)
	}

	result := make([]*pbresource.Resource, 0)
	for _, child := range children {
		// Retrieve child type's registration to access read ACL hook.
		childReg, err := s.resolveType(child.Id.Type)
		if err != nil {
			return nil, err
		}

		// Rebuild authorizer if tenancy not identical between owner and child (child scope
		// may be narrower).
		childAuthz := authz
		childAuthzContext := authzContext
		if !resource.EqualTenancy(req.Owner.Tenancy, child.Id.Tenancy) {
			childEntMeta := v2TenancyToV1EntMeta(child.Id.Tenancy)
			childAuthz, childAuthzContext, err = s.getAuthorizer(token, childEntMeta)
			if err != nil {
				return nil, err
			}
		}

		// Filter out children that fail real ACL.
		err = childReg.ACLs.Read(childAuthz, childAuthzContext, child.Id, child)
		switch {
		case acl.IsErrPermissionDenied(err):
			continue
		case err != nil:
			return nil, status.Errorf(codes.Internal, "failed read acl: %v", err)
		}

		result = append(result, child)
	}
	return &pbresource.ListByOwnerResponse{Resources: result}, nil
}

func (s *Server) ensureListByOwnerRequestValid(req *pbresource.ListByOwnerRequest) (*resource.Registration, error) {
	if req.Owner == nil {
		return nil, status.Errorf(codes.InvalidArgument, "owner is required")
	}

	if err := validateId(req.Owner, "owner"); err != nil {
		return nil, err
	}

	if req.Owner.Uid == "" {
		return nil, status.Errorf(codes.InvalidArgument, "owner uid is required")
	}

	reg, err := s.resolveType(req.Owner.Type)
	if err != nil {
		return nil, err
	}

	if err = checkV2Tenancy(s.UseV2Tenancy, req.Owner.Type); err != nil {
		return nil, err
	}

	if err = validateScopedTenancy(reg.Scope, reg.Type, req.Owner.Tenancy, true); err != nil {
		return nil, err
	}

	return reg, nil
}
