// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/catalog/workloadselector"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
)

type DecodedService = resource.DecodedResource[*pbcatalog.Service]

func RegisterService(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.ServiceType,
		Proto:    &pbcatalog.Service{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateService,
		Mutate:   MutateService,
		ACLs:     workloadselector.ACLHooks[*pbcatalog.Service](),
	})
}

var MutateService = resource.DecodeAndMutate(mutateService)

func mutateService(res *DecodedService) (bool, error) {
	changed := false

	// Default service port protocols.
	for _, port := range res.Data.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_UNSPECIFIED {
			port.Protocol = pbcatalog.Protocol_PROTOCOL_TCP
			changed = true
		}
	}

	return changed, nil
}

var ValidateService = resource.DecodeAndValidate(validateService)

func validateService(res *DecodedService) error {
	var err error

	// Validate the workload selector. We are allowing selectors with no
	// selection criteria as it will allow for users to manually manage
	// ServiceEndpoints objects for this service such as when desiring to
	// configure endpoint information for external services that are not
	// registered as workloads
	if selErr := ValidateSelector(res.Data.Workloads, true); selErr != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "workloads",
			Wrapped: selErr,
		})
	}

	usedVirtualPorts := make(map[uint32]int)

	// Validate each port
	for idx, port := range res.Data.Ports {
		if usedIdx, found := usedVirtualPorts[port.VirtualPort]; found {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:  "ports",
				Index: idx,
				Wrapped: resource.ErrInvalidField{
					Name: "virtual_port",
					Wrapped: errVirtualPortReused{
						Index: usedIdx,
						Value: port.VirtualPort,
					},
				},
			})
		} else if port.VirtualPort != 0 {
			usedVirtualPorts[port.VirtualPort] = idx
		}

		// validate the target port
		if nameErr := ValidatePortName(port.TargetPort); nameErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:  "ports",
				Index: idx,
				Wrapped: resource.ErrInvalidField{
					Name:    "target_port",
					Wrapped: nameErr,
				},
			})
		}

		if protoErr := ValidateProtocol(port.Protocol); protoErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:  "ports",
				Index: idx,
				Wrapped: resource.ErrInvalidField{
					Name:    "protocol",
					Wrapped: protoErr,
				},
			})
		}

		// validate the virtual port is within the allowed range - 0 is allowed
		// to signify that no virtual port should be used and the port will not
		// be available for transparent proxying within the mesh.
		if portErr := ValidateVirtualPort(port.VirtualPort); portErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:  "ports",
				Index: idx,
				Wrapped: resource.ErrInvalidField{
					Name:    "virtual_port",
					Wrapped: errInvalidVirtualPort,
				},
			})
		}

		// basic protobuf deserialization should enforce that only known variants of the protocol field are set.
	}

	// Validate that the Virtual IPs are all IP addresses
	for idx, vip := range res.Data.VirtualIps {
		if vipErr := validateIPAddress(vip); vipErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "virtual_ips",
				Index:   idx,
				Wrapped: vipErr,
			})
		}
	}

	return err
}
