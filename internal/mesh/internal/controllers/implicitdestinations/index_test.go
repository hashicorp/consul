// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package implicitdestinations

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestGetSourceWorkloadIdentitiesFromCTP(t *testing.T) {
	registry := resource.NewRegistry()
	types.Register(registry)
	auth.RegisterTypes(registry)
	catalog.RegisterTypes(registry)

	type testcase struct {
		ctp                     *types.DecodedComputedTrafficPermissions
		expectExact             []*pbresource.Reference
		expectWildNameInNS      []*pbresource.Tenancy
		expectWildNSInPartition []string
	}

	run := func(t *testing.T, tc testcase) {
		expectExactMap := make(map[resource.ReferenceKey]*pbresource.Reference)
		for _, ref := range tc.expectExact {
			rk := resource.NewReferenceKey(ref)
			expectExactMap[rk] = ref
		}

		gotExact, gotWildNameInNS, gotWildNSInPartition := getSourceWorkloadIdentitiesFromCTP(tc.ctp)
		prototest.AssertDeepEqual(t, expectExactMap, gotExact)
		prototest.AssertElementsMatch(t, tc.expectWildNameInNS, gotWildNameInNS)
		require.ElementsMatch(t, tc.expectWildNSInPartition, gotWildNSInPartition)
	}

	tenancy := resource.DefaultNamespacedTenancy()

	ctpID := &pbresource.ID{
		Type:    pbauth.ComputedTrafficPermissionsType,
		Tenancy: tenancy,
		Name:    "ctp1",
	}

	newRef := func(name string) *pbresource.Reference {
		return &pbresource.Reference{
			Type:    pbauth.WorkloadIdentityType,
			Tenancy: tenancy,
			Name:    name,
		}
	}

	cases := map[string]testcase{
		"empty": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID),
		},
		"single include": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{{
						Sources: []*pbauth.Source{
							{IdentityName: "foo"},
						},
					}},
				},
			),
			expectExact: []*pbresource.Reference{
				newRef("foo"),
			},
		},
		"multiple includes (1)": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{{
						Sources: []*pbauth.Source{
							{IdentityName: "foo"},
							{IdentityName: "bar"},
						},
					}},
				},
			),
			expectExact: []*pbresource.Reference{
				newRef("foo"),
				newRef("bar"),
			},
		},
		"multiple includes (2)": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{
						{Sources: []*pbauth.Source{{IdentityName: "foo"}}},
						{Sources: []*pbauth.Source{{IdentityName: "bar"}}},
					},
				},
			),
			expectExact: []*pbresource.Reference{
				newRef("foo"),
				newRef("bar"),
			},
		},
		"default ns wildcard (1) / excludes ignored": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{{
						Sources: []*pbauth.Source{{
							Exclude: []*pbauth.ExcludeSource{{
								IdentityName: "bar",
							}},
						}},
					}},
				},
			),
			expectWildNSInPartition: []string{"default"},
		},
		"default ns wildcard (2)": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{{
						Sources: []*pbauth.Source{
							{Partition: "default"},
						},
					}},
				},
			),
			expectWildNSInPartition: []string{"default"},
		},
		"multiple ns wildcards": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{{
						Sources: []*pbauth.Source{
							{Partition: "foo"},
							{Partition: "bar"},
						},
					}},
				},
			),
			expectWildNSInPartition: []string{"bar", "foo"},
		},
		"multiple ns wildcards deduped": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{{
						Sources: []*pbauth.Source{
							{Partition: "bar"},
							{Partition: "bar"},
						},
					}},
				},
			),
			expectWildNSInPartition: []string{"bar"},
		},
		"name wildcard": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{{
						Sources: []*pbauth.Source{
							{Partition: "default", Namespace: "zim"},
						},
					}},
				},
			),
			expectWildNameInNS: []*pbresource.Tenancy{
				{Partition: "default", Namespace: "zim"},
			},
		},
		"multiple name wildcards": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{{
						Sources: []*pbauth.Source{
							{Partition: "foo", Namespace: "zim"},
							{Partition: "bar", Namespace: "gir"},
						},
					}},
				},
			),
			expectWildNameInNS: []*pbresource.Tenancy{
				{Partition: "foo", Namespace: "zim"},
				{Partition: "bar", Namespace: "gir"},
			},
		},
		"multiple name wildcards deduped": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{{
						Sources: []*pbauth.Source{
							{Partition: "foo", Namespace: "zim"},
							{Partition: "foo", Namespace: "zim"},
						},
					}},
				},
			),
			expectWildNameInNS: []*pbresource.Tenancy{
				{Partition: "foo", Namespace: "zim"},
			},
		},
		"some of each": {
			ctp: ReconcileComputedTrafficPermissions(t, nil, ctpID,
				&pbauth.TrafficPermissions{
					Action: pbauth.Action_ACTION_ALLOW,
					Permissions: []*pbauth.Permission{
						{
							Sources: []*pbauth.Source{
								{Partition: "foo", Namespace: "zim"},
								{Partition: "bar", Namespace: "gir"},
								{IdentityName: "dib"},
							},
						},
						{
							Sources: []*pbauth.Source{
								{Partition: "foo"},
								{Partition: "bar"},
								{IdentityName: "gaz"},
							},
						},
					},
				},
			),
			expectWildNameInNS: []*pbresource.Tenancy{
				{Partition: "foo", Namespace: "zim"},
				{Partition: "bar", Namespace: "gir"},
			},
			expectWildNSInPartition: []string{"bar", "foo"},
			expectExact: []*pbresource.Reference{
				newRef("dib"),
				newRef("gaz"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
