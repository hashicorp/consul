// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
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

// Create a tombstone to capture the intent to delete child resources.
// Tombstones are created preemptively to prevent partial failures even though
// we are currently unaware of the success/failure/no-op of DeleteCAS. In
// the failure and no-op cases the tombstone is effectively a no-op and will
// still be deleted from the system by the reaper controller.
func (s *Server) maybeCreateTombstone(ctx context.Context, deleteId *pbresource.ID) error {
	// Don't create a tombstone when the resource being deleted is itself a tombstone.
	if proto.Equal(resource.TypeV1Tombstone, deleteId.Type) {
		return nil
	}

	data, err := anypb.New(&pbresource.Tombstone{OwnerId: deleteId})
	if err != nil {
		return status.Errorf(codes.Internal, "failed creating tombstone: %v", err)
	}

	tombstone := &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    resource.TypeV1Tombstone,
			Tenancy: deleteId.Tenancy,
			Name:    tombstoneName(deleteId),
		},
		Data: data,
		Metadata: map[string]string{
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	_, err = s.Write(ctx, &pbresource.WriteRequest{Resource: tombstone})
	if err != nil {
		return fmt.Errorf("failed writing tombstone: %w", err)
	}
	return nil
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

// Maintains a deterministic mapping between a resource and it's tombstone's
// name by embedding the resources's Uid in the name.
func tombstoneName(deleteId *pbresource.ID) string {
	// deleteId.Name is just included for easier identification
	return fmt.Sprintf("tombstone-%v-%v", deleteId.Name, deleteId.Uid)
}
