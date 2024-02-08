// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"golang.org/x/exp/slices"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
)

type DecodedTrafficPermissions = resource.DecodedResource[*pbauth.TrafficPermissions]

func RegisterTrafficPermissions(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  pbauth.TrafficPermissionsType,
		Proto: &pbauth.TrafficPermissions{},
		ACLs: &resource.ACLHooks{
			Read:  resource.DecodeAndAuthorizeRead(aclReadHookTrafficPermissions),
			Write: resource.DecodeAndAuthorizeWrite(aclWriteHookTrafficPermissions),
			List:  resource.NoOpACLListHook,
		},
		Validate: ValidateTrafficPermissions,
		Mutate:   MutateTrafficPermissions,
		Scope:    resource.ScopeNamespace,
	})
}

var MutateTrafficPermissions = resource.DecodeAndMutate(mutateTrafficPermissions)

func mutateTrafficPermissions(res *DecodedTrafficPermissions) (bool, error) {
	var changed bool

	for _, p := range res.Data.Permissions {
		for _, s := range p.Sources {
			if updated := normalizedTenancyForSource(s, res.Id.Tenancy); updated {
				changed = true
			}
		}
	}

	return changed, nil
}

func normalizedTenancyForSource(src *pbauth.Source, parentTenancy *pbresource.Tenancy) bool {
	var changed bool

	if t, c := defaultedSourceTenancy(src, parentTenancy); c {
		src.Partition = t.Partition
		// TODO(peering/v2) revisit default peer source
		// src.Peer = t.PeerName
		src.Peer = resource.DefaultPeerName
		src.Namespace = t.Namespace
		changed = true
	}

	for _, e := range src.Exclude {
		if t, c := defaultedSourceTenancy(e, parentTenancy); c {
			e.Partition = t.Partition
			// TODO(peering/v2) revisit default peer source
			// e.Peer = t.PeerName
			e.Peer = resource.DefaultPeerName
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

	// TODO(peering/v2) default peer name somehow
	// var peerChanged bool
	// tenancy.PeerName, peerChanged = firstNonEmptyString(tenancy.PeerName, parentTenancy.PeerName, resource.DefaultPeerName)

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

	// TODO(peering/v2) take peer being changed into account
	return tenancy, partitionChanged || namespaceChanged // || peerChange
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

var ValidateTrafficPermissions = resource.DecodeAndValidate(validateTrafficPermissions)

// validator takes a traffic permission and ensures that it conforms to the actions allowed in
// either CE or Enterprise versions of Consul
type validator interface {
	ValidateAction(data interface{ GetAction() pbauth.Action }) error
}

func validateTrafficPermissions(res *DecodedTrafficPermissions) error {
	var merr error

	err := v.ValidateAction(res.Data)
	if err != nil {
		merr = multierror.Append(merr, err)
	}

	if res.Data.Destination == nil || (len(res.Data.Destination.IdentityName) == 0) {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "data.destination",
			Wrapped: resource.ErrEmpty,
		})
	}
	// Validate permissions
	if err := validatePermissions(res.Id, res.Data); err != nil {
		merr = multierror.Append(merr, err)
	}

	return merr
}

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

func aclReadHookTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedTrafficPermissions) error {
	return authorizer.ToAllowAuthorizer().TrafficPermissionsReadAllowed(res.Data.Destination.IdentityName, authzContext)
}

func aclWriteHookTrafficPermissions(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedTrafficPermissions) error {
	return authorizer.ToAllowAuthorizer().TrafficPermissionsWriteAllowed(res.Data.Destination.IdentityName, authzContext)
}
