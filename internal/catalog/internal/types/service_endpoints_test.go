// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	defaultEndpointTenancy = &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
		PeerName:  "local",
	}
)

func createServiceEndpointsResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    ServiceEndpointsType,
			Tenancy: defaultEndpointTenancy,
			Name:    "test-service",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

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

	res := createServiceEndpointsResource(t, data)

	err := ValidateServiceEndpoints(res)
	require.NoError(t, err)
}

func TestValidateServiceEndpoints_ParseError(t *testing.T) {
	// Any type other than the ServiceEndpoints type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createServiceEndpointsResource(t, data)

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
			data := genData()
			tcase.modify(data)

			res := createServiceEndpointsResource(t, &pbcatalog.ServiceEndpoints{
				Endpoints: []*pbcatalog.Endpoint{
					data,
				},
			})

			err := ValidateServiceEndpoints(res)
			require.Error(t, err)
			tcase.validateErr(t, err)
		})
	}
}
