// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) List(ctx context.Context, req *pbresource.ListRequest) (*pbresource.ListResponse, error) {
	reg, err := s.validateListRequest(req)
	if err != nil {
		return nil, err
	}

	// v1 ACL subsystem is "wildcard" aware so just pass on through.
	entMeta := v2TenancyToV1EntMeta(req.Tenancy)
	token := tokenFromContext(ctx)
	authz, authzContext, err := s.getAuthorizer(token, entMeta)
	if err != nil {
		return nil, err
	}

	// Check ACLs.
	err = reg.ACLs.List(authz, authzContext)
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed list acl: %v", err)
	}

	// Ensure we're defaulting correctly when request tenancy units are empty.
	v1EntMetaToV2Tenancy(reg, entMeta, req.Tenancy)

	resources, err := s.Backend.List(
		ctx,
		readConsistencyFrom(ctx),
		storage.UnversionedTypeFrom(req.Type),
		req.Tenancy,
		req.NamePrefix,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed list: %v", err)
	}

	result := make([]*pbresource.Resource, 0)
	for _, resource := range resources {
		// Filter out non-matching GroupVersion.
		if resource.Id.Type.GroupVersion != req.Type.GroupVersion {
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

		// Filter out items that don't pass read ACLs.
		err = reg.ACLs.Read(authz, authzContext, resource.Id)
		switch {
		case acl.IsErrPermissionDenied(err):
			continue
		case err != nil:
			return nil, status.Errorf(codes.Internal, "failed read acl: %v", err)
		}
		result = append(result, resource)
	}
	return &pbresource.ListResponse{Resources: result}, nil
}

func (s *Server) validateListRequest(req *pbresource.ListRequest) (*resource.Registration, error) {
	var field string
	switch {
	case req.Type == nil:
		field = "type"
	case req.Tenancy == nil:
		field = "tenancy"
	}

	if field != "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s is required", field)
	}

	// Check type exists.
	reg, err := s.resolveType(req.Type)
	if err != nil {
		return nil, err
	}

	// Lowercase
	resource.Normalize(req.Tenancy)

	// Error when partition scoped and namespace not empty.
	if reg.Scope == resource.ScopePartition && req.Tenancy.Namespace != "" {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"partition scoped type %s cannot have a namespace. got: %s",
			resource.ToGVK(req.Type),
			req.Tenancy.Namespace,
		)
	}

	return reg, nil
}
