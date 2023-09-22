// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterComputedTrafficPermission(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbauth.ComputedTrafficPermissionsType,
		Proto:    &pbauth.ComputedTrafficPermissions{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateComputedTrafficPermissions,
	})
}

func ValidateComputedTrafficPermissions(res *pbresource.Resource) error {
	var ctp pbauth.ComputedTrafficPermissions

	if err := res.Data.UnmarshalTo(&ctp); err != nil {
		return resource.NewErrDataParse(&ctp, err)
	}

	var merr error

	for i, permission := range ctp.AllowPermissions {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "allow_permissions",
				Index:   i,
				Wrapped: err,
			}
		}
		if err := validatePermission(permission, wrapErr); err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	for i, permission := range ctp.DenyPermissions {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "deny_permissions",
				Index:   i,
				Wrapped: err,
			}
		}
		if err := validatePermission(permission, wrapErr); err != nil {
			merr = multierror.Append(merr, err)
		}
	}

	return merr
}
