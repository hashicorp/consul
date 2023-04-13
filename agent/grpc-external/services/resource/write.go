package resource

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// errUseWriteStatus is returned when the user attempts to modify the resource
// status using the Write endpoint.
//
// We only allow modifications to the status using the WriteStatus endpoint
// because:
//
//   - Setting statuses should only be done by controllers and requires different
//     permissions.
//
//   - Status-only updates shouldn't increment the resource generation.
//
// While we could accomplish both in the Write handler, there's seldom need to
// update the resource body and status at the same time, so it makes more sense
// to keep them separate.
var errUseWriteStatus = status.Error(codes.InvalidArgument, "resource.status can only be set using the WriteStatus endpoint")

func (s *Server) Write(ctx context.Context, req *pbresource.WriteRequest) (*pbresource.WriteResponse, error) {
	if err := validateWriteRequest(req); err != nil {
		return nil, err
	}

	reg, err := s.resolveType(req.Resource.Id.Type)
	if err != nil {
		return nil, err
	}

	authz, err := s.getAuthorizer(tokenFromContext(ctx))
	if err != nil {
		return nil, err
	}

	// check acls
	err = reg.ACLs.Write(authz, req.Resource.Id)
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed write acl: %v", err)
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

	if err = reg.Validate(req.Resource); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err = reg.Mutate(req.Resource); err != nil {
		return nil, status.Errorf(codes.Internal, "failed mutate hook: %v", err.Error())
	}

	// At the storage backend layer, all writes are CAS operations.
	//
	// This makes it possible to *safely* do things like keeping the Uid stable
	// across writes, carrying statuses over, and passing the current version of
	// the resource to hooks, without restricting ourselves to only using the more
	// feature-rich storage systems that support "patch" updates etc. natively.
	//
	// Although CAS semantics are useful for machine users like controllers, human
	// users generally don't need them. If the user is performing a non-CAS write,
	// we read the current version, and automatically retry if the CAS write fails.
	var result *pbresource.Resource
	err = s.retryCAS(ctx, req.Resource.Version, func() error {
		input := clone(req.Resource)

		// We read with EventualConsistency here because:
		//
		//	- In the common case, individual resources are written infrequently, and
		//	  when using the Raft backend followers are generally within a few hundred
		//	  milliseconds of the leader, so the first read will probably return the
		//	  current version.
		//
		//	- StrongConsistency is expensive. In the Raft backend, it involves a round
		//	  of heartbeats to verify cluster leadership (in addition to the write's
		//	  log replication).
		//
		//	- CAS failures will be retried by retryCAS anyway. So the read-modify-write
		//	  cycle should eventually succeed.
		existing, err := s.Backend.Read(ctx, storage.EventualConsistency, input.Id)
		switch {
		// Create path.
		case errors.Is(err, storage.ErrNotFound):
			input.Id.Uid = ulid.Make().String()

			// Enforce same tenancy for owner
			if input.Owner != nil && !proto.Equal(input.Id.Tenancy, input.Owner.Tenancy) {
				return status.Errorf(codes.InvalidArgument, "owner and resource tenancy must be the same")
			}

			// TODO: Prevent setting statuses in this endpoint.
			if len(input.Status) != 0 {
				return errUseWriteStatus
			}

		// Update path.
		case err == nil:
			// Use the stored ID because it includes the Uid.
			//
			// Generally, users won't provide the Uid but controllers will, because
			// controllers need to operate on a specific "incarnation" of a resource
			// as opposed to an older/newer resource with the same name, whereas users
			// just want to update the current resource.
			input.Id = existing.Id

			// User is doing a non-CAS write, use the current version.
			if input.Version == "" {
				input.Version = existing.Version
			}

			// Check the stored version matches the user-given version.
			//
			// Although CAS operations are implemented "for real" at the storage backend
			// layer, we must check the version here too to prevent a scenario where:
			//
			//	- Current resource version is `v2`
			//	- User passes version `v2`
			//	- Read returns stale version `v1`
			//	- We carry `v1`'s statuses over (effectively overwriting `v2`'s statuses)
			//	- CAS operation succeeds anyway because user-given version is current
			if input.Version != existing.Version {
				return storage.ErrCASFailure
			}

			// Owner can only be set on creation. Enforce immutability.
			if !proto.Equal(input.Owner, existing.Owner) {
				return status.Errorf(codes.InvalidArgument, "owner cannot be changed")
			}

			if input.Status == nil {
				input.Status = existing.Status
			} else if !resource.EqualStatus(input.Status, existing.Status) {
				return errUseWriteStatus
			}

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
	case isGRPCStatusError(err):
		return nil, err
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

func validateWriteRequest(req *pbresource.WriteRequest) error {
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
