// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterUpstreams(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.DestinationsType,
		Proto:    &pbmesh.Destinations{},
		Scope:    resource.ScopeNamespace,
		Mutate:   MutateUpstreams,
		Validate: ValidateUpstreams,
	})
}

func MutateUpstreams(res *pbresource.Resource) error {
	var destinations pbmesh.Destinations

	if err := res.Data.UnmarshalTo(&destinations); err != nil {
		return resource.NewErrDataParse(&destinations, err)
	}

	changed := false

	for _, dest := range destinations.Destinations {
		if dest.DestinationRef == nil {
			continue // skip; let the validation hook error out instead
		}
		if dest.DestinationRef.Tenancy != nil && !isLocalPeer(dest.DestinationRef.Tenancy.PeerName) {
			// TODO(peering/v2): remove this bypass when we know what to do with
			// non-local peer references.
			continue
		}
		orig := proto.Clone(dest.DestinationRef).(*pbresource.Reference)
		resource.DefaultReferenceTenancy(
			dest.DestinationRef,
			res.Id.GetTenancy(),
			resource.DefaultNamespacedTenancy(), // Services are all namespace scoped.
		)

		if !proto.Equal(orig, dest.DestinationRef) {
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return res.Data.MarshalFrom(&destinations)
}

func isLocalPeer(p string) bool {
	return p == "local" || p == ""
}

func ValidateUpstreams(res *pbresource.Resource) error {
	var destinations pbmesh.Destinations

	if err := res.Data.UnmarshalTo(&destinations); err != nil {
		return resource.NewErrDataParse(&destinations, err)
	}

	var merr error

	for i, dest := range destinations.Destinations {
		wrapDestErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "upstreams",
				Index:   i,
				Wrapped: err,
			}
		}

		wrapRefErr := func(err error) error {
			return wrapDestErr(resource.ErrInvalidField{
				Name:    "destination_ref",
				Wrapped: err,
			})
		}

		if refErr := catalog.ValidateLocalServiceRefNoSection(dest.DestinationRef, wrapRefErr); refErr != nil {
			merr = multierror.Append(merr, refErr)
		}

		// TODO(v2): validate port name using catalog validator
		// TODO(v2): validate ListenAddr
	}

	// TODO(v2): validate workload selectors

	return merr
}
