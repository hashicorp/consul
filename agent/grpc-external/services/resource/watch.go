package resource

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) WatchList(req *pbresource.WatchListRequest, stream pbresource.ResourceService_WatchListServer) error {
	// check type exists
	_, ok := s.registry.Resolve(req.Type)
	if !ok {
		return status.Error(
			codes.InvalidArgument,
			fmt.Sprintf("resource type %s not registered", resource.ToGVK(req.Type)),
		)
	}

	unversionedType := storage.UnversionedTypeFrom(req.Type)
	watch, err := s.backend.WatchList(
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

		if err = stream.Send(&pbresource.WatchEvent{
			Operation: event.Operation,
			Resource:  event.Resource,
		}); err != nil {
			return err
		}
	}
}
