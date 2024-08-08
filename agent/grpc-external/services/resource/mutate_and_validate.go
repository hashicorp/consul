// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resource

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) MutateAndValidate(ctx context.Context, req *pbresource.MutateAndValidateRequest) (*pbresource.MutateAndValidateResponse, error) {
	tenancyMarkedForDeletion, err := s.mutateAndValidate(ctx, req.Resource, false)
	if err != nil {
		return nil, err
	}

	if tenancyMarkedForDeletion {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"tenancy marked for deletion: %s/%s",
			req.Resource.Id.Tenancy.Partition,
			req.Resource.Id.Tenancy.Namespace,
		)
	}
	return &pbresource.MutateAndValidateResponse{Resource: req.Resource}, nil
}

// private DRY impl that is used by both the Write and MutateAndValidate RPCs.
func (s *Server) mutateAndValidate(ctx context.Context, res *pbresource.Resource, enforceLicenseCheck bool) (tenancyMarkedForDeletion bool, err error) {
	reg, err := s.ensureResourceValid(res, enforceLicenseCheck)
	if err != nil {
		return false, err
	}

	v1EntMeta := v2TenancyToV1EntMeta(res.Id.Tenancy)
	authz, authzContext, err := s.getAuthorizer(tokenFromContext(ctx), v1EntMeta)
	if err != nil {
		return false, err
	}
	v1EntMetaToV2Tenancy(reg, v1EntMeta, res.Id.Tenancy)

	// Check the user sent the correct type of data.
	if res.Data != nil && !res.Data.MessageIs(reg.Proto) {
		got := strings.TrimPrefix(res.Data.TypeUrl, "type.googleapis.com/")

		return false, status.Errorf(
			codes.InvalidArgument,
			"resource.data is of wrong type (expected=%q, got=%q)",
			reg.Proto.ProtoReflect().Descriptor().FullName(),
			got,
		)
	}

	if err = reg.Mutate(res); err != nil {
		return false, status.Errorf(codes.Internal, "failed mutate hook: %v", err.Error())
	}

	if err = reg.Validate(res); err != nil {
		return false, status.Error(codes.InvalidArgument, err.Error())
	}

	// ACL check comes before tenancy existence checks to not leak tenancy "existence".
	err = reg.ACLs.Write(authz, authzContext, res)
	switch {
	case acl.IsErrPermissionDenied(err):
		return false, status.Error(codes.PermissionDenied, err.Error())
	case err != nil:
		return false, status.Errorf(codes.Internal, "failed write acl: %v", err)
	}

	// Check tenancy exists for the V2 resource
	if err = tenancyExists(reg, s.TenancyBridge, res.Id.Tenancy, codes.InvalidArgument); err != nil {
		return false, err
	}

	// This is used later in the "create" and "update" paths to block non-delete related writes
	// when a tenancy unit has been marked for deletion.
	tenancyMarkedForDeletion, err = isTenancyMarkedForDeletion(reg, s.TenancyBridge, res.Id.Tenancy)
	if err != nil {
		return false, status.Errorf(codes.Internal, "failed tenancy marked for deletion check: %v", err)
	}
	if tenancyMarkedForDeletion {
		return true, nil
	}
	return false, nil
}

func (s *Server) ensureResourceValid(res *pbresource.Resource, enforceLicenseCheck bool) (*resource.Registration, error) {
	var field string
	switch {
	case res == nil:
		field = "resource"
	case res.Id == nil:
		field = "resource.id"
	}

	if field != "" {
		return nil, status.Errorf(codes.InvalidArgument, "%s is required", field)
	}

	if err := validateId(res.Id, "resource.id"); err != nil {
		return nil, err
	}

	if res.Owner != nil {
		if err := validateId(res.Owner, "resource.owner"); err != nil {
			return nil, err
		}
	}

	// Check type exists.
	reg, err := s.resolveType(res.Id.Type)
	if err != nil {
		return nil, err
	}

	// Since this is shared by Write and MutateAndValidate, only fail the operation
	// if it's a write operation and the feature is not allowed by the license.
	if err = s.FeatureCheck(reg); err != nil && enforceLicenseCheck {
		return nil, err
	}

	// Check scope
	if reg.Scope == resource.ScopePartition && res.Id.Tenancy.Namespace != "" {
		return nil, status.Errorf(
			codes.InvalidArgument,
			"partition scoped resource %s cannot have a namespace. got: %s",
			resource.ToGVK(res.Id.Type),
			res.Id.Tenancy.Namespace,
		)
	}

	return reg, nil
}
