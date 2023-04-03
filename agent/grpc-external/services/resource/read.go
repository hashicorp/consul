package resource

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) Read(ctx context.Context, req *pbresource.ReadRequest) (*pbresource.ReadResponse, error) {
	// check type exists
	reg, err := s.resolveType(req.Id.Type)
	if err != nil {
		return nil, err
	}

	// check acls
	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(tokenFromContext(ctx), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("getting authorizer: %w", err)
	}
	if err = reg.ACLs.Read(authz, req.Id); err != nil {
		switch {
		case acl.IsErrPermissionDenied(err):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		default:
			return nil, fmt.Errorf("authorizing read: %w", err)
		}
	}

	resource, err := s.Backend.Read(ctx, readConsistencyFrom(ctx), req.Id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.As(err, &storage.GroupVersionMismatchError{}) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, err
	}
	return &pbresource.ReadResponse{Resource: resource}, nil
}
