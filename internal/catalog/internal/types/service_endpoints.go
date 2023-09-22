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

const (
	ServiceEndpointsKind = "ServiceEndpoints"
)

var (
	ServiceEndpointsV2Beta1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV2Beta1,
		Kind:         ServiceEndpointsKind,
	}

	ServiceEndpointsType = ServiceEndpointsV2Beta1Type
)

func RegisterServiceEndpoints(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     ServiceEndpointsV2Beta1Type,
		Proto:    &pbcatalog.ServiceEndpoints{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateServiceEndpoints,
		Mutate:   MutateServiceEndpoints,
	})
}

func MutateServiceEndpoints(res *pbresource.Resource) error {
	if res.Owner == nil {
		res.Owner = &pbresource.ID{
			Type:    ServiceV2Beta1Type,
			Tenancy: res.Id.Tenancy,
			Name:    res.Id.Name,
		}
	}

	return nil
}

func ValidateServiceEndpoints(res *pbresource.Resource) error {
	var svcEndpoints pbcatalog.ServiceEndpoints

	if err := res.Data.UnmarshalTo(&svcEndpoints); err != nil {
		return resource.NewErrDataParse(&svcEndpoints, err)
	}

	var err error
	if !resource.EqualType(res.Owner.Type, ServiceV2Beta1Type) {
		err = multierror.Append(err, resource.ErrOwnerTypeInvalid{
			ResourceType: ServiceEndpointsV2Beta1Type,
			OwnerType:    res.Owner.Type,
		})
	}

	if !resource.EqualTenancy(res.Owner.Tenancy, res.Id.Tenancy) {
		err = multierror.Append(err, resource.ErrOwnerTenantInvalid{
			ResourceType:    ServiceEndpointsV2Beta1Type,
			ResourceTenancy: res.Id.Tenancy,
			OwnerTenancy:    res.Owner.Tenancy,
		})
	}

	if res.Owner.Name != res.Id.Name {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name: "name",
			Wrapped: errInvalidEndpointsOwnerName{
				Name:      res.Id.Name,
				OwnerName: res.Owner.Name,
			},
		})
	}

	for idx, endpoint := range svcEndpoints.Endpoints {
		if endpointErr := validateEndpoint(endpoint, res); endpointErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "endpoints",
				Index:   idx,
				Wrapped: endpointErr,
			})
		}
	}

	return err
}

func validateEndpoint(endpoint *pbcatalog.Endpoint, res *pbresource.Resource) error {
	var err error

	// Validate the target ref if not nil. When it is nil we are assuming that
	// the endpoints are being managed for an external service that has no
	// corresponding workloads that Consul has knowledge of.
	if endpoint.TargetRef != nil {
		// Validate the target reference
		if refErr := validateReference(WorkloadType, res.Id.GetTenancy(), endpoint.TargetRef); refErr != nil {
			err = multierror.Append(err, resource.ErrInvalidField{
				Name:    "target_ref",
				Wrapped: refErr,
			})
		}
	}

	// Validate the endpoint Addresses
	for addrIdx, addr := range endpoint.Addresses {
		if addrErr := validateWorkloadAddress(addr, endpoint.Ports); addrErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "addresses",
				Index:   addrIdx,
				Wrapped: addrErr,
			})
		}
	}

	// Ensure that the endpoint has at least 1 port.
	if len(endpoint.Ports) < 1 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "ports",
			Wrapped: resource.ErrEmpty,
		})
	}

	if healthErr := validateHealth(endpoint.HealthStatus); healthErr != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "health_status",
			Wrapped: healthErr,
		})
	}

	// Validate the endpoints ports
	for portName, port := range endpoint.Ports {
		// Port names must be DNS labels
		if portNameErr := validatePortName(portName); portNameErr != nil {
			err = multierror.Append(err, resource.ErrInvalidMapKey{
				Map:     "ports",
				Key:     portName,
				Wrapped: portNameErr,
			})
		}

		if protoErr := validateProtocol(port.Protocol); protoErr != nil {
			err = multierror.Append(err, resource.ErrInvalidMapValue{
				Map: "ports",
				Key: portName,
				Wrapped: resource.ErrInvalidField{
					Name:    "protocol",
					Wrapped: protoErr,
				},
			})
		}

		// As the physical port is the real port the endpoint will be bound to
		// it must be in the standard 1-65535 range.
		if port.Port < 1 || port.Port > math.MaxUint16 {
			err = multierror.Append(err, resource.ErrInvalidMapValue{
				Map: "ports",
				Key: portName,
				Wrapped: resource.ErrInvalidField{
					Name:    "physical_port",
					Wrapped: errInvalidPhysicalPort,
				},
			})
		}
	}

	return err
}
