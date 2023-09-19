// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	TrafficPermissionsKind = "TrafficPermissions"
)

var (
	TrafficPermissionsV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         TrafficPermissionsKind,
	}

	TrafficPermissionsType = TrafficPermissionsV2Beta1Type
)

func RegisterTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     TrafficPermissionsV2Beta1Type,
		Proto:    &pbauth.TrafficPermissions{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateTrafficPermissions,
		Mutate:   MutateTrafficPermissions,
	})
}

func MutateTrafficPermissions(res *pbresource.Resource) error {
	var tp pbauth.TrafficPermissions

	if err := res.Data.UnmarshalTo(&tp); err != nil {
		return resource.NewErrDataParse(&tp, err)
	}

	changed := false

	for _, p := range tp.Permissions {
		for _, s := range p.Sources {
			if updated := normalizedTenancyForSource(s, res.Id.Tenancy); updated {
				changed = true
			}
		}
	}

	if !changed {
		return nil
	}

	return res.Data.MarshalFrom(&tp)
}

func normalizedTenancyForSource(src *pbauth.Source, parentTenancy *pbresource.Tenancy) bool {
	var changed bool

	if t, c := defaultedSourceTenancy(src, parentTenancy); c {
		src.Partition = t.Partition
		src.Peer = t.PeerName
		changed = true
	}

	for _, e := range src.Exclude {
		if t, c := defaultedSourceTenancy(e, parentTenancy); c {
			e.Partition = t.Partition
			e.Peer = t.PeerName
			changed = true
		}
	}

	return changed
}

func defaultedSourceTenancy(s pbauth.SourceToSpiffe, parentTenancy *pbresource.Tenancy) (*pbresource.Tenancy, bool) {
	if !isLocalPeer(s.GetPeer()) {
		return nil, false
	}

	tenancy := pbauth.SourceToTenancy(s)
	origTenancy := proto.Clone(tenancy).(*pbresource.Tenancy)
	// This uses partition tenancy and traffic permissions use namespace tenancy. This is because namespace  can be empty.
	resource.DefaultTenancy(tenancy, parentTenancy, resource.DefaultPartitionedTenancy())

	var changed bool
	if !proto.Equal(tenancy, origTenancy) {
		changed = true
	}

	return tenancy, changed
}

func ValidateTrafficPermissions(res *pbresource.Resource) error {
	var tp pbauth.TrafficPermissions

	if err := res.Data.UnmarshalTo(&tp); err != nil {
		return resource.NewErrDataParse(&tp, err)
	}

	var merr error

	if tp.Action == pbauth.Action_ACTION_UNSPECIFIED {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "data.action",
			Wrapped: errInvalidAction,
		})
	}
	if tp.Destination == nil || (len(tp.Destination.IdentityName) == 0) {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "data.destination",
			Wrapped: resource.ErrEmpty,
		})
	}
	// Validate permissions
	for i, permission := range tp.Permissions {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "permissions",
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

func validatePermission(p *pbauth.Permission, wrapErr func(error) error) error {
	var merr error

	for s, src := range p.Sources {
		wrapSrcErr := func(err error) error {
			return wrapErr(resource.ErrInvalidListElement{
				Name:    "sources",
				Index:   s,
				Wrapped: err,
			})
		}
		if sourceHasIncompatibleTenancies(src) {
			merr = multierror.Append(merr, wrapSrcErr(resource.ErrInvalidListElement{
				Name:    "source",
				Wrapped: errSourcesTenancy,
			}))
		}

		if src.Namespace == "" && src.IdentityName != "" {
			merr = multierror.Append(merr, wrapSrcErr(resource.ErrInvalidListElement{
				Name:    "source",
				Wrapped: errSourceWildcards,
			}))
		}

		for e, d := range src.Exclude {
			wrapExclSrcErr := func(err error) error {
				return wrapErr(resource.ErrInvalidListElement{
					Name:    "exclude_sources",
					Index:   e,
					Wrapped: err,
				})
			}
			if sourceHasIncompatibleTenancies(d) {
				merr = multierror.Append(merr, wrapExclSrcErr(resource.ErrInvalidListElement{
					Name:    "exclude_source",
					Wrapped: errSourcesTenancy,
				}))
			}

			if d.Namespace == "" && d.IdentityName != "" {
				merr = multierror.Append(merr, wrapExclSrcErr(resource.ErrInvalidListElement{
					Name:    "source",
					Wrapped: errSourceWildcards,
				}))
			}
		}
	}
	for d, dest := range p.DestinationRules {
		wrapDestRuleErr := func(err error) error {
			return wrapErr(resource.ErrInvalidListElement{
				Name:    "destination_rules",
				Index:   d,
				Wrapped: err,
			})
		}
		if (len(dest.PathExact) > 0 && len(dest.PathPrefix) > 0) ||
			(len(dest.PathRegex) > 0 && len(dest.PathExact) > 0) ||
			(len(dest.PathRegex) > 0 && len(dest.PathPrefix) > 0) {
			merr = multierror.Append(merr, wrapDestRuleErr(resource.ErrInvalidListElement{
				Name:    "destination_rule",
				Wrapped: errInvalidPrefixValues,
			}))
		}
		if len(dest.Exclude) > 0 {
			for e, excl := range dest.Exclude {
				wrapExclPermRuleErr := func(err error) error {
					return wrapDestRuleErr(resource.ErrInvalidListElement{
						Name:    "exclude_permission_rules",
						Index:   e,
						Wrapped: err,
					})
				}
				if (len(excl.PathExact) > 0 && len(excl.PathPrefix) > 0) ||
					(len(excl.PathRegex) > 0 && len(excl.PathExact) > 0) ||
					(len(excl.PathRegex) > 0 && len(excl.PathPrefix) > 0) {
					merr = multierror.Append(merr, wrapExclPermRuleErr(resource.ErrInvalidListElement{
						Name:    "exclude_permission_rule",
						Wrapped: errInvalidPrefixValues,
					}))
				}
			}
		}
	}

	return merr
}

func sourceHasIncompatibleTenancies(src pbauth.SourceToSpiffe) bool {
	peerSet := src.GetPeer() != resource.DefaultPeerName
	apSet := src.GetPartition() != resource.DefaultPartitionName
	sgSet := src.GetSamenessGroup() != ""

	return (apSet && peerSet) || (apSet && sgSet) || (peerSet && sgSet)
}

func isLocalPeer(p string) bool {
	return p == "local" || p == ""
}
