// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
)

func RegisterTCPRoute(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.TCPRouteType,
		Proto:    &pbmesh.TCPRoute{},
		Scope:    resource.ScopeNamespace,
		Mutate:   MutateTCPRoute,
		Validate: ValidateTCPRoute,
		ACLs:     xRouteACLHooks[*pbmesh.TCPRoute](),
	})
}

var MutateTCPRoute = resource.DecodeAndMutate(mutateTCPRoute)

func mutateTCPRoute(res *DecodedTCPRoute) (bool, error) {
	changed := false

	if mutateParentRefs(res.Id.Tenancy, res.Data.ParentRefs) {
		changed = true
	}

	for _, rule := range res.Data.Rules {
		for _, backend := range rule.BackendRefs {
			if backend.BackendRef == nil || backend.BackendRef.Ref == nil {
				continue
			}
			if mutateXRouteRef(res.Id.Tenancy, backend.BackendRef.Ref) {
				changed = true
			}
		}
	}

	return changed, nil
}

var ValidateTCPRoute = resource.DecodeAndValidate(validateTCPRoute)

func validateTCPRoute(res *DecodedTCPRoute) error {
	var merr error

	if err := validateParentRefs(res.Id, res.Data.ParentRefs); err != nil {
		merr = multierror.Append(merr, err)
	}

	if len(res.Data.Rules) > 1 {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "rules",
			Wrapped: fmt.Errorf("must only specify a single rule for now"),
		})
	}

	for i, rule := range res.Data.Rules {
		wrapRuleErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "rules",
				Index:   i,
				Wrapped: err,
			}
		}

		if len(rule.BackendRefs) == 0 {
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

			wrapBackendRefFieldErr := func(err error) error {
				return wrapBackendRefErr(resource.ErrInvalidField{
					Name:    "backend_ref",
					Wrapped: err,
				})
			}
			if err := validateBackendRef(hbref.BackendRef, wrapBackendRefFieldErr); err != nil {
				merr = multierror.Append(merr, err)
			}
		}
	}

	return merr
}
