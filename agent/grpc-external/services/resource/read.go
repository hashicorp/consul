// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) Read(ctx context.Context, req *pbresource.ReadRequest) (*pbresource.ReadResponse, error) {
	if err := validateReadRequest(req); err != nil {
		return nil, err
	}

	// check type exists
	reg, err := s.resolveType(req.Id.Type)
	if err != nil {
		return nil, err
	}

	authz, err := s.getAuthorizer(tokenFromContext(ctx))
	if err != nil {
		return nil, err
	}

	// check acls
	err = reg.ACLs.Read(authz, req.Id)
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed read acl: %v", err)
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

func validateReadRequest(req *pbresource.ReadRequest) error {
	if req.Id == nil {
		return status.Errorf(codes.InvalidArgument, "id is required")
	}

	if err := validateId(req.Id, "id"); err != nil {
		return err
	}
	return nil
}
