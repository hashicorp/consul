// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"errors"

	"github.com/oklog/ulid/v2"
	"golang.org/x/exp/maps"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
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
	tenancyMarkedForDeletion, err := s.mutateAndValidate(ctx, req.Resource, true)
	if err != nil {
		return nil, err
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
		var mismatchError storage.GroupVersionMismatchError
		existing, err := s.Backend.Read(ctx, storage.EventualConsistency, input.Id)
		switch {
		// Create path.
		case errors.Is(err, storage.ErrNotFound):
			input.Id.Uid = ulid.Make().String()

			// Prevent setting statuses in this endpoint.
			if len(input.Status) != 0 {
				return errUseWriteStatus
			}

			// Reject creation in tenancy unit marked for deletion.
			if tenancyMarkedForDeletion {
				return status.Errorf(codes.InvalidArgument, "tenancy marked for deletion: %v", input.Id.Tenancy.String())
			}

			// Reject attempts to create a resource with a deletionTimestamp.
			if resource.IsMarkedForDeletion(input) {
				return status.Errorf(codes.InvalidArgument, "resource.metadata.%s can't be set on resource creation", resource.DeletionTimestampKey)
			}

			// Generally, we expect resources with owners to be created by controllers,
			// and they should provide the Uid. In cases where no Uid is given (e.g. the
			// owner is specified in the resource HCL) we'll look up whatever the current
			// Uid is and use that.
			//
			// An important note on consistency:
			//
			// We read the owner with StrongConsistency here to reduce the likelihood of
			// creating a resource pointing to the wrong "incarnation" of the owner in
			// cases where the owner is deleted and re-created in quick succession.
			//
			// That said, there is still a chance that the owner has been deleted by the
			// time we write this resource. This is not a relational database and we do
			// not support ACID transactions or real foreign key constraints.
			if input.Owner != nil && input.Owner.Uid == "" {
				owner, err := s.Backend.Read(ctx, storage.StrongConsistency, input.Owner)
				switch {
				case errors.Is(err, storage.ErrNotFound):
					return status.Error(codes.InvalidArgument, "resource.owner does not exist")
				case err != nil:
					return status.Errorf(codes.Internal, "failed to resolve owner: %v", err)
				}
				input.Owner = owner.Id
			}

			// TODO(spatel): Revisit owner<->resource tenancy rules post-1.16

		// Update path.
		case err == nil || errors.As(err, &mismatchError):
			// Allow writes that update GroupVersion.
			if mismatchError.Stored != nil {
				existing = mismatchError.Stored
			}
			// Use the stored ID because it includes the Uid.
			//
			// Generally, users won't provide the Uid but controllers will, because
			// controllers need to operate on a specific "incarnation" of a resource
			// as opposed to an older/newer resource with the same name, whereas users
			// just want to update the current resource.
			input.Id = existing.Id

			// User is doing a non-CAS write, use the current version and preserve
			// deferred deletion metadata if not present.
			if input.Version == "" {
				input.Version = existing.Version
				preserveDeferredDeletionMetadata(input, existing)
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

			// Fill in an empty Owner UID with the existing owner's UID. If other parts
			// of the owner ID like the type or name have changed then the subsequent
			// EqualID call will still error as you are not allowed to change the owner.
			// This is a small UX nicety to repeatedly "apply" a resource that should
			// have an owner without having to care about the current owners incarnation.
			if input.Owner != nil && existing.Owner != nil && input.Owner.Uid == "" {
				input.Owner.Uid = existing.Owner.Uid
			}

			// Owner can only be set on creation. Enforce immutability.
			if !resource.EqualID(input.Owner, existing.Owner) {
				return status.Errorf(codes.InvalidArgument, "owner cannot be changed")
			}

			// Carry over status and prevent updates
			if input.Status == nil {
				input.Status = existing.Status
			} else if !resource.EqualStatusMap(input.Status, existing.Status) {
				return errUseWriteStatus
			}

			// If the write is related to a deferred deletion (marking for deletion or removal
			// of finalizers), make sure nothing else is changed.
			if err := vetIfDeleteRelated(input, existing, tenancyMarkedForDeletion); err != nil {
				return err
			}

			// Otherwise, let the write continue
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

func ensureMetadataSameExceptFor(input *pbresource.Resource, existing *pbresource.Resource, ignoreKey string) error {
	// Work on copies since we're mutating them
	inputCopy := maps.Clone(input.Metadata)
	existingCopy := maps.Clone(existing.Metadata)

	delete(inputCopy, ignoreKey)
	delete(existingCopy, ignoreKey)

	if !maps.Equal(inputCopy, existingCopy) {
		return status.Error(codes.InvalidArgument, "cannot modify metadata")
	}

	return nil
}

func ensureDataUnchanged(input *pbresource.Resource, existing *pbresource.Resource) error {
	// Check data last since this could potentially be the most expensive comparison.
	if !proto.Equal(input.Data, existing.Data) {
		return status.Error(codes.InvalidArgument, "cannot modify data")
	}
	return nil
}

// EnsureFinalizerRemoved ensures at least one finalizer was removed.
// TODO: only public for test to access
func EnsureFinalizerRemoved(input *pbresource.Resource, existing *pbresource.Resource) error {
	inputFinalizers := resource.GetFinalizers(input)
	existingFinalizers := resource.GetFinalizers(existing)
	if !inputFinalizers.IsProperSubset(existingFinalizers) {
		return status.Error(codes.InvalidArgument, "expected at least one finalizer to be removed")
	}
	return nil
}

func vetIfDeleteRelated(input, existing *pbresource.Resource, tenancyMarkedForDeletion bool) error {
	// Keep track of whether this write is a normal write or a write that is related
	// to deferred resource deletion involving setting the deletionTimestamp or the
	// removal of finalizers.
	deleteRelated := false

	existingMarked := resource.IsMarkedForDeletion(existing)
	inputMarked := resource.IsMarkedForDeletion(input)

	// Block removal of deletion timestamp
	if !inputMarked && existingMarked {
		return status.Errorf(codes.InvalidArgument, "cannot remove %s", resource.DeletionTimestampKey)
	}

	// Block modification of existing deletion timestamp
	if existing.Metadata[resource.DeletionTimestampKey] != "" && (existing.Metadata[resource.DeletionTimestampKey] != input.Metadata[resource.DeletionTimestampKey]) {
		return status.Errorf(codes.InvalidArgument, "cannot modify %s", resource.DeletionTimestampKey)
	}

	// Block writes that do more than just adding a deletion timestamp
	if inputMarked && !existingMarked {
		deleteRelated = deleteRelated || true
		// Verify rest of resource is unchanged
		if err := ensureMetadataSameExceptFor(input, existing, resource.DeletionTimestampKey); err != nil {
			return err
		}
		if err := ensureDataUnchanged(input, existing); err != nil {
			return err
		}
	}

	// Block no-op writes writes to resource that already has a deletion timestamp. The
	// only valid writes should be removal of finalizers.
	if inputMarked && existingMarked {
		deleteRelated = deleteRelated || true
		// Check if a no-op
		errMetadataSame := ensureMetadataSameExceptFor(input, existing, resource.DeletionTimestampKey)
		errDataUnchanged := ensureDataUnchanged(input, existing)
		if errMetadataSame == nil && errDataUnchanged == nil {
			return status.Error(codes.InvalidArgument, "cannot no-op write resource marked for deletion")
		}
	}

	// Block writes that do more than removing finalizers if previously marked for deletion.
	if inputMarked && existingMarked && resource.HasFinalizers(existing) {
		deleteRelated = deleteRelated || true
		if err := ensureMetadataSameExceptFor(input, existing, resource.FinalizerKey); err != nil {
			return err
		}
		if err := ensureDataUnchanged(input, existing); err != nil {
			return err
		}
		if err := EnsureFinalizerRemoved(input, existing); err != nil {
			return err
		}
	}

	// Classify writes that just remove finalizer as deleteRelated regardless of deletion state.
	if err := EnsureFinalizerRemoved(input, existing); err == nil {
		if err := ensureDataUnchanged(input, existing); err == nil {
			deleteRelated = deleteRelated || true
		}
	}

	// Lastly, block writes when the resource's tenancy unit has been marked for deletion and
	// the write is not related a valid delete scenario.
	if tenancyMarkedForDeletion && !deleteRelated {
		return status.Errorf(codes.InvalidArgument, "cannot write resource when tenancy marked for deletion: %s", existing.Id.Tenancy)
	}

	return nil
}

// preserveDeferredDeletionMetadata only applies to user writes (Version == "") which is a precondition.
func preserveDeferredDeletionMetadata(input, existing *pbresource.Resource) {
	// preserve existing deletionTimestamp if not provided in input
	if !resource.IsMarkedForDeletion(input) && resource.IsMarkedForDeletion(existing) {
		if input.Metadata == nil {
			input.Metadata = make(map[string]string)
		}
		input.Metadata[resource.DeletionTimestampKey] = existing.Metadata[resource.DeletionTimestampKey]
	}

	// Only preserve finalizers if the is key absent from input and present in existing.
	// If the key is present in input, the user clearly wants to remove finalizers!
	inputHasKey := false
	if input.Metadata != nil {
		_, inputHasKey = input.Metadata[resource.FinalizerKey]
	}

	existingHasKey := false
	if existing.Metadata != nil {
		_, existingHasKey = existing.Metadata[resource.FinalizerKey]
	}

	if !inputHasKey && existingHasKey {
		if input.Metadata == nil {
			input.Metadata = make(map[string]string)
		}
		input.Metadata[resource.FinalizerKey] = existing.Metadata[resource.FinalizerKey]
	}
}
