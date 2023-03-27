package resource

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) Write(ctx context.Context, req *pbresource.WriteRequest) (*pbresource.WriteResponse, error) {
	if err := validateWriteRequiredFields(req); err != nil {
		return nil, err
	}

	reg, err := s.resolveType(req.Resource.Id.Type)
	if err != nil {
		return nil, err
	}

	// Check the user sent the correct type of data.
	if !req.Resource.Data.MessageIs(reg.Proto) {
		got := strings.TrimPrefix(req.Resource.Data.TypeUrl, "type.googleapis.com/")

		return nil, status.Errorf(
			codes.InvalidArgument,
			"resource.data is of wrong type (expected=%q, got=%q)",
			reg.Proto.ProtoReflect().Descriptor().FullName(),
			got,
		)
	}

	var result *pbresource.Resource
	err = s.retryCAS(ctx, req.Resource.Version, func() error {
		input := clone(req.Resource)

		// Read with EventualConsistency because we'll already automatically retry on
		// CAS failure, so it's not worth the latency penalty of StrongConsistency.
		existing, err := s.Backend.Read(ctx, storage.EventualConsistency, input.Id)
		switch {
		case err == nil:
			if input.Id.Uid == "" {
				input.Id.Uid = existing.Id.Uid
			}

			if input.Version == "" {
				input.Version = existing.Version
			} else if input.Version != existing.Version {
				// Although the storage backend will check the version on-write, we check
				// it here too to make sure we don't carry over statuses from the wrong
				// version.
				return storage.ErrCASFailure
			}
			// TODO: Carry over the statuses here.

		case errors.Is(err, storage.ErrNotFound):
			input.Id.Uid = ulid.Make().String()
			// TODO: Prevent setting statuses in this endpoint.

		default:
			return err
		}

		input.Generation = ulid.Make().String()
		result, err = s.Backend.WriteCAS(ctx, input)
		return err
	})

	switch {
	case errors.Is(err, storage.ErrCASFailure):
		return nil, status.Error(codes.Aborted, err.Error())
	case errors.Is(err, storage.ErrWrongUid):
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed to write resource: %v", err.Error())
	}

	return &pbresource.WriteResponse{Resource: result}, nil
}

// retryCAS retries the given operation with exponential backoff if the user
// didn't provide a version. This is intended to hide failures when the user
// isn't intentionally performing a CAS operation (all writes are, by design,
// CAS operations at the storage backend layer).
func (s *Server) retryCAS(ctx context.Context, vsn string, cas func() error) error {
	if vsn != "" {
		return cas()
	}

	const maxAttempts = 5

	// These parameters are fairly arbitrary, so if you find better ones then go
	// ahead and swap them out! In general, we want to wait long enough to smooth
	// over small amounts of storage replication lag, but not so long that we make
	// matters worse by holding onto load.
	backoff := &retry.Waiter{
		MinWait: 50 * time.Millisecond,
		MaxWait: 1 * time.Second,
		Jitter:  retry.NewJitter(50),
		Factor:  75 * time.Millisecond,
	}

	var err error
	for i := 1; i <= maxAttempts; i++ {
		if err = cas(); !errors.Is(err, storage.ErrCASFailure) {
			break
		}
		if backoff.Wait(ctx) != nil {
			break
		}
		s.Logger.Trace("retrying failed CAS operation", "failure_count", i)
	}
	return err
}

func validateWriteRequiredFields(req *pbresource.WriteRequest) error {
	var field string
	switch {
	case req.Resource == nil:
		field = "resource"
	case req.Resource.Id == nil:
		field = "resource.id"
	case req.Resource.Id.Type == nil:
		field = "resource.id.type"
	case req.Resource.Id.Tenancy == nil:
		field = "resource.id.tenancy"
	case req.Resource.Id.Name == "":
		field = "resource.id.name"
	case req.Resource.Data == nil:
		field = "resource.data"
	}

	if field == "" {
		return nil
	}
	return status.Errorf(codes.InvalidArgument, "%s is required", field)
}
