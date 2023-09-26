// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"math"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func RegisterService(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.ServiceType,
		Proto:    &pbcatalog.Service{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateService,
		Mutate:   MutateService,
	})
}

func MutateService(res *pbresource.Resource) error {
	var service pbcatalog.Service

	if err := res.Data.UnmarshalTo(&service); err != nil {
		return err
	}

	changed := false

	// Default service port protocols.
	for _, port := range service.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_UNSPECIFIED {
			port.Protocol = pbcatalog.Protocol_PROTOCOL_TCP
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return res.Data.MarshalFrom(&service)
}

func ValidateService(res *pbresource.Resource) error {
	var service pbcatalog.Service

	if err := res.Data.UnmarshalTo(&service); err != nil {
		return resource.NewErrDataParse(&service, err)
	}

	var err error

	// Validate the workload selector. We are allowing selectors with no
	// selection criteria as it will allow for users to manually manage
	// ServiceEndpoints objects for this service such as when desiring to
	// configure endpoint information for external services that are not
	// registered as workloads
	if selErr := validateSelector(service.Workloads, true); selErr != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "workloads",
			Wrapped: selErr,
		})
	}

	usedVirtualPorts := make(map[uint32]int)

	// Validate each port
	for idx, port := range service.Ports {
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
		if nameErr := validatePortName(port.TargetPort); nameErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:  "ports",
				Index: idx,
				Wrapped: resource.ErrInvalidField{
					Name:    "target_port",
					Wrapped: nameErr,
				},
			})
		}

		if protoErr := validateProtocol(port.Protocol); protoErr != nil {
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
		if port.VirtualPort > math.MaxUint16 {
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
	for idx, vip := range service.VirtualIps {
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
