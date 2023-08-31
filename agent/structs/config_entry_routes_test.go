// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
)

func TestTCPRoute(t *testing.T) {
	t.Parallel()

	cases := map[string]configEntryTestcase{
		"multiple services": {
			entry: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "route-one",
				Services: []TCPService{{
					Name: "foo",
				}, {
					Name: "bar",
				}},
			},
			validateErr: "tcp-route currently only supports one service",
		},
		"normalize parent kind": {
			entry: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Services: []TCPService{{
					Name: "foo",
				}},
			},
			normalizeOnly: true,
			check: func(t *testing.T, entry ConfigEntry) {
				expectedParent := ResourceReference{
					Kind:           APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
				}
				route := entry.(*TCPRouteConfigEntry)
				require.Len(t, route.Parents, 1)
				require.Equal(t, expectedParent, route.Parents[0])
			},
		},
		"invalid parent kind": {
			entry: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Kind: "route",
					Name: "gateway",
				}},
			},
			validateErr: "unsupported parent kind",
		},
		"duplicate parents with no listener specified": {
			entry: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "route-two",
				Parents: []ResourceReference{
					{
						Kind: "api-gateway",
						Name: "gateway",
					},
					{
						Kind: "api-gateway",
						Name: "gateway",
					},
				},
			},
			validateErr: "route parents must be unique",
		},
		"duplicate parents with listener specified": {
			entry: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "route-two",
				Parents: []ResourceReference{
					{
						Kind:        "api-gateway",
						Name:        "gateway",
						SectionName: "same",
					},
					{
						Kind:        "api-gateway",
						Name:        "gateway",
						SectionName: "same",
					},
				},
			},
			validateErr: "route parents must be unique",
		},
		"almost duplicate parents with one not specifying a listener": {
			entry: &TCPRouteConfigEntry{
				Kind: TCPRoute,
				Name: "route-two",
				Parents: []ResourceReference{
					{
						Kind: "api-gateway",
						Name: "gateway",
					},
					{
						Kind:        "api-gateway",
						Name:        "gateway",
						SectionName: "same",
					},
				},
			},
			check: func(t *testing.T, entry ConfigEntry) {
				expectedParents := []ResourceReference{
					{
						Kind:           APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
					},
					{
						Kind:           APIGateway,
						Name:           "gateway",
						SectionName:    "same",
						EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
					},
				}
				route := entry.(*TCPRouteConfigEntry)
				require.Len(t, route.Parents, 2)
				require.Equal(t, expectedParents[0], route.Parents[0])
				require.Equal(t, expectedParents[1], route.Parents[1])
			},
		},
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}

