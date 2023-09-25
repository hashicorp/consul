// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMutateUpstreams(t *testing.T) {
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

		err := MutateUpstreams(res)

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
			tenancy: newTestTenancy("foo.bar"),
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy(""), "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy(".zim"), "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy("gir.zim"), "api")},
				},
			},
			expect: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy("foo.bar"), "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy("foo.zim"), "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy("gir.zim"), "api")},
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
			require.NoError(t, MutateUpstreams(res))

			// Verify that mutate didn't actually change the object.
			got := resourcetest.MustDecode[*pbmesh.Destinations](t, res)
			prototest.AssertDeepEqual(t, tc.data, got.Data)
		}

		err := ValidateUpstreams(res)

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
			data: &pbmesh.Destinations{},
		},
		"dest/nil ref": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: nil},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: missing required field`,
		},
		"dest/bad type": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.WorkloadType, nil, "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "type" field: reference must have type catalog.v2beta1.Service`,
		},
		"dest/nil tenancy": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: &pbresource.Reference{Type: pbcatalog.ServiceType, Name: "api"}},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "tenancy" field: missing required field`,
		},
		"dest/bad dest tenancy/partition": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy(".bar"), "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "tenancy" field: invalid "partition" field: cannot be empty`,
		},
		"dest/bad dest tenancy/namespace": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy("foo"), "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "tenancy" field: invalid "namespace" field: cannot be empty`,
		},
		"dest/bad dest tenancy/peer_name": {
			skipMutate: true,
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, &pbresource.Tenancy{Partition: "foo", Namespace: "bar"}, "api")},
				},
			},
			expectErr: `invalid element at index 0 of list "upstreams": invalid "destination_ref" field: invalid "tenancy" field: invalid "peer_name" field: must be set to "local"`,
		},
		"normal": {
			data: &pbmesh.Destinations{
				Destinations: []*pbmesh.Destination{
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy("foo.bar"), "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy("foo.zim"), "api")},
					{DestinationRef: newRefWithTenancy(pbcatalog.ServiceType, newTestTenancy("gir.zim"), "api")},
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
