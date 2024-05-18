// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func createWorkloadResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: pbcatalog.WorkloadType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
			},
			Name: "api-1234",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validWorkload() *pbcatalog.Workload {
	return &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "127.0.0.1",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http": {
				Port:     8443,
				Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2,
			},
		},
		NodeName: "foo",
		Identity: "api",
		Locality: &pbcatalog.Locality{
			Region: "us-east-1",
			Zone:   "1a",
		},
	}
}

func TestValidateWorkload_Ok(t *testing.T) {
	res := createWorkloadResource(t, validWorkload())

	err := ValidateWorkload(res)
	require.NoError(t, err)
}

func TestValidateWorkload_OkWithDNSPolicy(t *testing.T) {
	data := validWorkload()
	data.Dns = &pbcatalog.DNSPolicy{
		Weights: &pbcatalog.Weights{
			Passing: 3,
			Warning: 2,
		},
	}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.NoError(t, err)
}

func TestValidateWorkload_ParseError(t *testing.T) {
	// Any type other than the Workload type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateWorkload_EmptyAddresses(t *testing.T) {
	data := validWorkload()
	data.Addresses = nil

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "addresses",
		Wrapped: resource.ErrEmpty,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_InvalidAddress(t *testing.T) {
	data := validWorkload()
	data.Addresses[0].Host = "-not-a-host"

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "host",
		Wrapped: errInvalidWorkloadHostFormat{Host: "-not-a-host"},
	}

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_MissingIdentity(t *testing.T) {
	data := validWorkload()
	data.Ports["mesh"] = &pbcatalog.WorkloadPort{
		Port:     9090,
		Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
	}
	data.Identity = ""

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "identity",
		Wrapped: resource.ErrMissing,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_InvalidIdentity(t *testing.T) {
	data := validWorkload()
	data.Identity = "/foiujd"

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "identity",
		Wrapped: errNotDNSLabel,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_InvalidNodeName(t *testing.T) {
	data := validWorkload()
	data.NodeName = "/foiujd"

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "node_name",
		Wrapped: errNotDNSLabel,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_NoPorts(t *testing.T) {
	data := validWorkload()
	data.Ports = nil

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "ports",
		Wrapped: resource.ErrEmpty,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_TooMuchMesh(t *testing.T) {
	data := validWorkload()
	data.Ports["mesh1"] = &pbcatalog.WorkloadPort{
		Port:     9090,
		Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
	}
	data.Ports["mesh2"] = &pbcatalog.WorkloadPort{
		Port:     9091,
		Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
	}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name: "ports",
		Wrapped: errTooMuchMesh{
			Ports: []string{"mesh1", "mesh2"},
		},
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_InvalidPortName(t *testing.T) {
	data := validWorkload()
	data.Ports[""] = &pbcatalog.WorkloadPort{
		Port: 42,
	}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidMapKey{
		Map:     "ports",
		Key:     "",
		Wrapped: resource.ErrEmpty,
	}
	var actual resource.ErrInvalidMapKey
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_InvalidPortProtocol(t *testing.T) {
	data := validWorkload()
	data.Ports["foo"] = &pbcatalog.WorkloadPort{
		Port:     42,
		Protocol: 99,
	}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidMapValue{
		Map: "ports",
		Key: "foo",
		Wrapped: resource.ErrInvalidField{
			Name:    "protocol",
			Wrapped: resource.NewConstError("not a supported enum value: 99"),
		},
	}
	var actual resource.ErrInvalidMapValue
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_Port0(t *testing.T) {
	data := validWorkload()
	data.Ports["bar"] = &pbcatalog.WorkloadPort{Port: 0}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "port",
		Wrapped: errInvalidPhysicalPort,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_PortTooHigh(t *testing.T) {
	data := validWorkload()
	data.Ports["bar"] = &pbcatalog.WorkloadPort{Port: 65536}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "port",
		Wrapped: errInvalidPhysicalPort,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_Locality(t *testing.T) {
	data := validWorkload()
	data.Locality = &pbcatalog.Locality{
		Zone: "1a",
	}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "locality",
		Wrapped: errLocalityZoneNoRegion,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_DNSPolicy(t *testing.T) {
	data := validWorkload()
	data.Dns = &pbcatalog.DNSPolicy{
		Weights: &pbcatalog.Weights{
			Passing: 0, // Needs to be a positive integer
		},
	}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)

	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.ErrorIs(t, err, errDNSPassingWeightOutOfRange)
}

func TestWorkloadACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	cases := map[string]rtest.ACLTestCase{
		"no rules": {
			Rules: ``,
			Data: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "1.1.1.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
			},
			Typ:     pbcatalog.WorkloadType,
			ReadOK:  rtest.DENY,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DEFAULT,
		},
		"service test read": {
			Rules: `service "test" { policy = "read" }`,
			Data: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "1.1.1.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
			},
			Typ:     pbcatalog.WorkloadType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DEFAULT,
		},
		"service test write": {
			Rules: `service "test" { policy = "write" }`,
			Data: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "1.1.1.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
			},
			Typ:     pbcatalog.WorkloadType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.ALLOW,
			ListOK:  rtest.DEFAULT,
		},
		"service test write with node": {
			Rules: `service "test" { policy = "write" }`,
			Data: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "1.1.1.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
				NodeName: "test-node",
			},
			Typ:     pbcatalog.WorkloadType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DEFAULT,
		},
		"service test write with workload identity": {
			Rules: `service "test" { policy = "write" }`,
			Data: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "1.1.1.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
				Identity: "test-identity",
			},
			Typ:     pbcatalog.WorkloadType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DEFAULT,
		},
		"service test write with workload identity and node": {
			Rules: `service "test" { policy = "write" }`,
			Data: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "1.1.1.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
				NodeName: "test-node",
				Identity: "test-identity",
			},
			Typ:     pbcatalog.WorkloadType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.DENY,
			ListOK:  rtest.DEFAULT,
		},
		"service test write with node and node policy": {
			Rules: `service "test" { policy = "write" } node "test-node" { policy = "read" }`,
			Data: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "1.1.1.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
				NodeName: "test-node",
			},
			Typ:     pbcatalog.WorkloadType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.ALLOW,
			ListOK:  rtest.DEFAULT,
		},
		"service test write with workload identity and identity policy ": {
			Rules: `service "test" { policy = "write" } identity "test-identity" { policy = "read" }`,
			Data: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "1.1.1.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
				Identity: "test-identity",
			},
			Typ:     pbcatalog.WorkloadType,
			ReadOK:  rtest.ALLOW,
			WriteOK: rtest.ALLOW,
			ListOK:  rtest.DEFAULT,
		},
		"service test write with workload identity and node with both node and identity policy": {
			Rules: `service "test" { policy = "write" } identity "test-identity" { policy = "read" } node "test-node" { policy = "read" }`,
			Data: &pbcatalog.Workload{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "1.1.1.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"tcp": {Port: 8080},
				},
				NodeName: "test-node",
				Identity: "test-identity",
			},
			Typ:     pbcatalog.WorkloadType,
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
