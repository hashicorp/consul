// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	TrafficPermissionsKind = "TrafficPermissions"
)

var (
	TrafficPermissionsV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         TrafficPermissionsKind,
	}

	TrafficPermissionsType = TrafficPermissionsV1Alpha1Type
)

func RegisterTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     TrafficPermissionsV1Alpha1Type,
		Proto:    &pbauth.TrafficPermissions{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateTrafficPermissions,
	})
}

func ValidateTrafficPermissions(res *pbresource.Resource) error {
	var tp pbauth.TrafficPermissions

	if err := res.Data.UnmarshalTo(&tp); err != nil {
		return resource.NewErrDataParse(&tp, err)
	}

	var err error

	if tp.Action == pbauth.Action_ACTION_UNSPECIFIED {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.action",
			Wrapped: errInvalidAction,
		})
	}
	if tp.Destination == nil || (len(tp.Destination.IdentityName) == 0) {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "data.destination",
			Wrapped: resource.ErrEmpty,
		})
	}
	// Validate permissions
	for i, permission := range tp.Permissions {
		wrapPermissionErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "permissions",
				Index:   i,
				Wrapped: err,
			}
		}
		for s, src := range permission.Sources {
			wrapSrcErr := func(err error) error {
				return wrapPermissionErr(resource.ErrInvalidListElement{
					Name:    "sources",
					Index:   s,
					Wrapped: err,
				})
			}
			if (len(src.Partition) > 0 && len(src.Peer) > 0) ||
				(len(src.Partition) > 0 && len(src.SamenessGroup) > 0) ||
				(len(src.Peer) > 0 && len(src.SamenessGroup) > 0) {
				err = multierror.Append(err, wrapSrcErr(resource.ErrInvalidListElement{
					Name:    "source",
					Wrapped: errSourcesTenancy,
				}))
			}
			if len(src.Exclude) > 0 {
				for e, d := range src.Exclude {
					wrapExclSrcErr := func(err error) error {
						return wrapPermissionErr(resource.ErrInvalidListElement{
							Name:    "exclude_sources",
							Index:   e,
							Wrapped: err,
						})
					}
					if (len(d.Partition) > 0 && len(d.Peer) > 0) ||
						(len(d.Partition) > 0 && len(d.SamenessGroup) > 0) ||
						(len(d.Peer) > 0 && len(d.SamenessGroup) > 0) {
						err = multierror.Append(err, wrapExclSrcErr(resource.ErrInvalidListElement{
							Name:    "exclude_source",
							Wrapped: errSourcesTenancy,
						}))
					}
				}
			}
		}
		if len(permission.DestinationRules) > 0 {
			for d, dest := range permission.DestinationRules {
				wrapDestRuleErr := func(err error) error {
					return wrapPermissionErr(resource.ErrInvalidListElement{
						Name:    "destination_rules",
						Index:   d,
						Wrapped: err,
					})
				}
				if (len(dest.PathExact) > 0 && len(dest.PathPrefix) > 0) ||
					(len(dest.PathRegex) > 0 && len(dest.PathExact) > 0) ||
					(len(dest.PathRegex) > 0 && len(dest.PathPrefix) > 0) {
					err = multierror.Append(err, wrapDestRuleErr(resource.ErrInvalidListElement{
						Name:    "destination_rule",
						Wrapped: errInvalidPrefixValues,
					}))
				}
				if len(dest.Exclude) > 0 {
					for e, excl := range dest.Exclude {
						wrapExclPermRuleErr := func(err error) error {
							return wrapPermissionErr(resource.ErrInvalidListElement{
								Name:    "exclude_permission_rules",
								Index:   e,
								Wrapped: err,
							})
						}
						if (len(excl.PathExact) > 0 && len(excl.PathPrefix) > 0) ||
							(len(excl.PathRegex) > 0 && len(excl.PathExact) > 0) ||
							(len(excl.PathRegex) > 0 && len(excl.PathPrefix) > 0) {
							err = multierror.Append(err, wrapExclPermRuleErr(resource.ErrInvalidListElement{
								Name:    "exclude_permission_rule",
								Wrapped: errInvalidPrefixValues,
							}))
						}
					}
				}
			}
		}
	}

	return err
}
