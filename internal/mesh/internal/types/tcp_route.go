// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	TCPRouteKind = "TCPRoute"
)

var (
	TCPRouteV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2beta1,
		Kind:         TCPRouteKind,
	}

	TCPRouteType = TCPRouteV2Beta1Type
)

func RegisterTCPRoute(r resource.Registry) {
	r.Register(resource.Registration{
		Type:  TCPRouteV2Beta1Type,
		Proto: &pbmesh.TCPRoute{},
		Scope: resource.ScopeNamespace,
		// TODO(rb): normalize parent/backend ref tenancies in a Mutate hook
		Validate: ValidateTCPRoute,
	})
}

func ValidateTCPRoute(res *pbresource.Resource) error {
	var route pbmesh.TCPRoute

	if err := res.Data.UnmarshalTo(&route); err != nil {
		return resource.NewErrDataParse(&route, err)
	}

	var merr error

	if err := validateParentRefs(route.ParentRefs); err != nil {
		merr = multierror.Append(merr, err)
	}

	for i, rule := range route.Rules {
		wrapRuleErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "rules",
				Index:   i,
				Wrapped: err,
			}
		}

		if len(rule.BackendRefs) == 0 {
			/*
				BackendRefs (optional)¶

				BackendRefs defines API objects where matching requests should be
				sent. If unspecified, the rule performs no forwarding. If
				unspecified and no filters are specified that would result in a
				response being sent, a 404 error code is returned.
			*/
			merr = multierror.Append(merr, wrapRuleErr(
				resource.ErrInvalidField{
					Name:    "backend_refs",
					Wrapped: resource.ErrEmpty,
				},
			))
		}
		for j, hbref := range rule.BackendRefs {
			wrapBackendRefErr := func(err error) error {
				return wrapRuleErr(resource.ErrInvalidListElement{
					Name:    "backend_refs",
					Index:   j,
					Wrapped: err,
				})
			}
			for _, err := range validateBackendRef(hbref.BackendRef) {
				merr = multierror.Append(merr, wrapBackendRefErr(
					resource.ErrInvalidField{
						Name:    "backend_ref",
						Wrapped: err,
					},
				))
			}
		}
	}

	return merr
}
