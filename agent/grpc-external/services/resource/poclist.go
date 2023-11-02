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

	switch op := req.Request.(type) {
	case *pbresource.POCListRequest_FilterByType:
		requestType = op.FilterByType.GetType()
		requestTenancy = op.FilterByType.GetTenancy()
	case *pbresource.POCListRequest_FilterByNamePrefix:
		requestType = op.FilterByNamePrefix.GetType()
		requestTenancy = op.FilterByNamePrefix.GetTenancy()	
		name_prefix = op.FilterByNamePrefix.GetNamePrefix()
	case *pbresource.POCListRequest_FilterByOwner:
		resp, err := s.ListByOwner(ctx, &pbresource.ListByOwnerRequest{Owner: op.FilterByOwner.GetOwner()})
		if err != nil {
			return nil, err
		}
		result := &pbresource.POCListResponse{Resources: resp.Resources}
		return result, err
	default:
		fmt.Println("No matching list operations")
	}

	reg, err := s.validatePOCListRequest(requestType, requestTenancy)
	if err != nil {
		return nil, err
	}

	// v1 ACL subsystem is "wildcard" aware so just pass on through.
	entMeta := v2TenancyToV1EntMeta(requestTenancy)
	token := tokenFromContext(ctx)
	authz, authzContext, err := s.getAuthorizer(token, entMeta)
	if err != nil {
		return nil, err
	}

	// TBD: ACL checks for list by tenancy???
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

	// TBD: Defaults for list by tenancy???
	if reg != nil {
		// Ensure we're defaulting correctly when request tenancy units are empty.
		v1EntMetaToV2Tenancy(reg, entMeta, requestTenancy)
	}

	resources, err := s.Backend.POCList(
		ctx,
		readConsistencyFrom(ctx),
		storage.UnversionedTypeFrom(requestType),
		requestTenancy,
		name_prefix,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed list: %v", err)
	}

	result := make([]*pbresource.Resource, 0)
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

		// TBD: ACL checks list by tenancy???
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
	return &pbresource.POCListResponse{Resources: result}, nil
}

func (s *Server) validatePOCListRequest(requestType *pbresource.Type, requestTenancy *pbresource.Tenancy) (*resource.Registration, error) {
	// TBD: Validation rules???

	// var field string
	// switch {
	// case requestType == nil:
	// 	field = "type"
	// case requestTenancy == nil:
	// 	field = "tenancy"
	// }

	// if field != "" {
	// 	return nil, status.Errorf(codes.InvalidArgument, "%s is required", field)
	// }

	var reg *resource.Registration
	if requestType != nil{
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

	// TBD: V2 tenancy feature flag check for list by tenancy???

	// if err = checkV2Tenancy(s.UseV2Tenancy, requestType); err != nil {
	// 	return nil, err
	// }

	// TBD: Drop this???

	// if err := validateWildcardTenancy(requestTenancy, req.NamePrefix); err != nil {
	// 	return nil, err
	// }

	

	return reg, nil
}
