// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestMutateHTTPRoute(t *testing.T) {
	type testcase struct {
		route     *pbmesh.HTTPRoute
		expect    *pbmesh.HTTPRoute
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(HTTPRouteType, "api").
			WithData(t, tc.route).
			Build()

		err := MutateHTTPRoute(res)

		got := resourcetest.MustDecode[pbmesh.HTTPRoute, *pbmesh.HTTPRoute](t, res)

		if tc.expectErr == "" {
			require.NoError(t, err)

			if tc.expect == nil {
				tc.expect = proto.Clone(tc.route).(*pbmesh.HTTPRoute)
			}

			prototest.AssertDeepEqual(t, tc.expect, got.Data)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"no-rules": {
			route: &pbmesh.HTTPRoute{},
		},
		"rules-with-no-matches": {
			route: &pbmesh.HTTPRoute{
				Rules: []*pbmesh.HTTPRouteRule{{
					// none
				}},
			},
		},
		"rules-with-matches-no-methods": {
			route: &pbmesh.HTTPRoute{
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
							Value: "/foo",
						},
					}},
				}},
			},
		},
		"rules-with-matches-methods-uppercase": {
			route: &pbmesh.HTTPRoute{
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{
						{
							Path: &pbmesh.HTTPPathMatch{
								Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
								Value: "/foo",
							},
							Method: "GET",
						},
						{
							Path: &pbmesh.HTTPPathMatch{
								Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
								Value: "/bar",
							},
							Method: "POST",
						},
					},
				}},
			},
		},
		"rules-with-matches-methods-lowercase": {
			route: &pbmesh.HTTPRoute{
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{
						{
							Path: &pbmesh.HTTPPathMatch{
								Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
								Value: "/foo",
							},
							Method: "get",
						},
						{
							Path: &pbmesh.HTTPPathMatch{
								Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
								Value: "/bar",
							},
							Method: "post",
						},
					},
				}},
			},
			expect: &pbmesh.HTTPRoute{
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{
						{
							Path: &pbmesh.HTTPPathMatch{
								Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
								Value: "/foo",
							},
							Method: "GET",
						},
						{
							Path: &pbmesh.HTTPPathMatch{
								Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
								Value: "/bar",
							},
							Method: "POST",
						},
					},
				}},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestValidateHTTPRoute(t *testing.T) {
	type testcase struct {
		route     *pbmesh.HTTPRoute
		expectErr string
	}

	run := func(t *testing.T, tc testcase) {
		res := resourcetest.Resource(HTTPRouteType, "api").
			WithData(t, tc.route).
			Build()

		err := MutateHTTPRoute(res)
		require.NoError(t, err)

		err = ValidateHTTPRoute(res)

		// Verify that validate didn't actually change the object.
		got := resourcetest.MustDecode[pbmesh.HTTPRoute, *pbmesh.HTTPRoute](t, res)
		prototest.AssertDeepEqual(t, tc.route, got.Data)

		if tc.expectErr == "" {
			require.NoError(t, err)
		} else {
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"hostnames not supported for services": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Hostnames: []string{"foo.local"},
			},
			expectErr: `invalid "hostnames" field: should not populate hostnames`,
		},
		"no rules": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
			},
		},
		"rules with no matches": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"rules with matches that are empty": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						// none
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"path match with no type is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Path: &pbmesh.HTTPPathMatch{
							Value: "/foo",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid "path" field: invalid "type" field: missing required field`,
		},
		"exact path match with no leading slash is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_EXACT,
							Value: "foo",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid "path" field: invalid "value" field: exact patch value does not start with '/': "foo"`,
		},
		"prefix path match with no leading slash is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
							Value: "foo",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid "path" field: invalid "value" field: prefix patch value does not start with '/': "foo"`,
		},
		"exact path match with leading slash is good": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_EXACT,
							Value: "/foo",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"prefix path match with leading slash is good": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Path: &pbmesh.HTTPPathMatch{
							Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
							Value: "/foo",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"header match with no type is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Headers: []*pbmesh.HTTPHeaderMatch{{
							Name: "x-foo",
						}},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid element at index 0 of list "headers": invalid "type" field: missing required field`,
		},
		"header match with no name is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Headers: []*pbmesh.HTTPHeaderMatch{{
							Type: pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
						}},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid element at index 0 of list "headers": invalid "name" field: missing required field`,
		},
		"header match is good": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Headers: []*pbmesh.HTTPHeaderMatch{{
							Type: pbmesh.HeaderMatchType_HEADER_MATCH_TYPE_EXACT,
							Name: "x-foo",
						}},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"queryparam match with no type is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						QueryParams: []*pbmesh.HTTPQueryParamMatch{{
							Name: "x-foo",
						}},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid element at index 0 of list "query_params": invalid "type" field: missing required field`,
		},
		"queryparam match with no name is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						QueryParams: []*pbmesh.HTTPQueryParamMatch{{
							Type: pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT,
						}},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid element at index 0 of list "query_params": invalid "name" field: missing required field`,
		},
		"queryparam match is good": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						QueryParams: []*pbmesh.HTTPQueryParamMatch{{
							Type: pbmesh.QueryParamMatchType_QUERY_PARAM_MATCH_TYPE_EXACT,
							Name: "x-foo",
						}},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"method match is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Method: "BOB",
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "matches": invalid "method" field: not a valid http method: "BOB"`,
		},
		"method match is good": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Matches: []*pbmesh.HTTPRouteMatch{{
						Method: "DELETE",
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"filter empty is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Filters: []*pbmesh.HTTPRouteFilter{{
						// none
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"filter req header mod is ok": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Filters: []*pbmesh.HTTPRouteFilter{{
						RequestHeaderModifier: &pbmesh.HTTPHeaderFilter{},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"filter resp header mod is ok": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Filters: []*pbmesh.HTTPRouteFilter{{
						ResponseHeaderModifier: &pbmesh.HTTPHeaderFilter{},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"filter rewrite header mod missing path prefix": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Filters: []*pbmesh.HTTPRouteFilter{{
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": invalid "url_rewrite" field: invalid "path_prefix" field: missing required field`,
		},
		"filter rewrite header mod is ok": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Filters: []*pbmesh.HTTPRouteFilter{{
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{
							PathPrefix: "/blah",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"filter req+resp header mod is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Filters: []*pbmesh.HTTPRouteFilter{{
						RequestHeaderModifier:  &pbmesh.HTTPHeaderFilter{},
						ResponseHeaderModifier: &pbmesh.HTTPHeaderFilter{},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"filter req+rewrite header mod is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Filters: []*pbmesh.HTTPRouteFilter{{
						RequestHeaderModifier: &pbmesh.HTTPHeaderFilter{},
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{
							PathPrefix: "/blah",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"filter resp+rewrite header mod is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Filters: []*pbmesh.HTTPRouteFilter{{
						ResponseHeaderModifier: &pbmesh.HTTPHeaderFilter{},
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{
							PathPrefix: "/blah",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"filter req+resp+rewrite header mod is bad": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Filters: []*pbmesh.HTTPRouteFilter{{
						RequestHeaderModifier:  &pbmesh.HTTPHeaderFilter{},
						ResponseHeaderModifier: &pbmesh.HTTPHeaderFilter{},
						UrlRewrite: &pbmesh.HTTPURLRewriteFilter{
							PathPrefix: "/blah",
						},
					}},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 0 of list "filters": exactly one of request_header_modifier, response_header_modifier, or url_rewrite`,
		},
		"timeout: bad request": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Timeouts: &pbmesh.HTTPRouteTimeouts{
						Request: durationpb.New(-1 * time.Second),
					},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid "timeouts" field: invalid "request" field: timeout cannot be negative: -1s`,
		},
		"timeout: bad backend request": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Timeouts: &pbmesh.HTTPRouteTimeouts{
						BackendRequest: durationpb.New(-1 * time.Second),
					},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid "timeouts" field: invalid "backend_request" field: timeout cannot be negative: -1s`,
		},
		"timeout: bad idle": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Timeouts: &pbmesh.HTTPRouteTimeouts{
						Idle: durationpb.New(-1 * time.Second),
					},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid "timeouts" field: invalid "idle" field: timeout cannot be negative: -1s`,
		},
		"timeout: good all": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Timeouts: &pbmesh.HTTPRouteTimeouts{
						Request:        durationpb.New(1 * time.Second),
						BackendRequest: durationpb.New(2 * time.Second),
						Idle:           durationpb.New(3 * time.Second),
					},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},
		"retries: bad number": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Retries: &pbmesh.HTTPRouteRetries{
						Number: -5,
					},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid "retries" field: invalid "number" field: cannot be negative: -5`,
		},
		"retries: bad conditions": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Retries: &pbmesh.HTTPRouteRetries{
						OnConditions: []string{"garbage"},
					},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid "retries" field: invalid element at index 0 of list "on_conditions": not a valid retry condition: "garbage"`,
		},
		"retries: good all": {
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{{
					Retries: &pbmesh.HTTPRouteRetries{
						Number:       5,
						OnConditions: []string{"internal"},
					},
					BackendRefs: []*pbmesh.HTTPBackendRef{{
						BackendRef: newBackendRef(catalog.ServiceType, "api", ""),
					}},
				}},
			},
		},

		// retries: bad on_conditions
		// retries: good on_conditions

	}

	// Add common parent refs test cases.
	for name, parentTC := range getXRouteParentRefTestCases() {
		cases["parent-ref: "+name] = testcase{
			route: &pbmesh.HTTPRoute{
				ParentRefs: parentTC.refs,
			},
			expectErr: parentTC.expectErr,
		}
	}
	// add common backend ref test cases.
	for name, backendTC := range getXRouteBackendRefTestCases() {
		var refs []*pbmesh.HTTPBackendRef
		for _, br := range backendTC.refs {
			refs = append(refs, &pbmesh.HTTPBackendRef{
				BackendRef: br,
			})
		}
		cases["backend-ref: "+name] = testcase{
			route: &pbmesh.HTTPRoute{
				ParentRefs: []*pbmesh.ParentReference{
					newParentRef(catalog.ServiceType, "web", ""),
				},
				Rules: []*pbmesh.HTTPRouteRule{
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

type xRouteParentRefTestcase struct {
	refs      []*pbmesh.ParentReference
	expectErr string
}

func getXRouteParentRefTestCases() map[string]xRouteParentRefTestcase {
	return map[string]xRouteParentRefTestcase{
		"no parent refs": {
			expectErr: `invalid "parent_refs" field: cannot be empty`,
		},
		"parent ref with nil ref": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", ""),
				{
					Ref:  nil,
					Port: "http",
				},
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: missing required field`,
		},
		"parent ref with bad type ref": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", ""),
				newParentRef(catalog.WorkloadType, "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: reference must have type catalog.v1alpha1.Service`,
		},
		"parent ref with section": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", ""),
				{
					Ref:  resourcetest.Resource(catalog.ServiceType, "web").Reference("section2"),
					Port: "http",
				},
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: invalid "section" field: section not supported for service parent refs`,
		},
		"duplicate exact parents": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", "http"),
				newParentRef(catalog.ServiceType, "api", "http"),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: parent ref "catalog.v1alpha1.Service/default.local.default/api" for port "http" exists twice`,
		},
		"duplicate wild parents": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", ""),
				newParentRef(catalog.ServiceType, "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: parent ref "catalog.v1alpha1.Service/default.local.default/api" for wildcard port exists twice`,
		},
		"duplicate parents via exact+wild overlap": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", "http"),
				newParentRef(catalog.ServiceType, "api", ""),
			},
			expectErr: `invalid element at index 1 of list "parent_refs": invalid "ref" field: parent ref "catalog.v1alpha1.Service/default.local.default/api" for ports [http] covered by wildcard port already`,
		},
		"good single parent ref": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", "http"),
			},
		},
		"good muliple parent refs": {
			refs: []*pbmesh.ParentReference{
				newParentRef(catalog.ServiceType, "api", "http"),
				newParentRef(catalog.ServiceType, "web", ""),
			},
		},
	}
}

type xRouteBackendRefTestcase struct {
	refs      []*pbmesh.BackendReference
	expectErr string
}

func getXRouteBackendRefTestCases() map[string]xRouteBackendRefTestcase {
	return map[string]xRouteBackendRefTestcase{
		"no backend refs": {
			expectErr: `invalid "backend_refs" field: cannot be empty`,
		},
		"backend ref with nil ref": {
			refs: []*pbmesh.BackendReference{
				newBackendRef(catalog.ServiceType, "api", ""),
				{
					Ref:  nil,
					Port: "http",
				},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "ref" field: missing required field`,
		},
		"backend ref with bad type ref": {
			refs: []*pbmesh.BackendReference{
				newBackendRef(catalog.ServiceType, "api", ""),
				newBackendRef(catalog.WorkloadType, "api", ""),
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "ref" field: reference must have type catalog.v1alpha1.Service`,
		},
		"backend ref with section": {
			refs: []*pbmesh.BackendReference{
				newBackendRef(catalog.ServiceType, "api", ""),
				{
					Ref:  resourcetest.Resource(catalog.ServiceType, "web").Reference("section2"),
					Port: "http",
				},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "ref" field: invalid "section" field: section not supported for service backend refs`,
		},
		"backend ref with datacenter": {
			refs: []*pbmesh.BackendReference{
				newBackendRef(catalog.ServiceType, "api", ""),
				{
					Ref:        newRef(catalog.ServiceType, "db"),
					Port:       "http",
					Datacenter: "dc2",
				},
			},
			expectErr: `invalid element at index 0 of list "rules": invalid element at index 1 of list "backend_refs": invalid "backend_ref" field: invalid "datacenter" field: datacenter is not yet supported on backend refs`,
		},
		"good backend ref": {
			refs: []*pbmesh.BackendReference{
				newBackendRef(catalog.ServiceType, "api", ""),
				{
					Ref:  newRef(catalog.ServiceType, "db"),
					Port: "http",
				},
			},
		},
	}
}

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return resourcetest.Resource(typ, name).Reference("")
}

func newBackendRef(typ *pbresource.Type, name, port string) *pbmesh.BackendReference {
	return &pbmesh.BackendReference{
		Ref:  newRef(typ, name),
		Port: port,
	}
}

func newParentRef(typ *pbresource.Type, name, port string) *pbmesh.ParentReference {
	return &pbmesh.ParentReference{
		Ref:  newRef(typ, name),
		Port: port,
	}
}
