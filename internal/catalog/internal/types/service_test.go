// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/catalog/internal/testhelpers"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func createServiceResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: pbcatalog.ServiceType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
			},
			Name: "test-policy",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestMutateServicePorts(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Names: []string{"foo", "bar"},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "tcp",
				Protocol:   pbcatalog.Protocol_PROTOCOL_UNSPECIFIED,
			},
			{
				TargetPort: "http",
				Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
			},
		},
		VirtualIps: []string{"198.18.0.1"},
	}

	res := createServiceResource(t, data)

	err := MutateService(res)
	require.NoError(t, err)

	got := resourcetest.MustDecode[*pbcatalog.Service](t, res)

	require.Len(t, got.Data.Ports, 2)
	require.Equal(t, pbcatalog.Protocol_PROTOCOL_TCP, got.Data.Ports[0].Protocol)

	// Check that specified protocol is not mutated.
	require.Equal(t, data.Ports[1].Protocol, got.Data.Ports[1].Protocol)
}

func TestValidateService_Ok(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Names:    []string{"foo", "bar"},
			Prefixes: []string{"abc-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort:  "http-internal",
				VirtualPort: 42,
				Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
			},
			{
				TargetPort: "other",
				// leaving VirtualPort unset to verify that seeing
				// a zero virtual port multiple times is fine.
				Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2,
			},
			{
				TargetPort: "other2",
				// leaving VirtualPort unset to verify that seeing
				// a zero virtual port multiple times is fine.
				Protocol: pbcatalog.Protocol_PROTOCOL_GRPC,
			},
		},
		VirtualIps: []string{"198.18.0.1"},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.NoError(t, err)
}

func TestValidateService_ParseError(t *testing.T) {
	// Any type other than the Service type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateService_EmptySelector(t *testing.T) {
	data := &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "http-internal",
				Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
			},
		},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.NoError(t, err)
}

func TestValidateService_InvalidSelector(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Names: []string{""},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "http-internal",
				Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
			},
		},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	expected := resource.ErrInvalidListElement{
		Name:    "names",
		Index:   0,
		Wrapped: resource.ErrEmpty,
	}

	var actual resource.ErrInvalidListElement
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateService_InvalidTargetPort(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "",
			},
		},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.Error(t, err)
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, "target_port", actual.Name)
	require.Equal(t, resource.ErrEmpty, actual.Wrapped)
}

func TestValidateService_VirtualPortReused(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				VirtualPort: 42,
				TargetPort:  "foo",
			},
			{
				VirtualPort: 42,
				TargetPort:  "bar",
			},
		},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.Error(t, err)
	var actual errVirtualPortReused
	require.ErrorAs(t, err, &actual)
	require.EqualValues(t, 0, actual.Index)
	require.EqualValues(t, 42, actual.Value)
}

func TestValidateService_InvalidPortProtocol(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "foo",
				Protocol:   99,
			},
		},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)

	expected := resource.ErrInvalidListElement{
		Name:  "ports",
		Index: 0,
		Wrapped: resource.ErrInvalidField{
			Name:    "protocol",
			Wrapped: resource.NewConstError("not a supported enum value: 99"),
		},
	}

	var actual resource.ErrInvalidListElement
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateService_VirtualPortInvalid(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				VirtualPort: 100000,
				TargetPort:  "foo",
			},
		},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.Error(t, err)
	require.ErrorIs(t, err, errInvalidVirtualPort)
}

func TestValidateService_InvalidVIP(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "foo",
			},
		},
		VirtualIps: []string{"foo"},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.Error(t, err)
	require.ErrorIs(t, err, errNotIPAddress)
}

func TestServiceACLs(t *testing.T) {
	testhelpers.RunWorkloadSelectingTypeACLsTests[*pbcatalog.Service](t, pbcatalog.ServiceType,
		func(selector *pbcatalog.WorkloadSelector) *pbcatalog.Service {
			return &pbcatalog.Service{Workloads: selector}
		},
		RegisterService,
	)
}
