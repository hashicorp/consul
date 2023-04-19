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

// Deletes a resource.
// - To delete a resource regardless of the stored version, set Version = ""
// - Supports deleting a resource by name, hence Id.Uid may be empty.
// - Delete of a previously deleted or non-existent resource is a no-op to support idempotency.
// - Errors with Aborted if the requested Version does not match the stored Version.
// - Errors with PermissionDenied if ACL check fails
//
// TODO(spatel): Move docs to the proto file
func (s *Server) Delete(ctx context.Context, req *pbresource.DeleteRequest) (*pbresource.DeleteResponse, error) {
	if err := validateDeleteRequest(req); err != nil {
		return nil, err
	}

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

	// The storage backend requires a Version and Uid to delete a resource based
	// on CAS semantics. When either are not provided, the resource must be read
	// with a strongly consistent read to retrieve either or both.
	//
	// n.b.: There is a chance DeleteCAS may fail with a storage.ErrCASFailure
	// if an update occurs between the Read and DeleteCAS. Consider refactoring
	// to use retryCAS() similar to the Write endpoint to close this gap.
	deleteVersion := req.Version
	deleteId := req.Id
	if deleteVersion == "" || deleteId.Uid == "" {
		existing, err := s.Backend.Read(ctx, storage.StrongConsistency, req.Id)
		switch {
		case err == nil:
			deleteVersion = existing.Version
			deleteId = existing.Id
		case errors.Is(err, storage.ErrNotFound):
			// Deletes are idempotent so no-op when not found
			return &pbresource.DeleteResponse{}, nil
		default:
			return nil, status.Errorf(codes.Internal, "failed read: %v", err)
		}
	}

	err = s.Backend.DeleteCAS(ctx, deleteId, deleteVersion)
	switch {
	case err == nil:
		return &pbresource.DeleteResponse{}, nil
	case errors.Is(err, storage.ErrCASFailure):
		return nil, status.Error(codes.Aborted, err.Error())
	default:
		return nil, status.Errorf(codes.Internal, "failed delete: %v", err)
	}
}

func validateDeleteRequest(req *pbresource.DeleteRequest) error {
	if req.Id == nil {
		return status.Errorf(codes.InvalidArgument, "id is required")
	}

	if err := validateId(req.Id, "id"); err != nil {
		return err
	}
	return nil
}
