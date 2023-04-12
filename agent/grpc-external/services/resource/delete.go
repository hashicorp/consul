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

// TODO(spatel): Move docs to the proto file
// Deletes a resource with the given Id and Version.
//
// Pass an empty Version to delete a resource regardless of the stored Version.
// Deletes of previously deleted or non-existent resource are no-ops.
// Returns an Aborted error if the requested Version does not match the stored Version.
func (s *Server) Delete(ctx context.Context, req *pbresource.DeleteRequest) (*pbresource.DeleteResponse, error) {
	reg, err := s.resolveType(req.Id.Type)
	if err != nil {
		return nil, err
	}

	authz, err := s.getAuthorizer(tokenFromContext(ctx))
	if err != nil {
		return nil, err
	}

	err = reg.ACLs.Write(authz, req.Id)
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed write acl: %v", err)
	}

	versionToDelete := req.Version
	if versionToDelete == "" {
		// Delete resource regardless of the stored Version. Hence, strong read
		// necessary to get latest Version
		existing, err := s.Backend.Read(ctx, storage.StrongConsistency, req.Id)
		switch {
		case err == nil:
			versionToDelete = existing.Version
		case errors.Is(err, storage.ErrNotFound):
			// deletes are idempotent so no-op if resource not found
			return &pbresource.DeleteResponse{}, nil
		default:
			return nil, status.Errorf(codes.Internal, "failed read: %v", err)
		}
	}

	err = s.Backend.DeleteCAS(ctx, req.Id, versionToDelete)
	switch {
	case err == nil:
		return &pbresource.DeleteResponse{}, nil
	case errors.Is(err, storage.ErrCASFailure):
		return nil, status.Error(codes.Aborted, err.Error())
	default:
		return nil, status.Errorf(codes.Internal, "failed delete: %v", err)
	}
}
