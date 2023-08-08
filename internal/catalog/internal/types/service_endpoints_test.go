// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
)

var (
	defaultEndpointTenancy = &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
		PeerName:  "local",
	}
)

func TestValidateServiceEndpoints_Ok(t *testing.T) {
	data := &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: &pbresource.ID{
					Type:    WorkloadType,
					Tenancy: defaultEndpointTenancy,
					Name:    "foo",
				},
				Addresses: []*pbcatalog.WorkloadAddress{
					{
						Host: "198.18.0.1",
					},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"foo": {
						Port:     8443,
						Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
				HealthStatus: pbcatalog.Health_HEALTH_PASSING,
			},
		},
	}

	res := resourcetest.Resource(ServiceEndpointsType, "test-service").WithData(t, data).Build()

	err := ValidateServiceEndpoints(res)
	require.NoError(t, err)
}

func TestValidateServiceEndpoints_ParseError(t *testing.T) {
	// Any type other than the ServiceEndpoints type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := resourcetest.Resource(ServiceEndpointsType, "test-service").WithData(t, data).Build()

	err := ValidateServiceEndpoints(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateServiceEndpoints_EndpointInvalid(t *testing.T) {
	genData := func() *pbcatalog.Endpoint {
		return &pbcatalog.Endpoint{
			TargetRef: &pbresource.ID{
				Type:    WorkloadType,
				Tenancy: defaultEndpointTenancy,
				Name:    "foo",
			},
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host:  "198.18.0.1",
					Ports: []string{"foo"},
				},
			},
			Ports: map[string]*pbcatalog.WorkloadPort{
				"foo": {
					Port: 42,
				},
			},
			HealthStatus: pbcatalog.Health_HEALTH_PASSING,
		}
	}

	type testCase struct {
		modify      func(*pbcatalog.Endpoint)
		validateErr func(t *testing.T, err error)
	}

	cases := map[string]testCase{
		"invalid-target": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.TargetRef.Type = NodeType
			},
			validateErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, resource.ErrInvalidReferenceType{AllowedType: WorkloadType})
			},
		},
		"invalid-address": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.Addresses[0].Ports = []string{"bar"}
			},
			validateErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errInvalidPortReference{Name: "bar"})
			},
		},
		"no-ports": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.Ports = nil
			},
			validateErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, resource.ErrEmpty)
			},
		},
		"invalid-port-name": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.Ports[""] = &pbcatalog.WorkloadPort{
					Port: 42,
				}
			},
			validateErr: func(t *testing.T, err error) {
				var mapErr resource.ErrInvalidMapKey
				require.ErrorAs(t, err, &mapErr)
				require.Equal(t, "ports", mapErr.Map)
				require.Equal(t, "", mapErr.Key)
				require.Equal(t, resource.ErrEmpty, mapErr.Wrapped)
			},
		},
		"port-0": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.Ports["foo"].Port = 0
			},
			validateErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errInvalidPhysicalPort)
			},
		},
		"port-out-of-range": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.Ports["foo"].Port = 65536
			},
			validateErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, errInvalidPhysicalPort)
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			endpoint := genData()
			tcase.modify(endpoint)

			data := &pbcatalog.ServiceEndpoints{
				Endpoints: []*pbcatalog.Endpoint{
					endpoint,
				},
			}
			res := resourcetest.Resource(ServiceEndpointsType, "test-service").WithData(t, data).Build()

			err := ValidateServiceEndpoints(res)
			require.Error(t, err)
			tcase.validateErr(t, err)
		})
	}
}

func TestMutateServiceEndpoints_PopulateOwner(t *testing.T) {
	res := resourcetest.Resource(ServiceEndpointsType, "test-service").Build()

	require.NoError(t, MutateServiceEndpoints(res))
	require.NotNil(t, res.Owner)
	require.True(t, resource.EqualType(res.Owner.Type, ServiceType))
	require.True(t, resource.EqualTenancy(res.Owner.Tenancy, defaultEndpointTenancy))
	require.Equal(t, res.Owner.Name, res.Id.Name)
}

func TestMutateServiceEndpoints_InvalidOwner(t *testing.T) {
	badTenancy := &pbresource.Tenancy{Partition: "default", Namespace: "other", PeerName: "local"}
	res := resourcetest.Resource(ServiceEndpointsType, "test-service").
		WithTenancy(defaultEndpointTenancy).
		WithOwner(&pbresource.ID{
			Type:    DNSPolicyType,
			Tenancy: badTenancy,
			Name:    "wrong"}).
		Build()

	err := MutateServiceEndpoints(res)
	require.Error(t, err)

	resourcetest.RequireError(t, err, resource.ErrOwnerTypeInvalid{ResourceType: ServiceEndpointsType, OwnerType: DNSPolicyType})
	resourcetest.RequireError(t, err, resource.ErrOwnerTenantInvalid{
		ResourceType:    ServiceEndpointsType,
		ResourceTenancy: defaultEndpointTenancy,
		OwnerTenancy:    badTenancy,
	})
	resourcetest.RequireError(t, err, resource.ErrInvalidField{
		Name: "name",
		Wrapped: errInvalidEndpointsOwnerName{
			Name:      "test-service",
			OwnerName: "wrong"},
	})
}
