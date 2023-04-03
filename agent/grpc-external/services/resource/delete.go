package resource

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Deletes the resource with the given Id and Version.
// Pass an empty Version to delete a resource regardless of it's stored Version.
func (s *Server) Delete(ctx context.Context, req *pbresource.DeleteRequest) (*pbresource.DeleteResponse, error) {
	// check type registered
	reg, err := s.resolveType(req.Id.Type)
	if err != nil {
		return nil, err
	}

	// TODO(spatel): reg will be used for ACL hooks
	_ = reg

	if req.Version == "" {
		// Delete resource regardless of the persisted Version. Hence, strong read
		// necessary to get latest Version
		existing, err := s.backend.Read(ctx, storage.StrongConsistency, req.Id)
		if err != nil {
			return nil, err
		}
		if err = s.backend.DeleteCAS(ctx, existing.Id, existing.Version); err != nil {
			return nil, err
		}
		return &pbresource.DeleteResponse{}, nil
	}

	// non-empty Version so let backend enforce Versions match for a CAS delete
	if err = s.backend.DeleteCAS(ctx, req.Id, req.Version); err != nil {
		if errors.Is(err, storage.ErrCASFailure) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, err
	}
	return &pbresource.DeleteResponse{}, nil
}
