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

type DecodedHealthStatus = resource.DecodedResource[*pbcatalog.HealthStatus]

func RegisterHealthStatus(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.HealthStatusType,
		Proto:    &pbcatalog.HealthStatus{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateHealthStatus,
		ACLs: &resource.ACLHooks{
			Read:  resource.AuthorizeReadWithResource(aclReadHookHealthStatus),
			Write: aclWriteHookHealthStatus,
			List:  resource.NoOpACLListHook,
		},
	})
}

var ValidateHealthStatus = resource.DecodeAndValidate(validateHealthStatus)

func validateHealthStatus(res *DecodedHealthStatus) error {
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

	// Ensure that the HealthStatus' owner is a type that we want to allow. The
	// owner is currently the resource that this HealthStatus applies to. If we
	// change this to be a parent reference within the HealthStatus.Data then
	// we could allow for other owners.
	if res.Resource.Owner == nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "owner",
			Wrapped: resource.ErrMissing,
		})
	} else if !resource.EqualType(res.Owner.Type, pbcatalog.WorkloadType) {
		err = multierror.Append(err, resource.ErrOwnerTypeInvalid{ResourceType: res.Id.Type, OwnerType: res.Owner.Type})
	}

	return err
}

func aclReadHookHealthStatus(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	// For a health status of a workload we need to check service:read perms.
	if res.GetOwner() != nil && resource.EqualType(res.GetOwner().GetType(), pbcatalog.WorkloadType) {
		return authorizer.ToAllowAuthorizer().ServiceReadAllowed(res.GetOwner().GetName(), authzContext)
	}

	return acl.PermissionDenied("cannot read catalog.HealthStatus because there is no owner")
}

func aclWriteHookHealthStatus(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *pbresource.Resource) error {
	// For a health status of a workload we need to check service:write perms.
	if res.GetOwner() != nil && resource.EqualType(res.GetOwner().GetType(), pbcatalog.WorkloadType) {
		return authorizer.ToAllowAuthorizer().ServiceWriteAllowed(res.GetOwner().GetName(), authzContext)
	}

	return acl.PermissionDenied("cannot write catalog.HealthStatus because there is no owner")
}