func TestHTTPRoute(t *testing.T) {
	t.Parallel()

	cases := map[string]configEntryTestcase{
		"normalize parent kind": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-one",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
			},
			normalizeOnly: true,
			check: func(t *testing.T, entry ConfigEntry) {
				expectedParent := ResourceReference{
					Kind:           APIGateway,
					Name:           "gateway",
					EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
				}
				route := entry.(*HTTPRouteConfigEntry)
				require.Len(t, route.Parents, 1)
				require.Equal(t, expectedParent, route.Parents[0])
			},
		},
		"invalid parent kind": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Kind: "route",
					Name: "gateway",
				}},
			},
			validateErr: "unsupported parent kind",
		},
		"wildcard hostnames": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Hostnames: []string{"*"},
			},
			validateErr: "host \"*\" must not be a wildcard",
		},
		"wildcard subdomain": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Hostnames: []string{"*.consul.example"},
			},
			validateErr: "host \"*.consul.example\" must not be a wildcard",
		},
		"valid dns hostname": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Hostnames: []string{"...not legal"},
			},
			validateErr: "host \"...not legal\" must be a valid DNS hostname",
		},
		"rule matches invalid header match type": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Rules: []HTTPRouteRule{{
					Matches: []HTTPMatch{{
						Headers: []HTTPHeaderMatch{{
							Match: HTTPHeaderMatchType("foo"),
							Name:  "foo",
						}},
					}},
				}},
			},
			validateErr: "Rule[0], Match[0], Headers[0], match type should be one of present, exact, prefix, suffix, or regex",
		},
		"rule matches invalid header match name": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Rules: []HTTPRouteRule{{
					Matches: []HTTPMatch{{
						Headers: []HTTPHeaderMatch{{
							Match: HTTPHeaderMatchPresent,
						}},
					}},
				}},
			},
			validateErr: "Rule[0], Match[0], Headers[0], missing required Name field",
		},
		"rule matches invalid query match type": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Rules: []HTTPRouteRule{{
					Matches: []HTTPMatch{{
						Query: []HTTPQueryMatch{{
							Match: HTTPQueryMatchType("foo"),
							Name:  "foo",
						}},
					}},
				}},
			},
			validateErr: "Rule[0], Match[0], Query[0], match type should be one of present, exact, or regex",
		},
		"rule matches invalid query match name": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Rules: []HTTPRouteRule{{
					Matches: []HTTPMatch{{
						Query: []HTTPQueryMatch{{
							Match: HTTPQueryMatchPresent,
						}},
					}},
				}},
			},
			validateErr: "Rule[0], Match[0], Query[0], missing required Name field",
		},
		"rule matches invalid path match type": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Rules: []HTTPRouteRule{{
					Matches: []HTTPMatch{{
						Path: HTTPPathMatch{
							Match: HTTPPathMatchType("foo"),
						},
					}},
				}},
			},
			validateErr: "Rule[0], Match[0], Path, match type should be one of exact, prefix, or regex",
		},
		"rule matches invalid path match prefix": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Rules: []HTTPRouteRule{{
					Matches: []HTTPMatch{{
						Path: HTTPPathMatch{
							Match: HTTPPathMatchPrefix,
						},
					}},
				}},
			},
			validateErr: "Rule[0], Match[0], Path, prefix type match doesn't start with '/': \"\"",
		},
		"rule matches invalid method": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Rules: []HTTPRouteRule{{
					Matches: []HTTPMatch{{
						Method: HTTPMatchMethod("foo"),
					}},
				}},
			},
			validateErr: "Rule[0], Match[0], Method contains an invalid method \"FOO\"",
		},
		"rule normalizes method casing and path matches": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{{
					Name: "gateway",
				}},
				Rules: []HTTPRouteRule{{
					Matches: []HTTPMatch{{
						Method: HTTPMatchMethod("trace"),
					}},
				}},
			},
		},
		"rule normalizes service weight": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-one",
				Rules: []HTTPRouteRule{{
					Services: []HTTPService{
						{
							Name:   "test",
							Weight: 0,
						},
						{
							Name:   "test2",
							Weight: -1,
						},
					},
				}},
			},
			check: func(t *testing.T, entry ConfigEntry) {
				route := entry.(*HTTPRouteConfigEntry)
				require.Equal(t, 1, route.Rules[0].Services[0].Weight)
				require.Equal(t, 1, route.Rules[0].Services[1].Weight)
			},
		},

		"duplicate parents with no listener specified": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{
					{
						Kind: "api-gateway",
						Name: "gateway",
					},
					{
						Kind: "api-gateway",
						Name: "gateway",
					},
				},
			},
			validateErr: "route parents must be unique",
		},
		"duplicate parents with listener specified": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{
					{
						Kind:        "api-gateway",
						Name:        "gateway",
						SectionName: "same",
					},
					{
						Kind:        "api-gateway",
						Name:        "gateway",
						SectionName: "same",
					},
				},
			},
			validateErr: "route parents must be unique",
		},
		"almost duplicate parents with one not specifying a listener": {
			entry: &HTTPRouteConfigEntry{
				Kind: HTTPRoute,
				Name: "route-two",
				Parents: []ResourceReference{
					{
						Kind: "api-gateway",
						Name: "gateway",
					},
					{
						Kind:        "api-gateway",
						Name:        "gateway",
						SectionName: "same",
					},
				},
			},
			check: func(t *testing.T, entry ConfigEntry) {
				expectedParents := []ResourceReference{
					{
						Kind:           APIGateway,
						Name:           "gateway",
						EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
					},
					{
						Kind:           APIGateway,
						Name:           "gateway",
						SectionName:    "same",
						EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
					},
				}
				route := entry.(*HTTPRouteConfigEntry)
				require.Len(t, route.Parents, 2)
				require.Equal(t, expectedParents[0], route.Parents[0])
				require.Equal(t, expectedParents[1], route.Parents[1])
			},
		},
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}

func TestHTTPMatch_DeepEqual(t *testing.T) {
	type fields struct {
		Headers []HTTPHeaderMatch
		Method  HTTPMatchMethod
		Path    HTTPPathMatch
		Query   []HTTPQueryMatch
	}
	type args struct {
		other HTTPMatch
	}
	tests := map[string]struct {
		match HTTPMatch
		other HTTPMatch
		want  bool
	}{
		"all fields equal": {
			match: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			other: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			want: true,
		},
		"differing number of header matches": {
			match: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			other: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			want: false,
		},
		"differing header matches": {
			match: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h4",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			other: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			want: false,
		},
		"different path matching": {
			match: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/zoidberg",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			other: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			want: false,
		},
		"differing methods": {
			match: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodConnect,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			other: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			want: false,
		},
		"differing number of query matches": {
			match: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			other: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			want: false,
		},
		"different query matches": {
			match: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "another",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			other: HTTPMatch{
				Headers: []HTTPHeaderMatch{
					{
						Match: HTTPHeaderMatchExact,
						Name:  "h1",
						Value: "a",
					},
					{
						Match: HTTPHeaderMatchPrefix,
						Name:  "h2",
						Value: "b",
					},
				},
				Method: HTTPMatchMethodGet,
				Path: HTTPPathMatch{
					Match: HTTPPathMatchType(HTTPHeaderMatchPrefix),
					Value: "/bender",
				},
				Query: []HTTPQueryMatch{
					{
						Match: HTTPQueryMatchExact,
						Name:  "q",
						Value: "nibbler",
					},
					{
						Match: HTTPQueryMatchPresent,
						Name:  "ship",
						Value: "planet express",
					},
				},
			},
			want: false,
		},
	}
	for name, tt := range tests {
		name := name
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := tt.match.DeepEqual(tt.other); got != tt.want {
				t.Errorf("HTTPMatch.DeepEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
