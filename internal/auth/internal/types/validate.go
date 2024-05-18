// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func validatePermissions(id *pbresource.ID, data interface{ GetPermissions() []*pbauth.Permission }) error {
	var merr error
	for i, permission := range data.GetPermissions() {
		wrapErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "permissions",
				Index:   i,
				Wrapped: err,
			}
		}
		if err := validatePermission(permission, id, wrapErr); err != nil {
			merr = multierror.Append(merr, err)
		}
	}
	return merr
}

func validatePermission(p *pbauth.Permission, id *pbresource.ID, wrapErr func(error) error) error {
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
		if sourceHasIncompatibleTenancies(src, id) {
			merr = multierror.Append(merr, wrapSrcErr(errSourcesTenancy))
		}

		if src.Namespace == "" && src.IdentityName != "" {
			merr = multierror.Append(merr, wrapSrcErr(errSourceWildcards))
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
			if sourceHasIncompatibleTenancies(d, id) {
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
		if len(dest.Headers) > 0 {
			for h, hdr := range dest.Headers {
				wrapHeaderErr := func(err error) error {
					return wrapDestRuleErr(resource.ErrInvalidListElement{
						Name:    "destination_header_rules",
						Index:   h,
						Wrapped: err,
					})
				}
				if len(hdr.Name) == 0 {
					merr = multierror.Append(merr, wrapHeaderErr(resource.ErrInvalidListElement{
						Name:    "destination_header_rule",
						Wrapped: errHeaderRulesInvalid,
					}))
				}
			}
		}
		if dest.IsEmpty() {
			merr = multierror.Append(merr, wrapDestRuleErr(resource.ErrInvalidListElement{
				Name:    "destination_rule",
				Wrapped: errInvalidRule,
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
				for eh, hdr := range excl.Headers {
					wrapExclHeaderErr := func(err error) error {
						return wrapDestRuleErr(resource.ErrInvalidListElement{
							Name:    "exclude_permission_header_rules",
							Index:   eh,
							Wrapped: err,
						})
					}
					if len(hdr.Name) == 0 {
						merr = multierror.Append(merr, wrapExclHeaderErr(resource.ErrInvalidListElement{
							Name:    "exclude_permission_header_rule",
							Wrapped: errHeaderRulesInvalid,
						}))
					}
				}
				for _, m := range excl.Methods {
					if len(dest.Methods) != 0 && !slices.Contains(dest.Methods, m) {
						merr = multierror.Append(merr, wrapExclPermRuleErr(resource.ErrInvalidListElement{
							Name:    "exclude_permission_header_rule",
							Wrapped: errExclValuesMustBeSubset,
						}))
					}
				}
				for _, port := range excl.PortNames {
					if len(dest.PortNames) != 0 && !slices.Contains(dest.PortNames, port) {
						merr = multierror.Append(merr, wrapExclPermRuleErr(resource.ErrInvalidListElement{
							Name:    "exclude_permission_header_rule",
							Wrapped: errExclValuesMustBeSubset,
						}))
					}
				}
				if excl.IsEmpty() {
					merr = multierror.Append(merr, wrapExclPermRuleErr(resource.ErrInvalidListElement{
						Name:    "exclude_permission_rule",
						Wrapped: errInvalidRule,
					}))
				}
			}
		}
	}

	return merr
}

func sourceHasIncompatibleTenancies(src pbauth.SourceToSpiffe, id *pbresource.ID) bool {
	if id.Tenancy == nil {
		id.Tenancy = &pbresource.Tenancy{}
	}
	peerSet := !isLocalPeer(src.GetPeer())
	apSet := src.GetPartition() != id.Tenancy.Partition
	sgSet := src.GetSamenessGroup() != ""

	return (apSet && peerSet) || (apSet && sgSet) || (peerSet && sgSet)
}

func isLocalPeer(p string) bool {
	return p == "local" || p == ""
}
