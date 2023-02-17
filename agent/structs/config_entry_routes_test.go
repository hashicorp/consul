package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
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
					Kind: APIGateway,
					Name: "gateway",
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
					Kind: APIGateway,
					Name: "gateway",
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
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}
