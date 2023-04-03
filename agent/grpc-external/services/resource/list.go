package resource

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) List(ctx context.Context, req *pbresource.ListRequest) (*pbresource.ListResponse, error) {
	// check type
	reg, err := s.resolveType(req.Type)
	if err != nil {
		return nil, err
	}

	// check list acls
	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(tokenFromContext(ctx), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("getting authorizer: %w", err)
	}
	if err = reg.ACLs.List(authz, req.Tenancy); err != nil {
		switch {
		case acl.IsErrPermissionDenied(err):
			return nil, status.Error(codes.PermissionDenied, err.Error())
		default:
			return nil, fmt.Errorf("authorizing list: %w", err)
		}
	}

	resources, err := s.Backend.List(ctx, readConsistencyFrom(ctx), storage.UnversionedTypeFrom(req.Type), req.Tenancy, req.NamePrefix)
	if err != nil {
		return nil, err
	}

	result := make([]*pbresource.Resource, 0)
	for _, resource := range resources {
		// filter out non-matching GroupVersion
		if resource.Id.Type.GroupVersion != req.Type.GroupVersion {
			continue
		}

		// filter out items that don't pass read ACLs
		if err = reg.ACLs.Read(authz, resource.Id); err != nil {
			switch {
			case acl.IsErrPermissionDenied(err):
				continue
			default:
				return nil, fmt.Errorf("authorizing read: %w", err)
			}
		}
		result = append(result, resource)
	}
	return &pbresource.ListResponse{Resources: result}, nil
}
