// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
)

// Delete deletes a resource.
// - To delete a resource regardless of the stored version, set Version = ""
// - Supports deleting a resource by name, hence Id.Uid may be empty.
// - Delete of a previously deleted or non-existent resource is a no-op to support idempotency.
// - Errors with Aborted if the requested Version does not match the stored Version.
// - Errors with PermissionDenied if ACL check fails
func (s *Server) Delete(ctx context.Context, req *pbresource.DeleteRequest) (*pbresource.DeleteResponse, error) {
	reg, err := s.ensureDeleteRequestValid(req)
	if err != nil {
		return nil, err
	}

	entMeta := v2TenancyToV1EntMeta(req.Id.Tenancy)
	authz, authzContext, err := s.getAuthorizer(tokenFromContext(ctx), entMeta)
	if err != nil {
		return nil, err
	}

	// Retrieve resource since ACL hook requires it. Furthermore, we'll need the
	// read to be strongly consistent if the passed in Version or Uid are empty.
	consistency := storage.EventualConsistency
	if req.Version == "" || req.Id.Uid == "" {
		consistency = storage.StrongConsistency
	}

	// Apply defaults when tenancy units empty.
	v1EntMetaToV2Tenancy(reg, entMeta, req.Id.Tenancy)

	existing, err := s.Backend.Read(ctx, consistency, req.Id)
	switch {
	case errors.Is(err, storage.ErrNotFound):
		// Deletes are idempotent so no-op when not found
		return &pbresource.DeleteResponse{}, nil
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed read: %v", err)
	}

	// Check ACLs
	err = reg.ACLs.Write(authz, authzContext, existing)
	switch {
	case acl.IsErrPermissionDenied(err):
		return nil, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return nil, status.Errorf(codes.Internal, "failed write acl: %v", err)
	}

	deleteVersion := req.Version
	deleteId := req.Id
	if deleteVersion == "" || deleteId.Uid == "" {
		deleteVersion = existing.Version
		deleteId = existing.Id
	}

	// Check finalizers for a deferred delete
	if resource.HasFinalizers(existing) {
		if resource.IsMarkedForDeletion(existing) {
			// Delete previously requested and finalizers still present so nothing to do
			return &pbresource.DeleteResponse{}, nil
		}

		// Mark for deletion and let controllers that put finalizers in place do their
		// thing. Note we're passing in a clone of the recently read resource since
		// we've not crossed a network/serialization boundary since the read and we
		// don't want to mutate the in-mem reference.
		return s.markForDeletion(ctx, clone(existing))
	}

	// Continue with an immediate delete
	if err := s.maybeCreateTombstone(ctx, deleteId); err != nil {
		return nil, err
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

func (s *Server) markForDeletion(ctx context.Context, res *pbresource.Resource) (*pbresource.DeleteResponse, error) {
	// Write the deletion timestamp
	res.Metadata[resource.DeletionTimestampKey] = time.Now().Format(time.RFC3339)
	_, err := s.Write(ctx, &pbresource.WriteRequest{Resource: res})
	if err != nil {
		return nil, err
	}
	return &pbresource.DeleteResponse{}, nil
}

// Create a tombstone to capture the intent to delete child resources.
// Tombstones are created preemptively to prevent partial failures even though
// we are currently unaware of the success/failure/no-op of DeleteCAS. In
// the failure and no-op cases the tombstone is effectively a no-op and will
// still be deleted from the system by the reaper controller.
func (s *Server) maybeCreateTombstone(ctx context.Context, deleteId *pbresource.ID) error {
	// Don't create a tombstone when the resource being deleted is itself a tombstone.
	if resource.EqualType(resource.TypeV1Tombstone, deleteId.Type) {
		return nil
	}

	data, err := anypb.New(&pbresource.Tombstone{Owner: deleteId})
	if err != nil {
		return status.Errorf(codes.Internal, "failed creating tombstone: %v", err)
	}

	// Since a tombstone is an internal resource type that should not be visible
	// or accessible by users, we're writing to the backend directly instead of
	// using the resource service's Write endpoint. This bypasses resource level
	// concerns that are either not relevant (valiation and mutation hooks) or
	// futher complicate the implementation (user provided tokens having
	// awareness of tombstone ACLs).
	//
	// ErrCASFailure should never happen since an empty Version is always passed.
	//
	// TODO(spatel): Probably a good idea to block writes of TypeV1Tombstone
	//  	on the ResourceService.Write() endpoint to lock things down?
	_, err = s.Backend.WriteCAS(ctx, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    resource.TypeV1Tombstone,
			Tenancy: deleteId.Tenancy,
			Name:    TombstoneNameFor(deleteId),
			Uid:     ulid.Make().String(),
		},
		Generation: ulid.Make().String(),
		Data:       data,
		Metadata: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
		},
	})

	switch {
	case err == nil:
		// Success!
		return nil
	case errors.Is(err, storage.ErrWrongUid):
		// Backend has detected that we're trying to change the Uid for an
		// existing tombstone (probably created from a previously failed Delete
		// where the tombstone WriteCAS succeeded but the resource DeleteCAS
		// failed). The fact that the tombstone already exists means we're good.
		return nil
	default:
		return status.Errorf(codes.Internal, "failed writing tombstone: %v", err)
	}
}

func (s *Server) ensureDeleteRequestValid(req *pbresource.DeleteRequest) (*resource.Registration, error) {
	if req.Id == nil {
		return nil, status.Errorf(codes.InvalidArgument, "id is required")
	}

	if err := validateId(req.Id, "id"); err != nil {
		return nil, err
	}

	reg, err := s.resolveType(req.Id.Type)
	if err != nil {
		return nil, err
	}

	if err = checkV2Tenancy(s.UseV2Tenancy, req.Id.Type); err != nil {
		return nil, err
	}

	if err := validateScopedTenancy(reg.Scope, reg.Type, req.Id.Tenancy, false); err != nil {
		return nil, err
	}

	if err := blockBuiltinsDeletion(reg.Type, req.Id); err != nil {
		return nil, err
	}
	return reg, nil
}

// Maintains a deterministic mapping between a resource and it's tombstone's
// name by embedding the resources's Uid in the name.
func TombstoneNameFor(deleteId *pbresource.ID) string {
	// deleteId.Name is just included for easier identification
	return fmt.Sprintf("tombstone-%v-%v", deleteId.Name, strings.ToLower(deleteId.Uid))
}

func blockDefaultNamespaceDeletion(rtype *pbresource.Type, id *pbresource.ID) error {
	if id.Name == resource.DefaultNamespaceName &&
		id.Tenancy.Partition == resource.DefaultPartitionName &&
		resource.EqualType(rtype, pbtenancy.NamespaceType) {
		return status.Errorf(codes.InvalidArgument, "cannot delete default namespace")
	}
	return nil
}
