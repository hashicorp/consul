// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"math"
	"sort"

	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type DecodedWorkload = resource.DecodedResource[*pbcatalog.Workload]

func RegisterWorkload(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     pbcatalog.WorkloadType,
		Proto:    &pbcatalog.Workload{},
		Scope:    resource.ScopeNamespace,
		Validate: ValidateWorkload,
		ACLs: &resource.ACLHooks{
			Read:  aclReadHookWorkload,
			Write: resource.DecodeAndAuthorizeWrite(aclWriteHookWorkload),
			List:  resource.NoOpACLListHook,
		},
	})
}

var ValidateWorkload = resource.DecodeAndValidate(validateWorkload)

func validateWorkload(res *DecodedWorkload) error {
	var err error

	// Validate that the workload has at least one port
	if len(res.Data.Ports) < 1 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "ports",
			Wrapped: resource.ErrEmpty,
		})
	}

	var meshPorts []string

	// Validate the Workload Ports
	for portName, port := range res.Data.Ports {
		if portNameErr := ValidatePortName(portName); portNameErr != nil {
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

		if protoErr := ValidateProtocol(port.Protocol); protoErr != nil {
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
	if len(meshPorts) > 0 && res.Data.Identity == "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "identity",
			Wrapped: resource.ErrMissing,
		})
	} else if res.Data.Identity != "" && !isValidDNSLabel(res.Data.Identity) {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "identity",
			Wrapped: errNotDNSLabel,
		})
	}

	// Validate workload locality
	if res.Data.Locality != nil && res.Data.Locality.Region == "" && res.Data.Locality.Zone != "" {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "locality",
			Wrapped: errLocalityZoneNoRegion,
		})
	}

	// Node associations are optional but if present the name should
	// be a valid DNS label.
	if res.Data.NodeName != "" {
		if !isValidDNSLabel(res.Data.NodeName) {
			err = multierror.Append(err, resource.ErrInvalidField{
				Name:    "node_name",
				Wrapped: errNotDNSLabel,
			})
		}
	}

	if len(res.Data.Addresses) < 1 {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "addresses",
			Wrapped: resource.ErrEmpty,
		})
	}

	// Validate Workload Addresses
	for idx, addr := range res.Data.Addresses {
		if addrErr := validateWorkloadAddress(addr, res.Data.Ports); addrErr != nil {
			err = multierror.Append(err, resource.ErrInvalidListElement{
				Name:    "addresses",
				Index:   idx,
				Wrapped: addrErr,
			})
		}
	}

	// Validate DNS
	if dnsErr := validateDNSPolicy(res.Data.Dns); dnsErr != nil {
		err = multierror.Append(err, resource.ErrInvalidField{
			Name:    "dns_policy",
			Wrapped: dnsErr,
		})
	}

	return err
}

func aclReadHookWorkload(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, id *pbresource.ID, _ *pbresource.Resource) error {
	return authorizer.ToAllowAuthorizer().ServiceReadAllowed(id.GetName(), authzContext)
}

func aclWriteHookWorkload(authorizer acl.Authorizer, authzContext *acl.AuthorizerContext, res *DecodedWorkload) error {
	// First check service:write on the workload name.
	err := authorizer.ToAllowAuthorizer().ServiceWriteAllowed(res.GetId().GetName(), authzContext)
	if err != nil {
		return err
	}

	// Check node:read permissions if node is specified.
	if res.Data.GetNodeName() != "" {
		return authorizer.ToAllowAuthorizer().NodeReadAllowed(res.Data.GetNodeName(), authzContext)
	}

	// Check identity:read permissions if identity is specified.
	if res.Data.GetIdentity() != "" {
		return authorizer.ToAllowAuthorizer().IdentityReadAllowed(res.Data.GetIdentity(), authzContext)
	}

	return nil
}
