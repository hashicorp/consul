// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type DecodedNodeHealthStatus = resource.DecodedResource[*pbcatalog.NodeHealthStatus]

func RegisterNodeHealthStatus(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.NodeHealthStatusType,
		Proto:    &pbcatalog.NodeHealthStatus{},
		Scope:    resource.ScopePartition,
		Validate: ValidateNodeHealthStatus,
		ACLs: &resource.ACLHooks{
			Read:  resource.AuthorizeReadWithResource(aclReadHookNodeHealthStatus),
			Write: aclWriteHookNodeHealthStatus,
			List:  resource.NoOpACLListHook,
		},
	})
}

var ValidateNodeHealthStatus = resource.DecodeAndValidate(validateNodeHealthStatus)

func validateNodeHealthStatus(res *DecodedNodeHealthStatus) error {
	var err error

	// Should we allow empty types? I think for now it will be safest to require
	// the type field is set and we can relax this restriction in the future
	// if we deem it desirable.
	if res.Data.Type == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "type",
			Wrapped: resource.ErrMissing,
		})
	}

	switch res.Data.Status {
	case pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_MAINTENANCE:
	default:
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "status",
			Wrapped: errInvalidHealth,
		})
	}

	// Ensure that the NodeHealthStatus' owner is a type that we want to allow. The
	// owner is currently the resource that this NodeHealthStatus applies to. If we
	// change this to be a parent reference within the NodeHealthStatus.Data then
	// we could allow for other owners.
	if res.Resource.Owner == nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "owner",
			Wrapped: resource.ErrMissing,
		})
	} else if !resource.EqualType(res.Owner.Type, pbcatalog.NodeType) {
		err = multierror.Append(err, resource.ErrOwnerTypeInvalid{ResourceType: res.Id.Type, OwnerType: res.Owner.Type})
	}

	return err
}

func aclReadHookNodeHealthStatus(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	// For a health status of a node we need to check node:read perms.
	if res.GetOwner() != nil && resource.EqualType(res.GetOwner().GetType(), pbcatalog.NodeType) {
		return authorizer.ToAllowAuthorizer().NodeReadAllowed(res.GetOwner().GetName(), authzContext)
	}

	return acl.PermissionDenied("cannot read catalog.NodeHealthStatus because there is no owner")
}

func aclWriteHookNodeHealthStatus(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	// For a health status of a node we need to check node:write perms.
	if res.GetOwner() != nil && resource.EqualType(res.GetOwner().GetType(), pbcatalog.NodeType) {
		return authorizer.ToAllowAuthorizer().NodeWriteAllowed(res.GetOwner().GetName(), authzContext)
	}

	return acl.PermissionDenied("cannot write catalog.NodeHealthStatus because there is no owner")
}
