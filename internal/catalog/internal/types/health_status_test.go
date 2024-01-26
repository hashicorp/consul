// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var (
	defaultHealthStatusOwnerTenancy = &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
	}

	defaultHealthStatusOwner = &pbresource.ID{
		Type:    pbcatalog.WorkloadType,
		Tenancy: defaultHealthStatusOwnerTenancy,
		Name:    "foo",
	}
)

func createHealthStatusResource(t *testing.T, data protoreflect.ProtoMessage, owner *pbresource.ID) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: pbcatalog.HealthStatusType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
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

func TestValidateHealthStatus_Ok(t *testing.T) {
	data := &pbcatalog.HealthStatus{
		Type:        "tcp",
		Status:      pbcatalog.Health_HEALTH_PASSING,
		Description: "Doesn't matter as this is user settable",
		Output:      "Health check executors are free to use this field",
	}

	type testCase struct {
		owner *pbresource.ID
	}

	cases := map[string]testCase{
		"workload-owned": {
			owner: &pbresource.ID{
				Type:    pbcatalog.WorkloadType,
				Tenancy: defaultHealthStatusOwnerTenancy,
				Name:    "foo-workload",
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			res := createHealthStatusResource(t, data, tcase.owner)
			err := ValidateHealthStatus(res)
			require.NoError(t, err)
		})
	}
}

func TestValidateHealthStatus_ParseError(t *testing.T) {
	// Any type other than the HealthStatus type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createHealthStatusResource(t, data, defaultHealthStatusOwner)

	err := ValidateHealthStatus(res)
	require.Error(t, err)
	require.ErrorAs(t, err, &resource.ErrDataParse{})
}

func TestValidateHealthStatus_InvalidHealth(t *testing.T) {
	// while this is a valid enum value it is not allowed to be used
	// as the Status field.
	data := &pbcatalog.HealthStatus{
		Type:   "tcp",
		Status: pbcatalog.Health_HEALTH_ANY,
	}

	res := createHealthStatusResource(t, data, defaultHealthStatusOwner)

	err := ValidateHealthStatus(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "status",
		Wrapped: errInvalidHealth,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateHealthStatus_MissingType(t *testing.T) {
	data := &pbcatalog.HealthStatus{
		Status: pbcatalog.Health_HEALTH_PASSING,
	}

	res := createHealthStatusResource(t, data, defaultHealthStatusOwner)

	err := ValidateHealthStatus(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "type",
		Wrapped: resource.ErrMissing,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateHealthStatus_MissingOwner(t *testing.T) {
	data := &pbcatalog.HealthStatus{
		Type:   "tcp",
		Status: pbcatalog.Health_HEALTH_PASSING,
	}

	res := createHealthStatusResource(t, data, nil)

	err := ValidateHealthStatus(res)
	require.Error(t, err)
	expected := resource.ErrInvalidField{
		Name:    "owner",
		Wrapped: resource.ErrMissing,
	}
	var actual resource.ErrInvalidField
	require.ErrorAs(t, err, &actual)
	require.Equal(t, expected, actual)
}

func TestValidateHealthStatus_InvalidOwner(t *testing.T) {
	data := &pbcatalog.HealthStatus{
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
					Kind:         pbcatalog.WorkloadKind,
				},
				Tenancy: defaultHealthStatusOwnerTenancy,
				Name:    "baz",
			},
		},
		"group-version-mismatch": {
			owner: &pbresource.ID{
				Type: &pbresource.Type{
					Group:        pbcatalog.GroupName,
					GroupVersion: "v99",
					Kind:         pbcatalog.WorkloadKind,
				},
				Tenancy: defaultHealthStatusOwnerTenancy,
				Name:    "baz",
			},
		},
		"kind-mismatch": {
			owner: &pbresource.ID{
				Type:    pbcatalog.ServiceType,
				Tenancy: defaultHealthStatusOwnerTenancy,
				Name:    "baz",
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			res := createHealthStatusResource(t, data, tcase.owner)
			err := ValidateHealthStatus(res)
			require.Error(t, err)
			expected := resource.ErrOwnerTypeInvalid{
				ResourceType: pbcatalog.HealthStatusType,
				OwnerType:    tcase.owner.Type,
			}
			var actual resource.ErrOwnerTypeInvalid
			require.ErrorAs(t, err, &actual)
			require.Equal(t, expected, actual)
		})
	}
}

func TestHealthStatusACLs(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	workload := resourcetest.Resource(pbcatalog.WorkloadType, "test").ID()

	healthStatusData := &pbcatalog.HealthStatus{
		Type:   "tcp",
		Status: pbcatalog.Health_HEALTH_PASSING,
	}

	cases := map[string]resourcetest.ACLTestCase{
		"no rules": {
			Rules:   ``,
			Data:    healthStatusData,
			Owner:   workload,
			Typ:     pbcatalog.HealthStatusType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test read": {
			Rules:   `service "test" { policy = "read" }`,
			Data:    healthStatusData,
			Owner:   workload,
			Typ:     pbcatalog.HealthStatusType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"service test write": {
			Rules:   `service "test" { policy = "write" }`,
			Data:    healthStatusData,
			Owner:   workload,
			Typ:     pbcatalog.HealthStatusType,
			ReadOK:  resourcetest.ALLOW,
			WriteOK: resourcetest.ALLOW,
			ListOK:  resourcetest.DEFAULT,
		},
		"node test read with workload owner": {
			Rules:   `node "test" { policy = "read" }`,
			Data:    healthStatusData,
			Owner:   workload,
			Typ:     pbcatalog.HealthStatusType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
		"node test write with workload owner": {
			Rules:   `node "test" { policy = "write" }`,
			Data:    healthStatusData,
			Owner:   workload,
			Typ:     pbcatalog.HealthStatusType,
			ReadOK:  resourcetest.DENY,
			WriteOK: resourcetest.DENY,
			ListOK:  resourcetest.DEFAULT,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resourcetest.RunACLTestCase(t, tc, registry)
		})
	}
}
