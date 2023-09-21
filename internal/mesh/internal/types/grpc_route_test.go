// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMutateGRPCRoute(t *testing.T) {
	type testcase struct {
		routeTenancy *pbresource.Tenancy
		route        *pbmesh.GRPCRoute
		expect       *pbmesh.GRPCRoute
	}

	cases := map[string]testcase{}

	// Add common parent refs test cases.
	for name, parentTC := range getXRouteParentRefMutateTestCases() {
		cases["parent-ref: "+name] = testcase{
			routeTenancy: parentTC.routeTenancy,
			route: &pbmesh.GRPCRoute{
				ParentRefs: parentTC.refs,
			},
			expect: &pbmesh.GRPCRoute{
				ParentRefs: parentTC.expect,
			},
		}
	}
	// add common backend ref test cases.
	for name, backendTC := range getXRouteBackendRefMutateTestCases() {
		var (
			refs   []*pbmesh.GRPCBackendRef
			expect []*pbmesh.GRPCBackendRef
		)
		for _, br := range backendTC.refs {
			refs = append(refs, &pbmesh.GRPCBackendRef{
				BackendRef: br,
			})
		}
		for _, br := range backendTC.expect {
			expect = append(expect, &pbmesh.GRPCBackendRef{
				BackendRef: br,
			})
		}
		cases["backend-ref: "+name] = testcase{
			routeTenancy: backendTC.routeTenancy,
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{
					{BackendRefs: refs},
				},
			},
			expect: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{
					{BackendRefs: expect},
				},
			},
		}
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.GRPCRouteType, "api").
			WithTenancy(tc.routeTenancy).
			WithData(t, tc.route).
			Build()

		err := MutateGRPCRoute(res)
		require.NoError(t, err)

		got := resourcetest.MustDecode[*pbmesh.GRPCRoute](t, res)

		if tc.expect == nil {
			tc.expect = proto.Clone(tc.route).(*pbmesh.GRPCRoute)
		}

		prototest.AssertDeepEqual(t, tc.expect, got.Data)
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestValidateGRPCRoute(t *testing.T) {
	type testcase struct {
		route     *pbmesh.GRPCRoute
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(pbmesh.GRPCRouteType, "api").
			WithData(t, tc.route).
			Build()

		// Ensure things are properly mutated and updated in the inputs.
		err := MutateGRPCRoute(res)
		require.NoError(t, err)
		{
			mutated := resourcetest.MustDecode[*pbmesh.GRPCRoute](t, res)
			tc.route = mutated.Data
		}

		err = ValidateGRPCRoute(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[*pbmesh.GRPCRoute](t, res)
		prototest.AssertDeepEqual(t, tc.route, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"hostnames not supported for services": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Hostnames: []string{"foo.local"},
			},
			expectErr: `invalid "hostnames" field: should not populate hostnames`,
		},
		"no rules": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
			},
		},
		"rules with no matches": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"rules with matches that are empty": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						// none
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"method match with no type is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Method: &pbmesh.GRPCMethodMatch{
							Service: "foo",
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid "type" field: missing required field`,
		},
		"method match with unknown type is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Method: &pbmesh.GRPCMethodMatch{
							Type:    99,
							Service: "foo",
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid "type" field: not a supported enum value: 99`,
		},
		"method match with no service nor method is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Method: &pbmesh.GRPCMethodMatch{
							Type: pbmesh.GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT,
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid "service" field: at least one of "service" or "method" must be set`,
		},
		"method match is good (1)": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Method: &pbmesh.GRPCMethodMatch{
							Type:    pbmesh.GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT,
							Service: "foo",
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"method match is good (2)": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Method: &pbmesh.GRPCMethodMatch{
							Type:   pbmesh.GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT,
							Method: "bar",
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"method match is good (3)": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Method: &pbmesh.GRPCMethodMatch{
							Type:    pbmesh.GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT,
							Service: "foo",
							Method:  "bar",
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"header match with no type is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Headers: []*pbmesh.GRPCHeaderMatch{{
							Name: "x-foo",
						}},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "headers": invalid "type" field: missing required field`,
		},
		"header match with unknown type is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Headers: []*pbmesh.GRPCHeaderMatch{{
							Type: 99,
							Name: "x-foo",
						}},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "headers": invalid "type" field: not a supported enum value: 99`,
		},
		"header match with no name is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Headers: []*pbmesh.GRPCHeaderMatch{{
							Type: pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
						}},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "headers": invalid "name" field: missing required field`,
		},
		"header match is good": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Matches: []*pbmesh.GRPCRouteMatch{{
						Headers: []*pbmesh.GRPCHeaderMatch{{
							Type: pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
							Name: "x-foo",
						}},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"filter empty is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Filters: []*pbmesh.GRPCRouteFilter{{
						// none
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"filter req header mod is ok": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Filters: []*pbmesh.GRPCRouteFilter{{
						RequestHeaderModifier: &pbmesh.HTTPHeaderFilter{},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"filter resp header mod is ok": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Filters: []*pbmesh.GRPCRouteFilter{{
						ResponseHeaderModifier: &pbmesh.HTTPHeaderFilter{},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"filter rewrite header mod missing path prefix": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Filters: []*pbmesh.GRPCRouteFilter{{
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": invalid "url_rewrite" field: invalid "path_prefix" field: field should not be empty if enclosing section is set`,
		},
		"filter rewrite header mod is ok": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Filters: []*pbmesh.GRPCRouteFilter{{
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{
							PathPrefix: "/blah",
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"filter req+resp header mod is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Filters: []*pbmesh.GRPCRouteFilter{{
						RequestHeaderModifier:  &pbmesh.HTTPHeaderFilter{},
						ResponseHeaderModifier: &pbmesh.HTTPHeaderFilter{},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"filter req+rewrite header mod is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Filters: []*pbmesh.GRPCRouteFilter{{
						RequestHeaderModifier: &pbmesh.HTTPHeaderFilter{},
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{
							PathPrefix: "/blah",
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"filter resp+rewrite header mod is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Filters: []*pbmesh.GRPCRouteFilter{{
						ResponseHeaderModifier: &pbmesh.HTTPHeaderFilter{},
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{
							PathPrefix: "/blah",
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"filter req+resp+rewrite header mod is bad": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Filters: []*pbmesh.GRPCRouteFilter{{
						RequestHeaderModifier:  &pbmesh.HTTPHeaderFilter{},
						ResponseHeaderModifier: &pbmesh.HTTPHeaderFilter{},
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{
							PathPrefix: "/blah",
						},
					}},
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"backend ref with filters is unsupported": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
						Filters: []*pbmesh.GRPCRouteFilter{{
							RequestHeaderModifier: &pbmesh.HTTPHeaderFilter{},
						}},
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "backend_refs": invalid "filters" field: filters are not supported at this level yet`,
		},
		"nil backend ref": {
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					BackendRefs: []*pbmesh.GRPCBackendRef{nil},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "backend_refs": invalid "backend_ref" field: missing required field`,
		},
	}

	// Add common timeouts test cases.
	for name, timeoutsTC := range getXRouteTimeoutsTestCases() {
		cases["timeouts: "+name] = testcase{
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Timeouts: timeoutsTC.timeouts,
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: timeoutsTC.expectErr,
		}
	}

	// Add common retries test cases.
	for name, retriesTC := range getXRouteRetriesTestCases() {
		cases["retries: "+name] = testcase{
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{{
					Retries: retriesTC.retries,
					BackendRefs: []*pbmesh.GRPCBackendRef{{
						BackendRef: newBackendRef(pbcatalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: retriesTC.expectErr,
		}
	}

	// Add common parent refs test cases.
	for name, parentTC := range getXRouteParentRefTestCases() {
		cases["parent-ref: "+name] = testcase{
			route: &pbmesh.GRPCRoute{
				ParentRefs: parentTC.refs,
			},
			expectErr: parentTC.expectErr,
		}
	}
	// add common backend ref test cases.
	for name, backendTC := range getXRouteBackendRefTestCases() {
		var refs []*pbmesh.GRPCBackendRef
		for _, br := range backendTC.refs {
			refs = append(refs, &pbmesh.GRPCBackendRef{
				BackendRef: br,
			})
		}
		cases["backend-ref: "+name] = testcase{
			route: &pbmesh.GRPCRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(pbcatalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.GRPCRouteRule{
					{BackendRefs: refs},
				},
			},
			expectErr: backendTC.expectErr,
		}
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestGRPCRouteACLs(t *testing.T) {
	testXRouteACLs[*pbmesh.GRPCRoute](t, func(t *testing.T, parentRefs, backendRefs []*pbresource.Reference) *pbresource.Resource {
		data := &pbmesh.GRPCRoute{
			ParentRefs: nil,
		}
		for _, ref := range parentRefs {
			data.ParentRefs = append(data.ParentRefs, &pbmesh.ParentReference{
				Ref: ref,
			})
		}

		var ruleRefs []*pbmesh.GRPCBackendRef
		for _, ref := range backendRefs {
			ruleRefs = append(ruleRefs, &pbmesh.GRPCBackendRef{
				BackendRef: &pbmesh.BackendReference{
					Ref: ref,
				},
			})
		}
		data.Rules = []*pbmesh.GRPCRouteRule{
			{BackendRefs: ruleRefs},
		}

		return resourcetest.Resource(pbmesh.GRPCRouteType, "api-grpc-route").
			WithTenancy(resource.DefaultNamespacedTenancy()).
			WithData(t, data).
			Build()
	})
}
