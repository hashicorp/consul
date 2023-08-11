// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) WatchList(req *pbresource.WatchListRequest, stream pbresource.ResourceService_WatchListServer) error {
	if err := validateWatchListRequest(req); err != nil {
		return err
	}

	// check type exists
	reg, err := s.resolveType(req.Type)
	if err != nil {
		return err
	}

	// TODO(spatel): Refactor _ and entMeta as part of NET-4914
	authz, authzContext, err := s.getAuthorizer(tokenFromContext(stream.Context()), acl.DefaultEnterpriseMeta())
	if err != nil {
		return err
	}

	// check acls
	err = reg.ACLs.List(authz, req.Tenancy)
	switch {
	case acl.IsErrPermissionDenied(err):
		return status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return status.Errorf(codes.Internal, "failed list acl: %v", err)
	}

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

func validateWatchListRequest(req *pbresource.WatchListRequest) error {
	var field string
	switch {
	case req.Type == nil:
		field = "type"
	case req.Tenancy == nil:
		field = "tenancy"
	default:
		return nil
	}
	return status.Errorf(codes.InvalidArgument, "%s is required", field)
}
