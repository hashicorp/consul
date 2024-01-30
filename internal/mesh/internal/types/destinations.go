// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"net"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/catalog/workloadselector"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterDestinations(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbmesh.DestinationsType,
		Proto:    &pbmesh.Destinations{},
		Scope:    resource.ScopeNamespace,
		Mutate:   MutateDestinations,
		Validate: ValidateDestinations,
		ACLs:     workloadselector.ACLHooks[*pbmesh.Destinations](),
	})
}

var MutateDestinations = resource.DecodeAndMutate(mutateDestinations)

func mutateDestinations(res *DecodedDestinations) (bool, error) {
	changed := false

	for _, dest := range res.Data.Destinations {
		if dest.DestinationRef == nil {
			continue // skip; let the validation hook error out instead
		}

		// TODO(peering/v2): handle non-local peer references here somehow
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

	return changed, nil
}

func isLocalPeer(p string) bool {
	return p == resource.DefaultPeerName || p == ""
}

var ValidateDestinations = resource.DecodeAndValidate(validateDestinations)

func validateDestinations(res *DecodedDestinations) error {
	var merr error

	if selErr := catalog.ValidateSelector(res.Data.Workloads, false); selErr != nil {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "workloads",
			Wrapped: selErr,
		})
	}

	if res.Data.GetPqDestinations() != nil {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "pq_destinations",
			Wrapped: resource.ErrUnsupported,
		})
	}

	for i, dest := range res.Data.Destinations {
		wrapDestErr := func(err error) error {
			return resource.ErrInvalidListElement{
				Name:    "destinations",
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

		if portErr := catalog.ValidateServicePortID(dest.DestinationPort); portErr != nil {
			merr = multierror.Append(merr, wrapDestErr(resource.ErrInvalidField{
				Name:    "destination_port",
				Wrapped: portErr,
			}))
		}

		if dest.GetDatacenter() != "" {
			merr = multierror.Append(merr, wrapDestErr(resource.ErrInvalidField{
				Name:    "datacenter",
				Wrapped: resource.ErrUnsupported,
			}))
		}

		if listenAddrErr := validateListenAddr(dest); listenAddrErr != nil {
			merr = multierror.Append(merr, wrapDestErr(listenAddrErr))
		}
	}

	return merr
}

func validateListenAddr(dest *pbmesh.Destination) error {
	var merr error

	if dest.GetListenAddr() == nil {
		return multierror.Append(merr, resource.ErrInvalidFields{
			Names:   []string{"ip_port", "unix"},
			Wrapped: resource.ErrMissingOneOf,
		})
	}

	switch listenAddr := dest.GetListenAddr().(type) {
	case *pbmesh.Destination_IpPort:
		if ipPortErr := validateIPPort(listenAddr.IpPort); ipPortErr != nil {
			merr = multierror.Append(merr, resource.ErrInvalidField{
				Name:    "ip_port",
				Wrapped: ipPortErr,
			})
		}
	case *pbmesh.Destination_Unix:
		if !catalog.IsValidUnixSocketPath(listenAddr.Unix.GetPath()) {
			merr = multierror.Append(merr, resource.ErrInvalidField{
				Name: "unix",
				Wrapped: resource.ErrInvalidField{
					Name:    "path",
					Wrapped: errInvalidUnixSocketPath,
				},
			})
		}
	}

	return merr
}

func validateIPPort(ipPort *pbmesh.IPPortAddress) error {
	var merr error

	if listenPortErr := validatePort(ipPort.GetPort(), "port"); listenPortErr != nil {
		merr = multierror.Append(merr, listenPortErr)
	}

	if net.ParseIP(ipPort.GetIp()) == nil {
		merr = multierror.Append(merr, resource.ErrInvalidField{
			Name:    "ip",
			Wrapped: errInvalidIP,
		})
	}

	return merr
}
