// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	defaultNodeHealthStatusOwnerTenancy = &pbresource.Tenancy{
		Partition: "default",
	}

	defaultNodeHealthStatusOwner = &pbresource.ID{
		Type:    pbcatalog.NodeType,
		Tenancy: defaultNodeHealthStatusOwnerTenancy,
		Name:    "foo",
	}
)

func createNodeHealthStatusResource(t *testing.T, data protoreflect.ProtoMessage, owner *pbresource.ID) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: pbcatalog.NodeHealthStatusType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
			},
			Name: "test-status",
		},
		Owner: owner,
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestValidateNodeHealthStatus_Ok(t *testing.T) {
	data := &pbcatalog.NodeHealthStatus{
		Type:        "tcp",
		Status:      pbcatalog.Health_HEALTH_PASSING,
		Description: "Doesn't matter as this is user settable",
		Output:      "Health check executors are free to use this field",
	}

	type testCase struct {
		owner *pbresource.ID
	}

	cases := map[string]testCase{
		"node-owned": {
			owner: &pbresource.ID{
				Type:    pbcatalog.NodeType,
				Tenancy: defaultNodeHealthStatusOwnerTenancy,
				Name:    "bar-node",
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			res := createNodeHealthStatusResource(t, data, tcase.owner)
			err := ValidateNodeHealthStatus(res)
			require.NoError(t, err)
		})
	}
}

func TestValidateNodeHealthStatus_ParseError(t *testing.T) {
	// Any type other than the NodeHealthStatus type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createNodeHealthStatusResource(t, data, defaultNodeHealthStatusOwner)

	err := ValidateNodeHealthStatus(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateNodeHealthStatus_InvalidHealth(t *testing.T) {
	// while this is a valid enum value it is not allowed to be used
	// as the Status field.
	data := &pbcatalog.NodeHealthStatus{
		Type:   "tcp",
		Status: pbcatalog.Health_HEALTH_ANY,
	}

	res := createNodeHealthStatusResource(t, data, defaultNodeHealthStatusOwner)

	err := ValidateNodeHealthStatus(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "status",
		Wrapped: errInvalidHealth,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateNodeHealthStatus_MissingType(t *testing.T) {
	data := &pbcatalog.NodeHealthStatus{
		Status: pbcatalog.Health_HEALTH_PASSING,
	}

	res := createNodeHealthStatusResource(t, data, defaultNodeHealthStatusOwner)

	err := ValidateNodeHealthStatus(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "type",
		Wrapped: resource.ErrMissing,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateNodeHealthStatus_MissingOwner(t *testing.T) {
	data := &pbcatalog.NodeHealthStatus{
		Type:   "tcp",
		Status: pbcatalog.Health_HEALTH_PASSING,
	}

	res := createNodeHealthStatusResource(t, data, nil)

	err := ValidateNodeHealthStatus(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "owner",
		Wrapped: resource.ErrMissing,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateNodeHealthStatus_InvalidOwner(t *testing.T) {
	data := &pbcatalog.NodeHealthStatus{
		Type:   "tcp",
		Status: pbcatalog.Health_HEALTH_PASSING,
	}

	type testCase struct {
		owner *pbresource.ID
	}

	cases := map[string]testCase{
		"group-mismatch": {
			owner: &pbresource.ID{
				Type: &pbresource.Type{
					Group:        "fake",
					GroupVersion: pbcatalog.Version,
					Kind:         pbcatalog.NodeKind,
				},
				Tenancy: defaultNodeHealthStatusOwnerTenancy,
				Name:    "baz",
			},
		},
		"group-version-mismatch": {
			owner: &pbresource.ID{
				Type: &pbresource.Type{
					Group:        pbcatalog.GroupName,
					GroupVersion: "v99",
					Kind:         pbcatalog.NodeKind,
				},
				Tenancy: defaultNodeHealthStatusOwnerTenancy,
				Name:    "baz",
			},
		},
		"kind-mismatch": {
			owner: &pbresource.ID{
				Type:    pbcatalog.ServiceType,
				Tenancy: defaultNodeHealthStatusOwnerTenancy,
				Name:    "baz",
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			res := createNodeHealthStatusResource(t, data, tcase.owner)
			err := ValidateNodeHealthStatus(res)
			require.Error(t, err)
			expected := resource.ErrOwnerTypeInvalid{
				ResourceType: pbcatalog.NodeHealthStatusType,
				OwnerType:    tcase.owner.Type,
			}
			var actual resource.ErrOwnerTypeInvalid
			require.ErrorAs(t, err, &actual)
			require.Equal(t, expected, actual)
		})
	}
}

func TestNodeHealthStatusACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	node := resourcetest.Resource(pbcatalog.NodeType, "test").ID()

	nodehealthStatusData := &pbcatalog.NodeHealthStatus{
		Type:   "tcp",
		Status: pbcatalog.Health_HEALTH_PASSING,
	}

	cases := map[string]resourcetest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Data:    nodehealthStatusData,
			Owner:   node,
			Typ:     pbcatalog.NodeHealthStatusType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test read with node owner": {
			Rules:   `service "test" { policy = "read" }`,
			Data:    nodehealthStatusData,
			Owner:   node,
			Typ:     pbcatalog.NodeHealthStatusType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test write with node owner": {
			Rules:   `service "test" { policy = "write" }`,
			Data:    nodehealthStatusData,
			Owner:   node,
			Typ:     pbcatalog.NodeHealthStatusType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"node test read with node owner": {
			Rules:   `node "test" { policy = "read" }`,
			Data:    nodehealthStatusData,
			Owner:   node,
			Typ:     pbcatalog.NodeHealthStatusType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"node test write with node owner": {
			Rules:   `node "test" { policy = "write" }`,
			Data:    nodehealthStatusData,
			Owner:   node,
			Typ:     pbcatalog.NodeHealthStatusType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}
}
