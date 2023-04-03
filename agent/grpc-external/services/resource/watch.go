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

func (s *Server) WatchList(req *pbresource.WatchListRequest, stream pbresource.ResourceService_WatchListServer) error {
	// check type exists
	reg, err := s.resolveType(req.Type)
	if err != nil {
		return err
	}

	// check acls
	// TODO(spatel): FIXME Where do I get the ctx in order to extract the token?
	ctx := context.Background()
	authz, err := s.ACLResolver.ResolveTokenAndDefaultMeta(tokenFromContext(ctx), nil, nil)
	if err != nil {
		return fmt.Errorf("getting authorizer: %w", err)
	}
	if err = reg.ACLs.List(authz, req.Tenancy); err != nil {
		switch {
		case acl.IsErrPermissionDenied(err):
			return status.Error(codes.PermissionDenied, err.Error())
		default:
			return fmt.Errorf("authorizing list: %w", err)
		}
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

	for {
		event, err := watch.Next(stream.Context())
		if err != nil {
			return err
		}

		// drop versions that don't match
		if event.Resource.Id.Type.GroupVersion != req.Type.GroupVersion {
			continue
		}

		// filter out items that don't pass read ACLs
		if err = reg.ACLs.Read(authz, event.Resource.Id); err != nil {
			switch {
			case acl.IsErrPermissionDenied(err):
				continue
			default:
				return fmt.Errorf("authorizing read: %w", err)
			}
		}

		if err = stream.Send(event); err != nil {
			return err
		}
	}
}
