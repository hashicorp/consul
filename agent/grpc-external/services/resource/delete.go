package resource

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) Delete(ctx context.Context, req *pbresource.DeleteRequest) (*pbresource.DeleteResponse, error) {
	// check type registered
	if err := s.resolveType(req.Id.Type); err != nil {
		return nil, err
	}

	err := s.backend.DeleteCAS(ctx, req.Id, req.Version)
	if err != nil {
		if errors.Is(err, storage.ErrCASFailure) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, err
	}
	return &pbresource.DeleteResponse{}, nil
}
