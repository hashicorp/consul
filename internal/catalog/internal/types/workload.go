// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"math"
	"sort"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	WorkloadKind = "Workload"
)

var (
	WorkloadV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         WorkloadKind,
	}

	WorkloadType = WorkloadV1Alpha1Type
)

func RegisterWorkload(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     WorkloadV1Alpha1Type,
		Proto:    &pbcatalog.Workload{},
		Validate: ValidateWorkload,
		Scope:    resource.ScopeNamespace,
	})
}

func ValidateWorkload(res *pbresource.Resource) error {
	var workload pbcatalog.Workload

	if err := res.Data.UnmarshalTo(&workload); err != nil {
		return resource.NewErrDataParse(&workload, err)
	}

	var err error

	// Validate that the workload has at least one port
	if len(workload.Ports) < 1 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "ports",
			Wrapped: resource.ErrEmpty,
		})
	}

	var meshPorts []string

	// Validate the Workload Ports
	for portName, port := range workload.Ports {
		if portNameErr := validatePortName(portName); portNameErr != nil {
			err = multierror.Append(err, resource.ErrInvalidMapKey{
				Map:     "ports",
				Key:     portName,
				Wrapped: portNameErr,
			})
		}

		// disallow port 0 for now
		if port.Port < 1 || port.Port > math.MaxUint16 {
			err = multierror.Append(err, resource.ErrInvalidMapValue{
				Map: "ports",
				Key: portName,
				Wrapped: resource.ErrInvalidField{
					Name:    "port",
					Wrapped: errInvalidPhysicalPort,
				},
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

		// Collect the list of mesh ports
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			meshPorts = append(meshPorts, portName)
		}
	}

	if len(meshPorts) > 1 {
		sort.Strings(meshPorts)
		err = multierror.Append(err, resource.ErrInvalidField{
			Name: "ports",
			Wrapped: errTooMuchMesh{
				Ports: meshPorts,
			},
		})
	}

	// If the workload is mesh enabled then a valid identity must be provided.
	// If not mesh enabled but a non-empty identity is provided then we still
	// validate that its valid.
	if len(meshPorts) > 0 && workload.Identity == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "identity",
			Wrapped: resource.ErrMissing,
		})
	} else if workload.Identity != "" && !isValidDNSLabel(workload.Identity) {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "identity",
			Wrapped: errNotDNSLabel,
		})
	}

	// Validate workload locality
	if workload.Locality != nil && workload.Locality.Region == "" && workload.Locality.Zone != "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "locality",
			Wrapped: errLocalityZoneNoRegion,
		})
	}

	// Node associations are optional but if present the name should
	// be a valid DNS label.
	if workload.NodeName != "" {
		if !isValidDNSLabel(workload.NodeName) {
			err = multierror.Append(err, resource.ErrInvalidField{
				Name:    "node_name",
				Wrapped: errNotDNSLabel,
			})
		}
	}

	if len(workload.Addresses) < 1 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "addresses",
			Wrapped: resource.ErrEmpty,
		})
	}

	// Validate Workload Addresses
	for idx, addr := range workload.Addresses {
		if addrErr := validateWorkloadAddress(addr, workload.Ports); addrErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "addresses",
				Index:   idx,
				Wrapped: addrErr,
			})
		}
	}

	return err
}
