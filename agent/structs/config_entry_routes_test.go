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
		"add redirect filter": {
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
				Rules: []HTTPRouteRule{
					{
						Filters: HTTPFilters{
							Headers:         nil,
							URLRewrite:      nil,
							RetryFilter:     nil,
							TimeoutFilter:   nil,
							JWT:             nil,
							RequestRedirect: &RequestRedirectFilter{Hostname: "hi"},
						},
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
