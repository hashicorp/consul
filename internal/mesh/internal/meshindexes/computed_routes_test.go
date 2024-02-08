// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package meshindexes

import (
	"testing"

	"github.com/stretchr/testify/require"

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

	run := GetWorkloadIdentitiesFromService

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

func TestGetBackendServiceRefsFromComputedRoutes(t *testing.T) {
	type testcase struct {
		cr     *types.DecodedComputedRoutes
		expect []*pbresource.Reference
	}

	run := func(t *testing.T, tc testcase) {
		got := GetBackendServiceRefsFromComputedRoutes(tc.cr)
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
