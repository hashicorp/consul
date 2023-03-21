package resource

import (
	"fmt"

	"github.com/hashicorp/consul/internal/resource"
	storage "github.com/hashicorp/consul/internal/storage"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func watchListOpFrom(eventOp pbresource.WatchEvent_Operation) pbresource.WatchListResponse_Operation {
	switch eventOp {
	case pbresource.WatchEvent_OPERATION_UPSERT:
		return pbresource.WatchListResponse_OPERATION_UPSERT
	case pbresource.WatchEvent_OPERATION_DELETE:
		return pbresource.WatchListResponse_OPERATION_DELETE
	default:
		panic(fmt.Sprintf("unhandled op %s", eventOp.String()))
	}
}

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

		if err = stream.Send(&pbresource.WatchListResponse{
			Operation: watchListOpFrom(event.Operation),
			Resource:  event.Resource,
		}); err != nil {
			return err
		}
	}

}
