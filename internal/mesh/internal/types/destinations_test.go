// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	catalogtesthelpers "github.com/hashicorp/consul/internal/catalog/catalogtest/helpers"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMutateDestinations(t *testing.T) {
	type testcase struct {
		tenancy   *pbresource.Tenancy
		data      *pbmesh.Destinations
		expect    *pbmesh.Destinations
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.DestinationsType, "api").
			WithTenancy(tc.tenancy).
			WithData(t, tc.data).
			Build()

		err := MutateDestinations(res)

		got := resourcetest.MustDecode[*pbmesh.Destinations](t, res)

		if tc.expectErr == "" {
			require.NoError(t, err)
			prototest.AssertDeepEqual(t, tc.expect, got.Data)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"empty-1": {
			data:   &pbmesh.Destinations{},
			expect: &pbmesh.Destinations{},
		},
		"invalid/nil dest ref": {
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: nil},
				},
			},
			expect: &pbmesh.Destinations{ // untouched
				Destinations: []*pbmesh.Destination{
					{DestinationRef: nil},
				},
			},
		},
		"dest ref tenancy defaulting": {
			tenancy: resourcetest.Tenancy("foo.bar"),
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "", "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, ".zim", "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "gir.zim", "api")},
				},
			},
			expect: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "foo.zim", "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "gir.zim", "api")},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestValidateUpstreams(t *testing.T) {
	type testcase struct {
		data       *pbmesh.Destinations
		skipMutate bool
		expectErr  string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.DestinationsType, "api").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, tc.data).
			Build()

		if !tc.skipMutate {
			require.NoError(t, MutateDestinations(res))

			// Verify that mutate didn't actually change the object.
			got := resourcetest.MustDecode[*pbmesh.Destinations](t, res)
			prototest.AssertDeepEqual(t, tc.data, got.Data)
		}

		err := ValidateDestinations(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbmesh.Destinations](t, res)
		prototest.AssertDeepEqual(t, tc.data, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		// emptiness
		"empty": {
			data:      &pbmesh.Destinations{},
			expectErr: `invalid "workloads" field: cannot be empty`,
		},
		"empty selector": {
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{},
			},
			expectErr: `invalid "workloads" field: cannot be empty`,
		},
		"bad selector": {
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{
					Names:  []string{"blah"},
					Filter: "garbage.foo == bar",
				},
			},
			expectErr: `invalid "filter" field: filter "garbage.foo == bar" is invalid: Selector "garbage" is not valid`,
		},
		"dest/nil ref": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"blah"},
				},
				Destinations: []*pbmesh.Destination{
					{DestinationRef: nil},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: missing required field`,
		},
		"dest/bad type": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"blah"},
				},
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.WorkloadType, "default.default", "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "type" field: reference must have type catalog.v2beta1.Service`,
		},
		"dest/nil tenancy": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"blah"},
				},
				Destinations: []*pbmesh.Destination{
					{DestinationRef: &pbresource.Reference{Type: pbcatalog.ServiceType, Name: "api"}},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "tenancy" field: missing required field`,
		},
		"dest/bad dest tenancy/partition": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"blah"},
				},
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, ".bar", "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "tenancy" field: invalid "partition" field: cannot be empty`,
		},
		"dest/bad dest tenancy/namespace": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"blah"},
				},
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "foo", "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "tenancy" field: invalid "namespace" field: cannot be empty`,
		},
		"dest/bad dest tenancy/peer_name": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"blah"},
				},
				Destinations: []*pbmesh.Destination{
					{DestinationRef: resourcetest.Resource(pbcatalog.ServiceType, "api").
						WithTenancy(&pbresource.Tenancy{Partition: "foo", Namespace: "bar"}).
						Reference("")},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "tenancy" field: invalid "peer_name" field: must be set to "local"`,
		},
		"normal": {
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: []string{"blah"},
				},
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "foo.zim", "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "gir.zim", "api")},
				},
			},
		},
		"normal with selector": {
			data: &pbmesh.Destinations{
				Workloads: &pbcatalog.WorkloadSelector{
					Names:  []string{"blah"},
					Filter: "metadata.foo == bar",
				},
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "foo.bar", "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "foo.zim", "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, "gir.zim", "api")},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestDestinationsACLs(t *testing.T) {
	catalogtesthelpers.RunWorkloadSelectingTypeACLsTests[*pbmesh.Destinations](t, pbmesh.DestinationsType,
		func(selector *pbcatalog.WorkloadSelector) *pbmesh.Destinations {
			return &pbmesh.Destinations{Workloads: selector}
		},
		func(registry resource.Registry) {
			RegisterDestinations(registry)
		})
}
