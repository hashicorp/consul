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
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestGetWorkloadIdentitiesFromService(t *testing.T) {
	tenancy := resource.DefaultNamespacedTenancy()

	build := func(conds ...*pbresource.Condition) *pbresource.Resource {
		b := rtest.Resource(pbcatalog.ServiceType, "web").
			WithTenancy(tenancy).
			WithData(t, &pbcatalog.Service{})
		if len(conds) > 0 {
			b.WithStatus(catalog.EndpointsStatusKey, &pbresource.Status{
				Conditions: conds,
			})
		}
		return b.Build()
	}

	fooRef := &pbresource.Reference{
		Type:    pbauth.WorkloadIdentityType,
		Tenancy: tenancy,
		Name:    "foo",
	}
	barRef := &pbresource.Reference{
		Type:    pbauth.WorkloadIdentityType,
		Tenancy: tenancy,
		Name:    "bar",
	}

	makeRefs := func(refs ...*pbresource.Reference) []*pbresource.Reference {
		return refs
	}

	run := getWorkloadIdentitiesFromService

	require.Empty(t, run(build(nil)))
	require.Empty(t, run(build(&pbresource.Condition{
		Type:    catalog.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Message: "",
	})))
	prototest.AssertDeepEqual(t, makeRefs(fooRef), run(build(&pbresource.Condition{
		Type:    catalog.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Message: "foo",
	})))
	require.Empty(t, run(build(&pbresource.Condition{
		Type:    catalog.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_FALSE,
		Message: "foo",
	})))
	prototest.AssertDeepEqual(t, makeRefs(barRef, fooRef), run(build(&pbresource.Condition{
		Type:    catalog.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Message: "bar,foo", // proper order
	})))
	prototest.AssertDeepEqual(t, makeRefs(barRef, fooRef), run(build(&pbresource.Condition{
		Type:    catalog.StatusConditionBoundIdentities,
		State:   pbresource.Condition_STATE_TRUE,
		Message: "foo,bar", // incorrect order gets fixed
	})))
}

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

func TestGetBackendServiceRefsFromComputedRoutes(t *testing.T) {
	type testcase struct {
		cr     *types.DecodedComputedRoutes
		expect []*pbresource.Reference
	}

	run := func(t *testing.T, tc testcase) {
		got := getBackendServiceRefsFromComputedRoutes(tc.cr)
		prototest.AssertElementsMatch(t, tc.expect, got)
	}

	tenancy := resource.DefaultNamespacedTenancy()

	newRef := func(name string) *pbresource.Reference {
		return &pbresource.Reference{
			Type:    pbcatalog.ServiceType,
			Tenancy: tenancy,
			Name:    name,
		}
	}

	cr1 := resourcetest.Resource(pbmesh.ComputedRoutesType, "cr1").
		WithTenancy(tenancy).
		WithData(t, &pbmesh.ComputedRoutes{
			PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
				"http": {
					Targets: map[string]*pbmesh.BackendTargetDetails{
						"opaque1": {
							BackendRef: &pbmesh.BackendReference{Ref: newRef("aaa")},
						},
					},
				},
			},
		}).
		Build()

	cr2 := resourcetest.Resource(pbmesh.ComputedRoutesType, "cr2").
		WithTenancy(tenancy).
		WithData(t, &pbmesh.ComputedRoutes{
			PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
				"http": {
					Targets: map[string]*pbmesh.BackendTargetDetails{
						"opaque1": {
							BackendRef: &pbmesh.BackendReference{Ref: newRef("aaa")},
						},
						"opaque2": {
							BackendRef: &pbmesh.BackendReference{Ref: newRef("bbb")},
						},
					},
				},
				"grpc": {
					Targets: map[string]*pbmesh.BackendTargetDetails{
						"opaque2": {
							BackendRef: &pbmesh.BackendReference{Ref: newRef("bbb")},
						},
						"opaque3": {
							BackendRef: &pbmesh.BackendReference{Ref: newRef("ccc")},
						},
					},
				},
			},
		}).
		Build()

	cases := map[string]testcase{
		"one": {
			cr: resourcetest.MustDecode[*pbmesh.ComputedRoutes](t, cr1),
			expect: []*pbresource.Reference{
				newRef("aaa"),
			},
		},
		"two": {
			cr: resourcetest.MustDecode[*pbmesh.ComputedRoutes](t, cr2),
			expect: []*pbresource.Reference{
				newRef("aaa"),
				newRef("bbb"),
				newRef("ccc"),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
