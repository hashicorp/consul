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

func (s *Server) ListByOwner(ctx context.Context, req *pbresource.ListByOwnerRequest) (*pbresource.ListByOwnerResponse, error) {
	if err := validateListByOwnerRequest(req); err != nil {
		return nil, err
	}

	_, err := s.resolveType(req.Owner.Type)
	if err != nil {
		return nil, err
	}

	childIds, err := s.Backend.OwnerReferences(ctx, req.Owner)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed getting owner references: %v", err)
	}

	authz, err := s.getAuthorizer(tokenFromContext(ctx))
	if err != nil {
		return nil, err
	}

	result := make([]*pbresource.Resource, 0)
	for _, childId := range childIds {
		reg, err := s.resolveType(childId.Type)
		if err != nil {
			return nil, err
		}

		// ACL filter
		err = reg.ACLs.Read(authz, childId)
		switch {
		case acl.IsErrPermissionDenied(err):
			continue
		case err != nil:
			return nil, status.Errorf(codes.Internal, "failed read acl: %v", err)
		}

		child, err := s.Backend.Read(ctx, readConsistencyFrom(ctx), childId)
		switch {
		case err == nil:
			result = append(result, child)
		case errors.Is(err, storage.ErrNotFound):
			continue
		default:
			return nil, status.Errorf(codes.Internal, "failed read: %v", err)
		}
	}
	return &pbresource.ListByOwnerResponse{Resources: result}, nil
}

func validateListByOwnerRequest(req *pbresource.ListByOwnerRequest) error {
	if req.Owner == nil {
		return status.Errorf(codes.InvalidArgument, "owner is required")
	}

	if err := validateId(req.Owner, "owner"); err != nil {
		return err
	}
	return nil
}
