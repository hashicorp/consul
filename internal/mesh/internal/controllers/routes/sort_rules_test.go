// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestGammaInitialSortWrappedRoutes(t *testing.T) {
	// These generations were created with ulid.Make().String() at 1 second
	// intervals. They are in ascending order.
	generations := []string{
		"01H8HGKHXCAJY7TRNJ06JGEZP9",
		"01H8HGKJWMEYWKZATG44QZF0G7",
		"01H8HGKKVW2339KFHAFXMEFRHP",
		"01H8HGKMV45XAC2W1KCQ7KJ00J",
		"01H8HGKNTCB3ZYN16AZ2KSXN54",
		"01H8HGKPSN9V3QQXTQVZ1EQM2T",
		"01H8HGKQRXMR8NY662AC520CDE",
		"01H8HGKRR5C41RHGXY7H3N0JYX",
		"01H8HGKSQDNSQ54VN86SBTR149",
		"01H8HGKTPPRWFXWHKV90M2WW5R",
	}
	require.True(t, sort.StringsAreSorted(generations))

	nodeID := func(node *inputRouteNode) string {
		return resource.IDToString(node.OriginalResource.Id) + ":" + node.OriginalResource.Generation
	}

	nodeIDSlice := func(nodes []*inputRouteNode) []string {
		var out []string
		for _, node := range nodes {
			out = append(out, nodeID(node))
		}
		return out
	}

	// newNode will only populate the fields that the sorting function cares
	// about.
	newNode := func(tenancy *pbresource.Tenancy, name string, gen string) *inputRouteNode {
		id := rtest.Resource(types.HTTPRouteType, name).
			WithTenancy(tenancy).
			ID()

		res := rtest.ResourceID(id).
			WithGeneration(gen).
			WithData(t, &pbmesh.HTTPRoute{}).
			Build()

		return &inputRouteNode{
			OriginalResource: res,
		}
	}

	type testcase struct {
		routes []*inputRouteNode
	}

	run := func(t *testing.T, tc testcase) {
		expect := nodeIDSlice(tc.routes)

		in := tc.routes

		if len(in) > 1 {
			// Randomly permute it
			for {
				rand.Shuffle(len(in), func(i, j int) {
					in[i], in[j] = in[j], in[i]
				})
				curr := nodeIDSlice(tc.routes)

				if slices.Equal(expect, curr) {
					// Loop until the shuffle was actually different.
				} else {
					break
				}

			}
		}

		gammaInitialSortWrappedRoutes(in)

		prototest.AssertDeepEqual(t, tc.routes, in)
	}

	// Order:
	// 1. generation older first
	// 2. tenancy namespace A first
	// 3. object name A first
	cases := map[string]testcase{
		"empty": {},
		"one": {
			routes: []*inputRouteNode{
				newNode(defaultTenancy(), "foo", generations[0]),
			},
		},
		"two: by generation": {
			routes: []*inputRouteNode{
				newNode(defaultTenancy(), "foo", generations[0]),
				newNode(defaultTenancy(), "foo", generations[1]),
			},
		},
		"two: by namespace": {
			routes: []*inputRouteNode{
				newNode(&pbresource.Tenancy{Namespace: "aaa"}, "foo", generations[0]),
				newNode(&pbresource.Tenancy{Namespace: "bbb"}, "foo", generations[0]),
			},
		},
		"two: by name": {
			routes: []*inputRouteNode{
				newNode(defaultTenancy(), "bar", generations[0]),
				newNode(defaultTenancy(), "foo", generations[0]),
			},
		},
		"two: by name with empty namespace": {
			routes: []*inputRouteNode{
				newNode(nsTenancy(""), "bar", generations[0]),
				newNode(nsTenancy(""), "foo", generations[0]),
			},
		},
		"four: by generation": {
			routes: []*inputRouteNode{
				newNode(defaultTenancy(), "foo", generations[0]),
				newNode(defaultTenancy(), "foo", generations[1]),
				newNode(defaultTenancy(), "foo", generations[2]),
				newNode(defaultTenancy(), "foo", generations[3]),
			},
		},
		"four: by name with some empty namespaces": {
			routes: []*inputRouteNode{
				newNode(nsTenancy("aaa"), "foo", generations[0]),
				newNode(nsTenancy("bbb"), "foo", generations[0]),
				newNode(&pbresource.Tenancy{}, "bar", generations[0]),
				newNode(&pbresource.Tenancy{}, "foo", generations[0]),
			},
		},
		"mixed": {
			// Seed this with data such that each later sort criteria should
			// want to be more to the top than an earlier criteria would allow,
			// to prove that the sort is going to have to shake out the way the
			// algorithm wants it to for maximum algorithm exercise.
			routes: []*inputRouteNode{
				// gen beats name
				newNode(defaultTenancy(), "zzz", generations[0]),
				newNode(defaultTenancy(), "aaa", generations[1]),
				// gen beats ns
				newNode(nsTenancy("zzz"), "foo", generations[2]),
				newNode(nsTenancy("aaa"), "foo", generations[3]),
				// ns beats name
				newNode(nsTenancy("aaa"), "zzz", generations[4]),
				newNode(nsTenancy("bbb"), "aaa", generations[5]),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestGammaSortHTTPRouteRules(t *testing.T) {
	type testcase struct {
		rules []*pbmesh.ComputedHTTPRouteRule
	}

	run := func(t *testing.T, tc testcase) {
		dup := protoSliceClone(tc.rules)

		gammaSortHTTPRouteRules(dup)

		prototest.AssertDeepEqual(t, tc.rules, dup)
	}

	cases := map[string]testcase{
		//
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func newID(typ *pbresource.Type, tenancy *pbresource.Tenancy, name string) *pbresource.ID {
	return rtest.Resource(typ, name).
		WithTenancy(tenancy).
		ID()
}

func nsTenancy(ns string) *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Partition: "default",
		Namespace: ns,
		PeerName:  "local",
	}
}

func defaultTenancy() *pbresource.Tenancy {
	return nsTenancy("default")
}
