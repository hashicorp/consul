// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	defaultEndpointTenancy = &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
	}

	badEndpointTenancy = &pbresource.Tenancy{
		Partition: "default",
		Namespace: "bad",
	}
)

func TestValidateServiceEndpoints_Ok(t *testing.T) {
	data := &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				TargetRef: &pbresource.ID{
					Type:    pbcatalog.WorkloadType,
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
				Dns: &pbcatalog.DNSPolicy{
					Weights: &pbcatalog.Weights{
						Passing: 3,
						Warning: 2,
					},
				},
			},
		},
	}

	res := rtest.Resource(pbcatalog.ServiceEndpointsType, "test-service").
		WithTenancy(defaultEndpointTenancy).
		WithData(t, data).
		Build()

	// fill in owner automatically
	require.NoError(t, MutateServiceEndpoints(res))

	// Now validate that everything is good.
	err := ValidateServiceEndpoints(res)
	require.NoError(t, err)
}

func TestValidateServiceEndpoints_ParseError(t *testing.T) {
	// Any type other than the ServiceEndpoints type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := rtest.Resource(pbcatalog.ServiceEndpointsType, "test-service").WithData(t, data).Build()

	err := ValidateServiceEndpoints(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateServiceEndpoints_EndpointInvalid(t *testing.T) {
	genData := func() *pbcatalog.Endpoint {
		return &pbcatalog.Endpoint{
			TargetRef: &pbresource.ID{
				Type:    pbcatalog.WorkloadType,
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
		owner       *pbresource.ID
		modify      func(*pbcatalog.Endpoint)
		validateErr func(t *testing.T, err error)
	}

	cases := map[string]testCase{
		"invalid-target": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.TargetRef.Type = pbcatalog.NodeType
			},
			validateErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, resource.ErrInvalidReferenceType{AllowedType: pbcatalog.WorkloadType})
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
		"invalid-health-status": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.Ports["foo"] = &pbcatalog.WorkloadPort{
					Port: 42,
				}
				endpoint.HealthStatus = 99
			},
			validateErr: func(t *testing.T, err error) {
				rtest.RequireError(t, err, resource.ErrInvalidField{
					Name:    "health_status",
					Wrapped: resource.NewConstError("not a supported enum value: 99"),
				})
			},
		},
		"invalid-port-name": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.Ports[""] = &pbcatalog.WorkloadPort{
					Port: 42,
				}
			},
			validateErr: func(t *testing.T, err error) {
				rtest.RequireError(t, err, resource.ErrInvalidMapKey{
					Map:     "ports",
					Key:     "",
					Wrapped: resource.ErrEmpty,
				})
			},
		},
		"invalid-port-protocol": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.Ports["foo"] = &pbcatalog.WorkloadPort{
					Port:     42,
					Protocol: 99,
				}
			},
			validateErr: func(t *testing.T, err error) {
				rtest.RequireError(t, err, resource.ErrInvalidMapValue{
					Map: "ports",
					Key: "foo",
					Wrapped: resource.ErrInvalidField{
						Name:    "protocol",
						Wrapped: resource.NewConstError("not a supported enum value: 99"),
					},
				})
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
		"invalid-owner": {
			owner: &pbresource.ID{
				Type:    pbcatalog.HealthStatusType,
				Tenancy: badEndpointTenancy,
				Name:    "wrong",
			},
			validateErr: func(t *testing.T, err error) {
				rtest.RequireError(t, err, resource.ErrOwnerTypeInvalid{
					ResourceType: pbcatalog.ServiceEndpointsType,
					OwnerType:    pbcatalog.HealthStatusType})
				rtest.RequireError(t, err, resource.ErrOwnerTenantInvalid{
					ResourceType:    pbcatalog.ServiceEndpointsType,
					ResourceTenancy: defaultEndpointTenancy,
					OwnerTenancy:    badEndpointTenancy,
				})
				rtest.RequireError(t, err, resource.ErrInvalidField{
					Name: "name",
					Wrapped: errInvalidEndpointsOwnerName{
						Name:      "test-service",
						OwnerName: "wrong"},
				})
			},
		},
		"dns-policy-invalid": {
			modify: func(endpoint *pbcatalog.Endpoint) {
				endpoint.Dns = &pbcatalog.DNSPolicy{
					Weights: &pbcatalog.Weights{
						Passing: 0,
					},
				}
			},
			validateErr: func(t *testing.T, err error) {
				var actual resource.ErrInvalidField
				require.ErrorAs(t, err, &actual)
				require.ErrorIs(t, err, errDNSPassingWeightOutOfRange)
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			endpoint := genData()
			if tcase.modify != nil {
				tcase.modify(endpoint)
			}

			data := &pbcatalog.ServiceEndpoints{
				Endpoints: []*pbcatalog.Endpoint{
					endpoint,
				},
			}
			res := rtest.Resource(pbcatalog.ServiceEndpointsType, "test-service").
				WithTenancy(defaultEndpointTenancy).
				WithOwner(tcase.owner).
				WithData(t, data).
				Build()

			// Run the mututation to setup defaults
			require.NoError(t, MutateServiceEndpoints(res))

			err := ValidateServiceEndpoints(res)
			require.Error(t, err)
			tcase.validateErr(t, err)
		})
	}
}

func TestMutateServiceEndpoints_PopulateOwner(t *testing.T) {
	res := rtest.Resource(pbcatalog.ServiceEndpointsType, "test-service").
		WithTenancy(defaultEndpointTenancy).
		Build()

	require.NoError(t, MutateServiceEndpoints(res))
	require.NotNil(t, res.Owner)
	require.True(t, resource.EqualType(res.Owner.Type, pbcatalog.ServiceType))
	require.True(t, resource.EqualTenancy(res.Owner.Tenancy, defaultEndpointTenancy))
	require.Equal(t, res.Owner.Name, res.Id.Name)
}

func TestServiceEndpointsACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	service := rtest.Resource(pbcatalog.ServiceType, "test").
		WithTenancy(resource.DefaultNamespacedTenancy()).ID()
	serviceEndpointsData := &pbcatalog.ServiceEndpoints{}
	cases := map[string]rtest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Data:    serviceEndpointsData,
			Owner:   service,
			Typ:     pbcatalog.ServiceEndpointsType,
			ReadOK:  rtest.DENY,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DEFAULT,
		},
		"service test read": {
			Rules:   `service "test" { policy = "read" }`,
			Data:    serviceEndpointsData,
			Owner:   service,
			Typ:     pbcatalog.ServiceEndpointsType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DEFAULT,
		},
		"service test write": {
			Rules:   `service "test" { policy = "write" }`,
			Data:    serviceEndpointsData,
			Owner:   service,
			Typ:     pbcatalog.ServiceEndpointsType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.ALLOW,
			ListOK:  rtest.DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rtest.RunACLTestCase(t, tc, registry)
		})
	}
}
