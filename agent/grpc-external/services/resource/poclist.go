// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) POCList(ctx context.Context, req *pbresource.POCListRequest) (*pbresource.POCListResponse, error) {
	var requestType *pbresource.Type
	var requestTenancy *pbresource.Tenancy
	var name_prefix string
	var reg *resource.Registration
	var err error

	// by type - query tenancy units can be wildcard (no tenancy is provided)
	// by tenancy - only namespace can be wildcard
	// by name prefix - no wildcards
	// by owner - no wildcards at all

	switch op := req.Request.(type) {
	case *pbresource.POCListRequest_FilterByType:
		requestType = op.FilterByType.GetType()
		name_prefix = ""
		requestTenancy = &pbresource.Tenancy{
			Namespace: "*",
			Partition: "*",
			PeerName:  "*",
		}
		reg, err = s.validatePOCListRequest(requestType, requestTenancy)
		if err != nil {
			return nil, err
		}
	case *pbresource.POCListRequest_FilterByTenancy:
		requestType = op.FilterByTenancy.GetType()
		requestTenancy = op.FilterByTenancy.GetTenancy()
		name_prefix = ""
		// TODO: default namespace to wilcard if not provided?
		// TODO: update validate to only allow wildcard for namespace
		reg, err = s.validatePOCListRequest(requestType, requestTenancy)
		if err != nil {
			return nil, err
		}
	case *pbresource.POCListRequest_FilterByNamePrefix:
		requestType = op.FilterByNamePrefix.GetType()
		requestTenancy = op.FilterByNamePrefix.GetTenancy()
		name_prefix = op.FilterByNamePrefix.GetNamePrefix()
		// TODO: update validate to block all wildcards
		reg, err = s.validatePOCListRequest(requestType, requestTenancy)
		if err != nil {
			return nil, err
		}
	case *pbresource.POCListRequest_FilterByOwner:
		ownerReq := &pbresource.ListByOwnerRequest{Owner: op.FilterByOwner.GetOwner()}
		reg, err = s.ensureListByOwnerRequestValid(ownerReq)
		if err != nil {
			return nil, err
		}

		requestType = ownerReq.Owner.Type
		requestTenancy = ownerReq.Owner.Tenancy
	default:
		fmt.Println("No matching list operations")
	}

	// v1 ACL subsystem is "wildcard" aware so just pass on through.
	entMeta := v2TenancyToV1EntMeta(requestTenancy)
	token := tokenFromContext(ctx)
	authz, authzContext, err := s.getAuthorizer(token, entMeta)
	if err != nil {
		return nil, err
	}

	if reg != nil {
		// Check ACLs.
		err = reg.ACLs.List(authz, authzContext)
		switch {
		case acl.IsErrPermissionDenied(err):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		case err != nil:
			return nil, status.Errorf(codes.Internal, "failed list acl: %v", err)
		}
	}

	if reg != nil {
		// Ensure we're defaulting correctly when request tenancy units are empty.
		v1EntMetaToV2Tenancy(reg, entMeta, requestTenancy)
	}

	var resources []*pbresource.Resource
	result := make([]*pbresource.Resource, 0)

	switch filter := req.Request.(type) {
	case *pbresource.POCListRequest_FilterByOwner:
		// Get owned resources.
		resources, err = s.Backend.ListByOwner(ctx, filter.FilterByOwner.Owner)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed list by owner: %v", err)
		}

		for _, child := range resources {
			// Retrieve child type's registration to access read ACL hook.
			childReg, err := s.resolveType(child.Id.Type)
			if err != nil {
				return nil, err
			}

			// Rebuild authorizer if tenancy not identical between owner and child (child scope
			// may be narrower).
			childAuthz := authz
			childAuthzContext := authzContext
			if !resource.EqualTenancy(filter.FilterByOwner.Owner.Tenancy, child.Id.Tenancy) {
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
	default:
		resources, err = s.Backend.List(
			ctx,
			readConsistencyFrom(ctx),
			storage.UnversionedTypeFrom(requestType),
			requestTenancy,
			name_prefix,
		)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed list: %v", err)
		}

		for _, resource := range resources {
			// Filter out non-matching GroupVersion.
			if requestType != nil && resource.Id.Type.GroupVersion != requestType.GroupVersion {
				continue
			}

			// Need to rebuild authorizer per resource since wildcard inputs may
			// result in different tenancies. Consider caching per tenancy if this
			// is deemed expensive.
			entMeta = v2TenancyToV1EntMeta(resource.Id.Tenancy)
			authz, authzContext, err = s.getAuthorizer(token, entMeta)
			if err != nil {
				return nil, err
			}

			if reg != nil {
				// Filter out items that don't pass read ACLs.
				err = reg.ACLs.Read(authz, authzContext, resource.Id, resource)
				switch {
				case acl.IsErrPermissionDenied(err):
					continue
				case err != nil:
					return nil, status.Errorf(codes.Internal, "failed read acl: %v", err)
				}
			}
			result = append(result, resource)
		}
	}
	return &pbresource.POCListResponse{Resources: result}, nil
}

func (s *Server) validatePOCListRequest(requestType *pbresource.Type, requestTenancy *pbresource.Tenancy) (*resource.Registration, error) {
	var field string
	switch {
	case requestType == nil:
		field = "type"
	case requestTenancy == nil:
		field = "tenancy"
	}

	if field != "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s is required", field)
	}

	var reg *resource.Registration
	if requestType != nil {
		// Check type exists.
		reg, err := s.resolveType(requestType)
		if err != nil {
			return nil, err
		}

		// Error when partition scoped and namespace not empty.
		if reg.Scope == resource.ScopePartition && requestTenancy.Namespace != "" {
			return nil, status.Errorf(
				codes.InvalidArgument,
				"partition scoped type %s cannot have a namespace. got: %s",
				resource.ToGVK(requestType),
				requestTenancy.Namespace,
			)
		}
	}

	if err := checkV2Tenancy(s.UseV2Tenancy, requestType); err != nil {
		return nil, err
	}

	return reg, nil
}
