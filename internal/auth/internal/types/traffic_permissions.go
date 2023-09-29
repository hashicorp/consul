// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbauth.TrafficPermissionsType,
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

	var changed bool

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
		src.Namespace = t.Namespace
		changed = true
	}

	for _, e := range src.Exclude {
		if t, c := defaultedSourceTenancy(e, parentTenancy); c {
			e.Partition = t.Partition
			e.Peer = t.PeerName
			e.Namespace = t.Namespace
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

	var peerChanged bool
	tenancy.PeerName, peerChanged = firstNonEmptyString(tenancy.PeerName, parentTenancy.PeerName, resource.DefaultPeerName)

	var partitionChanged bool
	tenancy.Partition, partitionChanged = firstNonEmptyString(tenancy.Partition, parentTenancy.Partition, resource.DefaultPartitionName)

	var namespaceChanged bool
	if s.GetIdentityName() != "" {
		if tenancy.Partition == parentTenancy.Partition {
			tenancy.Namespace, namespaceChanged = firstNonEmptyString(tenancy.Namespace, parentTenancy.Namespace, resource.DefaultNamespaceName)
		} else {
			tenancy.Namespace, namespaceChanged = firstNonEmptyString(tenancy.Namespace, resource.DefaultNamespaceName, resource.DefaultNamespaceName)
		}
	}

	return tenancy, peerChanged || partitionChanged || namespaceChanged
}

func firstNonEmptyString(a, b, c string) (string, bool) {
	if a != "" {
		return a, false
	}

	if b != "" {
		return b, true
	}

	return c, true
}

func ValidateTrafficPermissions(res *pbresource.Resource) error {
	var tp pbauth.TrafficPermissions

	if err := res.Data.UnmarshalTo(&tp); err != nil {
		return resource.NewErrDataParse(&tp, err)
	}

	var merr error

	// enumcover:pbauth.Action
	switch tp.Action {
	case pbauth.Action_ACTION_ALLOW:
	case pbauth.Action_ACTION_DENY:
	case pbauth.Action_ACTION_UNSPECIFIED:
		fallthrough
	default:
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

	if len(p.Sources) == 0 {
		merr = multierror.Append(merr, wrapErr(resource.ErrInvalidField{
			Name:    "sources",
			Wrapped: resource.ErrEmpty,
		}))
	}

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
			merr = multierror.Append(merr, wrapSrcErr(resource.ErrInvalidField{
				Name:    "source",
				Wrapped: errSourceWildcards,
			}))
		}

		// Excludes are only valid for wildcard sources.
		if src.IdentityName != "" && len(src.Exclude) > 0 {
			merr = multierror.Append(merr, wrapSrcErr(resource.ErrInvalidField{
				Name:    "exclude_sources",
				Wrapped: errSourceExcludes,
			}))
			continue
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
				merr = multierror.Append(merr, wrapExclSrcErr(resource.ErrInvalidField{
					Name:    "exclude_source",
					Wrapped: errSourcesTenancy,
				}))
			}

			if d.Namespace == "" && d.IdentityName != "" {
				merr = multierror.Append(merr, wrapExclSrcErr(resource.ErrInvalidField{
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
