// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) WatchList(req *pbresource.WatchListRequest, stream pbresource.ResourceService_WatchListServer) error {
	reg, err := s.ensureWatchListRequestValid(req)
	if err != nil {
		return err
	}

	// v1 ACL subsystem is "wildcard" aware so just pass on through.
	entMeta := v2TenancyToV1EntMeta(req.Tenancy)
	token := tokenFromContext(stream.Context())
	authz, authzContext, err := s.getAuthorizer(token, entMeta)
	if err != nil {
		return err
	}

	// Check list ACL.
	err = reg.ACLs.List(authz, authzContext)
	switch {
	case acl.IsErrPermissionDenied(err):
		return status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return status.Errorf(codes.Internal, "failed list acl: %v", err)
	}

	// Ensure we're defaulting correctly when request tenancy units are empty.
	v1EntMetaToV2Tenancy(reg, entMeta, req.Tenancy)

	unversionedType := storage.UnversionedTypeFrom(req.Type)
	watch, err := s.Backend.WatchList(
		stream.Context(),
		unversionedType,
		req.Tenancy,
		req.NamePrefix,
	)
	if err != nil {
		return err
	}
	defer watch.Close()

	for {
		event, err := watch.Next(stream.Context())
		switch {
		case errors.Is(err, storage.ErrWatchClosed):
			return status.Error(codes.Aborted, "watch closed by the storage backend (possibly due to snapshot restoration)")
		case err != nil:
			return status.Errorf(codes.Internal, "failed next: %v", err)
		}

		// drop group versions that don't match
		if event.Resource.Id.Type.GroupVersion != req.Type.GroupVersion {
			continue
		}

		// Need to rebuild authorizer per resource since wildcard inputs may
		// result in different tenancies. Consider caching per tenancy if this
		// is deemed expensive.
		entMeta = v2TenancyToV1EntMeta(event.Resource.Id.Tenancy)
		authz, authzContext, err = s.getAuthorizer(token, entMeta)
		if err != nil {
			return err
		}

		// filter out items that don't pass read ACLs
		err = reg.ACLs.Read(authz, authzContext, event.Resource.Id, event.Resource)
		switch {
		case acl.IsErrPermissionDenied(err):
			continue
		case err != nil:
			return status.Errorf(codes.Internal, "failed read acl: %v", err)
		}

		if err = stream.Send(event); err != nil {
			return err
		}
	}
}

func (s *Server) ensureWatchListRequestValid(req *pbresource.WatchListRequest) (*resource.Registration, error) {
	if req.Type == nil {
		return nil, status.Errorf(codes.InvalidArgument, "type is required")
	}

	// Check type exists.
	reg, err := s.resolveType(req.Type)
	if err != nil {
		return nil, err
	}

	// if no tenancy is passed defaults to wildcard
	if req.Tenancy == nil {
		req.Tenancy = wildcardTenancyFor(reg.Scope)
	}

	if err = checkV2Tenancy(s.UseV2Tenancy, req.Type); err != nil {
		return nil, err
	}

	if err := validateWildcardTenancy(req.Tenancy, req.NamePrefix); err != nil {
		return nil, err
	}

	// Check scope
	if err = validateScopedTenancy(reg.Scope, req.Type, req.Tenancy, true); err != nil {
		return nil, err
	}

	return reg, nil
}

func wildcardTenancyFor(scope resource.Scope) *pbresource.Tenancy {
	var defaultTenancy *pbresource.Tenancy

	switch scope {
	case resource.ScopeCluster:
		defaultTenancy = &pbresource.Tenancy{}
	case resource.ScopePartition:
		defaultTenancy = &pbresource.Tenancy{
			Partition: storage.Wildcard,
		}
	default:
		defaultTenancy = &pbresource.Tenancy{
			Partition: storage.Wildcard,
			Namespace: storage.Wildcard,
		}
	}
	return defaultTenancy
}
