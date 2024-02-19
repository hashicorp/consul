// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
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
		id := rtest.Resource(pbmesh.HTTPRouteType, name).
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

		if len(tc.routes) > 1 {
			// Randomly permute it
			in := tc.routes
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

		gammaInitialSortWrappedRoutes(tc.routes)

		got := nodeIDSlice(tc.routes)

		require.Equal(t, expect, got)
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

	// In this test we will use the 'backend target' field to track the rule
	// identity to make assertions easy.

	targetSlice := func(rules []*pbmesh.ComputedHTTPRouteRule) []string {
		var out []string
		for _, rule := range rules {
			out = append(out, rule.BackendRefs[0].BackendTarget)
		}
		return out
	}

	run := func(t *testing.T, tc testcase) {
		expect := targetSlice(tc.rules)

		if len(tc.rules) > 1 {
			// Randomly permute it
			in := tc.rules
			for {
				rand.Shuffle(len(in), func(i, j int) {
					in[i], in[j] = in[j], in[i]
				})
				curr := targetSlice(tc.rules)

				if slices.Equal(expect, curr) {
					// Loop until the shuffle was actually different.
				} else {
					break
				}

			}
		}

		gammaSortHTTPRouteRules(tc.rules)

		got := targetSlice(tc.rules)

		require.Equal(t, expect, got)
	}

	newRule := func(target string, m ...*pbmesh.HTTPRouteMatch) *pbmesh.ComputedHTTPRouteRule {
		return &pbmesh.ComputedHTTPRouteRule{
			Matches: m,
			BackendRefs: []*pbmesh.ComputedHTTPBackendRef{{
				BackendTarget: target,
			}},
		}
	}

	// Rules:
	// 1. exact path match exists
	// 2. prefix match exists
	// 3. prefix match has lots of characters
	// 4. has method match
	// 5. has lots of header matches
	// 6. has lots of query param matches
	cases := map[string]testcase{
		"empty": {},
		"one": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1", &pbmesh.HTTPRouteMatch{
					//
				}),
			},
		},
		"two: by exact path match exists": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1", &pbmesh.HTTPRouteMatch{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_EXACT,
						Value: "/",
					},
				}),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/",
					},
				}),
			},
		},
		"two: by prefix path match exists": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1", &pbmesh.HTTPRouteMatch{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/",
					},
				}),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_REGEX,
						Value: "/[a]",
					},
				}),
			},
		},
		"two: by prefix path match length": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1", &pbmesh.HTTPRouteMatch{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/longer",
					},
				}),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/short",
					},
				}),
			},
		},
		"two: by method match exists": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1", &pbmesh.HTTPRouteMatch{
					Method: "GET",
				}),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					Headers: []*pbmesh.HTTPHeaderMatch{{
						Type:  pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
						Name:  "x-blah",
						Value: "foo",
					}},
				}),
			},
		},
		"two: by header match quantity": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1", &pbmesh.HTTPRouteMatch{
					Headers: []*pbmesh.HTTPHeaderMatch{
						{
							Type:  pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
							Name:  "x-blah",
							Value: "foo",
						},
						{
							Type:  pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
							Name:  "x-other",
							Value: "bar",
						},
					},
				}),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					Headers: []*pbmesh.HTTPHeaderMatch{{
						Type:  pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
						Name:  "x-blah",
						Value: "foo",
					}},
				}),
			},
		},
		"two: by query param match quantity": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1", &pbmesh.HTTPRouteMatch{
					QueryParams: []*pbmesh.HTTPQueryParamMatch{
						{
							Type:  pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT,
							Name:  "foo",
							Value: "1",
						},
						{
							Type:  pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT,
							Name:  "bar",
							Value: "1",
						},
					},
				}),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					QueryParams: []*pbmesh.HTTPQueryParamMatch{{
						Type:  pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT,
						Name:  "foo",
						Value: "1",
					}},
				}),
			},
		},
		"mixed: has path exact beats has path prefix when both are present": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1",
					&pbmesh.HTTPRouteMatch{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_EXACT,
							Value: "/",
						},
					},
					&pbmesh.HTTPRouteMatch{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
							Value: "/short",
						},
					},
				),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/longer",
					},
				}),
			},
		},
		"mixed: longer path prefix beats shorter when both are present": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1",
					&pbmesh.HTTPRouteMatch{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
							Value: "/longer",
						},
					},
					&pbmesh.HTTPRouteMatch{
						Method: "GET",
					},
				),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					Path: &pbmesh.HTTPPathMatch{
						Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
						Value: "/short",
					},
				}),
			},
		},
		"mixed: has method match beats header match when both are present": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1",
					&pbmesh.HTTPRouteMatch{
						Method: "GET",
					},
					&pbmesh.HTTPRouteMatch{
						Headers: []*pbmesh.HTTPHeaderMatch{{
							Type:  pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
							Name:  "x-blah",
							Value: "foo",
						}},
					},
				),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					Headers: []*pbmesh.HTTPHeaderMatch{
						{
							Type:  pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
							Name:  "x-blah",
							Value: "foo",
						},
						{
							Type:  pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
							Name:  "x-other",
							Value: "bar",
						},
					},
				}),
			},
		},
		"mixed: header match beats query param match when both are present": {
			rules: []*pbmesh.ComputedHTTPRouteRule{
				newRule("r1", &pbmesh.HTTPRouteMatch{
					Headers: []*pbmesh.HTTPHeaderMatch{
						{
							Type:  pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
							Name:  "x-blah",
							Value: "foo",
						},
						{
							Type:  pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
							Name:  "x-other",
							Value: "bar",
						},
					},
					QueryParams: []*pbmesh.HTTPQueryParamMatch{{
						Type:  pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT,
						Name:  "foo",
						Value: "1",
					}},
				}),
				newRule("r2", &pbmesh.HTTPRouteMatch{
					QueryParams: []*pbmesh.HTTPQueryParamMatch{
						{
							Type:  pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT,
							Name:  "foo",
							Value: "1",
						},
						{
							Type:  pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT,
							Name:  "bar",
							Value: "1",
						},
					},
				}),
			},
		},
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
	}
}

func defaultTenancy() *pbresource.Tenancy {
	return nsTenancy("default")
}
