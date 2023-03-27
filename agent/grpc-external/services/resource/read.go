package resource

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) Read(ctx context.Context, req *pbresource.ReadRequest) (*pbresource.ReadResponse, error) {
	// check type exists
	_, ok := s.registry.Resolve(req.Id.Type)
	if !ok {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("resource type %s not registered", resource.ToGVK(req.Id.Type)))
	}

	consistency := storage.EventualConsistency
	if isConsistentRead(ctx) {
		consistency = storage.StrongConsistency
	}

	resource, err := s.backend.Read(ctx, consistency, req.Id)
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

func isConsistentRead(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}

	vals := md.Get("x-consul-consistency-mode")
	if len(vals) == 0 {
		return false
	}

	return vals[0] == "consistent"
}
