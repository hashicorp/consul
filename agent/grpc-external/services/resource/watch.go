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
	reg, err := s.validateWatchListRequest(req)
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
		err = reg.ACLs.Read(authz, authzContext, event.Resource.Id)
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

func (s *Server) validateWatchListRequest(req *pbresource.WatchListRequest) (*resource.Registration, error) {
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
	if reg.Scope() == pbresource.Scope_PARTITION && req.Tenancy.Namespace != "" {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"partition scoped type %s cannot have a namespace. got: %s",
			resource.ToGVK(req.Type),
			req.Tenancy.Namespace,
		)
	}

	return reg, nil
}
