package structs

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/stretchr/testify/require"
)

func TestConfigEntries_ListRelatedServices_AndACLs(t *testing.T) {
	// This test tests both of these because they are related functions.
	t.Parallel()

	newServiceACL := func(t *testing.T, canRead, canWrite []string) acl.Authorizer {
		var buf bytes.Buffer
		for _, s := range canRead {
			buf.WriteString(fmt.Sprintf("service %q { policy = %q }\n", s, "read"))
		}
		for _, s := range canWrite {
			buf.WriteString(fmt.Sprintf("service %q { policy = %q }\n", s, "write"))
		}

		policy, err := acl.NewPolicyFromSource("", 0, buf.String(), acl.SyntaxCurrent, nil)
		require.NoError(t, err)

		authorizer, err := acl.NewPolicyAuthorizer(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)
		return authorizer
	}

	type testACL struct {
		name       string
		authorizer acl.Authorizer
		canRead    bool
		canWrite   bool
	}

	defaultDenyCase := testACL{
		name:       "deny",
		authorizer: newServiceACL(t, nil, nil),
		canRead:    false,
		canWrite:   false,
	}
	readTestCase := testACL{
		name:       "can read test",
		authorizer: newServiceACL(t, []string{"test"}, nil),
		canRead:    true,
		canWrite:   false,
	}
	writeTestCase := testACL{
		name:       "can write test",
		authorizer: newServiceACL(t, nil, []string{"test"}),
		canRead:    true,
		canWrite:   true,
	}
	writeTestCaseDenied := testACL{
		name:       "cannot write test",
		authorizer: newServiceACL(t, nil, []string{"test"}),
		canRead:    true,
		canWrite:   false,
	}

	for _, tc := range []struct {
		name           string
		entry          discoveryChainConfigEntry
		expectServices []string
		expectACLs     []testACL
	}{
		{
			name: "resolver: self",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
			},
			expectServices: nil,
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCase,
			},
		},
		{
			name: "resolver: redirect",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service: "other",
				},
			},
			expectServices: []string{"other"},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCaseDenied,
				{
					name:       "can write test (with other:read)",
					authorizer: newServiceACL(t, []string{"other"}, []string{"test"}),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name: "resolver: failover",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"foo": {OnlyPassing: true},
					"bar": {OnlyPassing: true},
				},
				Failover: map[string]ServiceResolverFailover{
					"foo": ServiceResolverFailover{
						Service: "other1",
					},
					"bar": ServiceResolverFailover{
						Service: "other2",
					},
				},
			},
			expectServices: []string{"other1", "other2"},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCaseDenied,
				{
					name:       "can write test (with other1:read and other2:read)",
					authorizer: newServiceACL(t, []string{"other1", "other2"}, []string{"test"}),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name: "splitter: self",
			entry: &ServiceSplitterConfigEntry{
				Kind: ServiceSplitter,
				Name: "test",
				Splits: []ServiceSplit{
					{Weight: 100},
				},
			},
			expectServices: nil,
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCase,
			},
		},
		{
			name: "splitter: some",
			entry: &ServiceSplitterConfigEntry{
				Kind: ServiceSplitter,
				Name: "test",
				Splits: []ServiceSplit{
					{Weight: 25, Service: "b"},
					{Weight: 25, Service: "a"},
					{Weight: 50, Service: "c"},
				},
			},
			expectServices: []string{"a", "b", "c"},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCaseDenied,
				{
					name:       "can write test (with a:read, b:read, and c:read)",
					authorizer: newServiceACL(t, []string{"a", "b", "c"}, []string{"test"}),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
		{
			name: "router: self",
			entry: &ServiceRouterConfigEntry{
				Kind: ServiceRouter,
				Name: "test",
			},
			expectServices: []string{"test"},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCase,
			},
		},
		{
			name: "router: some",
			entry: &ServiceRouterConfigEntry{
				Kind: ServiceRouter,
				Name: "test",
				Routes: []ServiceRoute{
					{
						Match: &ServiceRouteMatch{HTTP: &ServiceRouteHTTPMatch{
							PathPrefix: "/foo",
						}},
						Destination: &ServiceRouteDestination{
							Service: "foo",
						},
					},
					{
						Match: &ServiceRouteMatch{HTTP: &ServiceRouteHTTPMatch{
							PathPrefix: "/bar",
						}},
						Destination: &ServiceRouteDestination{
							Service: "bar",
						},
					},
				},
			},
			expectServices: []string{"bar", "foo", "test"},
			expectACLs: []testACL{
				defaultDenyCase,
				readTestCase,
				writeTestCaseDenied,
				{
					name:       "can write test (with foo:read and bar:read)",
					authorizer: newServiceACL(t, []string{"foo", "bar"}, []string{"test"}),
					canRead:    true,
					canWrite:   true,
				},
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// sanity check inputs
			require.NoError(t, tc.entry.Normalize())
			require.NoError(t, tc.entry.Validate())

			got := tc.entry.ListRelatedServices()
			require.Equal(t, tc.expectServices, got)

			for _, a := range tc.expectACLs {
				a := a
				t.Run(a.name, func(t *testing.T) {
					require.Equal(t, a.canRead, tc.entry.CanRead(a.authorizer))
					require.Equal(t, a.canWrite, tc.entry.CanWrite(a.authorizer))
				})
			}
		})
	}
}

func TestServiceResolverConfigEntry(t *testing.T) {
	t.Parallel()

	type testcase struct {
		name         string
		entry        *ServiceResolverConfigEntry
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceResolverConfigEntry)
	}

	cases := []testcase{
		{
			name:         "nil",
			entry:        nil,
			normalizeErr: "config entry is nil",
		},
		{
			name:        "no name",
			entry:       &ServiceResolverConfigEntry{},
			validateErr: "Name is required",
		},
		{
			name: "empty",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
			},
		},
		{
			name: "empty subset name",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"": {OnlyPassing: true},
				},
			},
			validateErr: "Subset defined with empty name",
		},
		{
			name: "default subset does not exist",
			entry: &ServiceResolverConfigEntry{
				Kind:          ServiceResolver,
				Name:          "test",
				DefaultSubset: "gone",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
			},
			validateErr: `DefaultSubset "gone" is not a valid subset`,
		},
		{
			name: "default subset does exist",
			entry: &ServiceResolverConfigEntry{
				Kind:          ServiceResolver,
				Name:          "test",
				DefaultSubset: "v1",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
			},
		},
		{
			name: "empty redirect",
			entry: &ServiceResolverConfigEntry{
				Kind:     ServiceResolver,
				Name:     "test",
				Redirect: &ServiceResolverRedirect{},
			},
			validateErr: "Redirect is empty",
		},
		{
			name: "redirect subset with no service",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					ServiceSubset: "next",
				},
			},
			validateErr: "Redirect.ServiceSubset defined without Redirect.Service",
		},
		{
			name: "redirect namespace with no service",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Namespace: "alternate",
				},
			},
			validateErr: "Redirect.Namespace defined without Redirect.Service",
		},
		{
			name: "self redirect with invalid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service:       "test",
					ServiceSubset: "gone",
				},
			},
			validateErr: `Redirect.ServiceSubset "gone" is not a valid subset of "test"`,
		},
		{
			name: "self redirect with valid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Redirect: &ServiceResolverRedirect{
					Service:       "test",
					ServiceSubset: "v1",
				},
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
			},
		},
		{
			name: "simple wildcard failover",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": ServiceResolverFailover{
						Datacenters: []string{"dc2"},
					},
				},
			},
		},
		{
			name: "failover for missing subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"gone": ServiceResolverFailover{
						Datacenters: []string{"dc2"},
					},
				},
			},
			validateErr: `Bad Failover["gone"]: not a valid subset`,
		},
		{
			name: "failover for present subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": ServiceResolverFailover{
						Datacenters: []string{"dc2"},
					},
				},
			},
		},
		{
			name: "failover empty",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": ServiceResolverFailover{},
				},
			},
			validateErr: `Bad Failover["v1"] one of Service, ServiceSubset, Namespace, or Datacenters is required`,
		},
		{
			name: "failover to self using invalid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": ServiceResolverFailover{
						Service:       "test",
						ServiceSubset: "gone",
					},
				},
			},
			validateErr: `Bad Failover["v1"].ServiceSubset "gone" is not a valid subset of "test"`,
		},
		{
			name: "failover to self using valid subset",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					"v1": {Filter: "Service.Meta.version == v1"},
					"v2": {Filter: "Service.Meta.version == v2"},
				},
				Failover: map[string]ServiceResolverFailover{
					"v1": ServiceResolverFailover{
						Service:       "test",
						ServiceSubset: "v2",
					},
				},
			},
		},
		{
			name: "failover with empty datacenters in list",
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Failover: map[string]ServiceResolverFailover{
					"*": ServiceResolverFailover{
						Service:     "backup",
						Datacenters: []string{"", "dc2", "dc3"},
					},
				},
			},
			validateErr: `Bad Failover["*"].Datacenters: found empty datacenter`,
		},
		{
			name: "bad connect timeout",
			entry: &ServiceResolverConfigEntry{
				Kind:           ServiceResolver,
				Name:           "test",
				ConnectTimeout: -1 * time.Second,
			},
			validateErr: "Bad ConnectTimeout",
		},
	}

	// Bulk add a bunch of similar validation cases.
	for _, invalidSubset := range invalidSubsetNames {
		tc := testcase{
			name: "invalid subset name: " + invalidSubset,
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					invalidSubset: {OnlyPassing: true},
				},
			},
			validateErr: fmt.Sprintf("Subset %q is invalid", invalidSubset),
		}
		cases = append(cases, tc)
	}

	for _, goodSubset := range validSubsetNames {
		tc := testcase{
			name: "valid subset name: " + goodSubset,
			entry: &ServiceResolverConfigEntry{
				Kind: ServiceResolver,
				Name: "test",
				Subsets: map[string]ServiceResolverSubset{
					goodSubset: {OnlyPassing: true},
				},
			},
		}
		cases = append(cases, tc)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceSplitterConfigEntry(t *testing.T) {
	t.Parallel()

	makesplitter := func(splits ...ServiceSplit) *ServiceSplitterConfigEntry {
		return &ServiceSplitterConfigEntry{
			Kind:   ServiceSplitter,
			Name:   "test",
			Splits: splits,
		}
	}

	makesplit := func(weight float32, service, serviceSubset, namespace string) ServiceSplit {
		return ServiceSplit{
			Weight:        weight,
			Service:       service,
			ServiceSubset: serviceSubset,
			Namespace:     namespace,
		}
	}

	for _, tc := range []struct {
		name         string
		entry        *ServiceSplitterConfigEntry
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceSplitterConfigEntry)
	}{
		{
			name:         "nil",
			entry:        nil,
			normalizeErr: "config entry is nil",
		},
		{
			name:        "no name",
			entry:       &ServiceSplitterConfigEntry{},
			validateErr: "Name is required",
		},
		{
			name:        "empty",
			entry:       makesplitter(),
			validateErr: "no splits configured",
		},
		{
			name: "1 split",
			entry: makesplitter(
				makesplit(100, "test", "", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100), entry.Splits[0].Weight)
			},
		},
		{
			name: "1 split not enough weight",
			entry: makesplitter(
				makesplit(99.99, "test", "", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(99.99), entry.Splits[0].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "1 split too much weight",
			entry: makesplitter(
				makesplit(100.01, "test", "", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100.01), entry.Splits[0].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "2 splits",
			entry: makesplitter(
				makesplit(99, "test", "v1", ""),
				makesplit(1, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(99), entry.Splits[0].Weight)
				require.Equal(t, float32(1), entry.Splits[1].Weight)
			},
		},
		{
			name: "2 splits - rounded up to smallest units",
			entry: makesplitter(
				makesplit(99.999, "test", "v1", ""),
				makesplit(0.001, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100), entry.Splits[0].Weight)
				require.Equal(t, float32(0), entry.Splits[1].Weight)
			},
		},
		{
			name: "2 splits not enough weight",
			entry: makesplitter(
				makesplit(99.98, "test", "v1", ""),
				makesplit(0.01, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(99.98), entry.Splits[0].Weight)
				require.Equal(t, float32(0.01), entry.Splits[1].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "2 splits too much weight",
			entry: makesplitter(
				makesplit(100, "test", "v1", ""),
				makesplit(0.01, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(100), entry.Splits[0].Weight)
				require.Equal(t, float32(0.01), entry.Splits[1].Weight)
			},
			validateErr: "the sum of all split weights must be 100",
		},
		{
			name: "3 splits",
			entry: makesplitter(
				makesplit(34, "test", "v1", ""),
				makesplit(33, "test", "v2", ""),
				makesplit(33, "test", "v3", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(34), entry.Splits[0].Weight)
				require.Equal(t, float32(33), entry.Splits[1].Weight)
				require.Equal(t, float32(33), entry.Splits[2].Weight)
			},
		},
		{
			name: "3 splits one duplicated same weights",
			entry: makesplitter(
				makesplit(34, "test", "v1", ""),
				makesplit(33, "test", "v2", ""),
				makesplit(33, "test", "v2", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(34), entry.Splits[0].Weight)
				require.Equal(t, float32(33), entry.Splits[1].Weight)
				require.Equal(t, float32(33), entry.Splits[2].Weight)
			},
			validateErr: "split destination occurs more than once",
		},
		{
			name: "3 splits one duplicated diff weights",
			entry: makesplitter(
				makesplit(34, "test", "v1", ""),
				makesplit(33, "test", "v2", ""),
				makesplit(33, "test", "v1", ""),
			),
			check: func(t *testing.T, entry *ServiceSplitterConfigEntry) {
				require.Equal(t, float32(34), entry.Splits[0].Weight)
				require.Equal(t, float32(33), entry.Splits[1].Weight)
				require.Equal(t, float32(33), entry.Splits[2].Weight)
			},
			validateErr: "split destination occurs more than once",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestServiceRouterConfigEntry(t *testing.T) {
	t.Parallel()

	httpMatch := func(http *ServiceRouteHTTPMatch) *ServiceRouteMatch {
		return &ServiceRouteMatch{HTTP: http}
	}
	httpMatchHeader := func(headers ...ServiceRouteHTTPMatchHeader) *ServiceRouteMatch {
		return httpMatch(&ServiceRouteHTTPMatch{
			Header: headers,
		})
	}
	httpMatchParam := func(params ...ServiceRouteHTTPMatchQueryParam) *ServiceRouteMatch {
		return httpMatch(&ServiceRouteHTTPMatch{
			QueryParam: params,
		})
	}
	toService := func(svc string) *ServiceRouteDestination {
		return &ServiceRouteDestination{Service: svc}
	}
	routeMatch := func(match *ServiceRouteMatch) ServiceRoute {
		return ServiceRoute{
			Match:       match,
			Destination: toService("other"),
		}
	}
	makerouter := func(routes ...ServiceRoute) *ServiceRouterConfigEntry {
		return &ServiceRouterConfigEntry{
			Kind:   ServiceRouter,
			Name:   "test",
			Routes: routes,
		}
	}

	type testcase struct {
		name         string
		entry        *ServiceRouterConfigEntry
		normalizeErr string
		validateErr  string
		// check is called between normalize and validate
		check func(t *testing.T, entry *ServiceRouterConfigEntry)
	}

	cases := []testcase{
		{
			name:         "nil",
			entry:        nil,
			normalizeErr: "config entry is nil",
		},
		{
			name:        "no name",
			entry:       &ServiceRouterConfigEntry{},
			validateErr: "Name is required",
		},
		{
			name:  "empty",
			entry: makerouter(),
		},
		{
			name: "1 empty route",
			entry: makerouter(
				ServiceRoute{},
			),
		},

		{
			name: "route with path exact",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact: "/exact",
			}))),
		},
		{
			name: "route with bad path exact",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact: "no-leading-slash",
			}))),
			validateErr: "PathExact doesn't start with '/'",
		},
		{
			name: "route with path prefix",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathPrefix: "/prefix",
			}))),
		},
		{
			name: "route with bad path prefix",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathPrefix: "no-leading-slash",
			}))),
			validateErr: "PathPrefix doesn't start with '/'",
		},
		{
			name: "route with path regex",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathRegex: "/regex",
			}))),
		},
		{
			name: "route with path exact and prefix",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact:  "/exact",
				PathPrefix: "/prefix",
			}))),
			validateErr: "should only contain at most one of PathExact, PathPrefix, or PathRegex",
		},
		{
			name: "route with path exact and regex",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact: "/exact",
				PathRegex: "/regex",
			}))),
			validateErr: "should only contain at most one of PathExact, PathPrefix, or PathRegex",
		},
		{
			name: "route with path prefix and regex",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathPrefix: "/prefix",
				PathRegex:  "/regex",
			}))),
			validateErr: "should only contain at most one of PathExact, PathPrefix, or PathRegex",
		},
		{
			name: "route with path exact, prefix, and regex",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				PathExact:  "/exact",
				PathPrefix: "/prefix",
				PathRegex:  "/regex",
			}))),
			validateErr: "should only contain at most one of PathExact, PathPrefix, or PathRegex",
		},

		{
			name: "route with no name header",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Present: true,
			}))),
			validateErr: "missing required Name field",
		},
		{
			name: "route with header present",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
			}))),
		},
		{
			name: "route with header not present",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Invert:  true,
			}))),
		},
		{
			name: "route with header exact",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:  "foo",
				Exact: "bar",
			}))),
		},
		{
			name: "route with header regex",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:  "foo",
				Regex: "bar",
			}))),
		},
		{
			name: "route with header prefix",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:   "foo",
				Prefix: "bar",
			}))),
		},
		{
			name: "route with header suffix",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:   "foo",
				Suffix: "bar",
			}))),
		},
		{
			name: "route with header present and exact",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Exact:   "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, Prefix, Suffix, or Regex",
		},
		{
			name: "route with header present and regex",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Regex:   "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, Prefix, Suffix, or Regex",
		},
		{
			name: "route with header present and prefix",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Prefix:  "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, Prefix, Suffix, or Regex",
		},
		{
			name: "route with header present and suffix",
			entry: makerouter(routeMatch(httpMatchHeader(ServiceRouteHTTPMatchHeader{
				Name:    "foo",
				Present: true,
				Suffix:  "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, Prefix, Suffix, or Regex",
		},
		// NOTE: Some combinatoric cases for header operators (some 5 choose 2,
		// all 5 choose 3, all 5 choose 4, all 5 choose 5) are omitted from
		// testing.

		////////////////
		{
			name: "route with no name query param",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Exact: "foo",
			}))),
			validateErr: "missing required Name field",
		},
		{
			name: "route with query param exact match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:  "foo",
				Exact: "bar",
			}))),
		},
		{
			name: "route with query param regex match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:  "foo",
				Regex: "bar",
			}))),
		},
		{
			name: "route with query param present match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:    "foo",
				Present: true,
			}))),
		},
		{
			name: "route with query param exact and regex match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:  "foo",
				Exact: "bar",
				Regex: "bar",
			}))),
			validateErr: "should only contain one of Present, Exact, or Regex",
		},
		{
			name: "route with query param exact and present match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:    "foo",
				Exact:   "bar",
				Present: true,
			}))),
			validateErr: "should only contain one of Present, Exact, or Regex",
		},
		{
			name: "route with query param regex and present match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:    "foo",
				Regex:   "bar",
				Present: true,
			}))),
			validateErr: "should only contain one of Present, Exact, or Regex",
		},
		{
			name: "route with query param exact, regex, and present match",
			entry: makerouter(routeMatch(httpMatchParam(ServiceRouteHTTPMatchQueryParam{
				Name:    "foo",
				Exact:   "bar",
				Regex:   "bar",
				Present: true,
			}))),
			validateErr: "should only contain one of Present, Exact, or Regex",
		},
		////////////////
		{
			name: "route with no match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: nil,
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
			validateErr: "cannot make use of PrefixRewrite without configuring either PathExact or PathPrefix",
		},
		{
			name: "route with path prefix match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatch(&ServiceRouteHTTPMatch{
					PathPrefix: "/api",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
		},
		{
			name: "route with path exact match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatch(&ServiceRouteHTTPMatch{
					PathExact: "/api",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
		},
		{
			name: "route with path regex match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatch(&ServiceRouteHTTPMatch{
					PathRegex: "/api",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
			validateErr: "cannot make use of PrefixRewrite without configuring either PathExact or PathPrefix",
		},
		{
			name: "route with header match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatchHeader(ServiceRouteHTTPMatchHeader{
					Name:  "foo",
					Exact: "bar",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
			validateErr: "cannot make use of PrefixRewrite without configuring either PathExact or PathPrefix",
		},
		{
			name: "route with header match and prefix rewrite",
			entry: makerouter(ServiceRoute{
				Match: httpMatchParam(ServiceRouteHTTPMatchQueryParam{
					Name:  "foo",
					Exact: "bar",
				}),
				Destination: &ServiceRouteDestination{
					Service:       "other",
					PrefixRewrite: "/new",
				},
			}),
			validateErr: "cannot make use of PrefixRewrite without configuring either PathExact or PathPrefix",
		},
		////////////////
		{
			name: "route with method matches",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				Methods: []string{
					"get", "POST", "dElEtE",
				},
			}))),
			check: func(t *testing.T, entry *ServiceRouterConfigEntry) {
				m := entry.Routes[0].Match.HTTP.Methods
				require.Equal(t, []string{"GET", "POST", "DELETE"}, m)
			},
		},
		{
			name: "route with method matches repeated",
			entry: makerouter(routeMatch(httpMatch(&ServiceRouteHTTPMatch{
				Methods: []string{
					"GET", "DELETE", "get",
				},
			}))),
			validateErr: "Methods contains \"GET\" more than once",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Normalize()
			if tc.normalizeErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

var validSubsetNames = []string{
	"a", "aa", "2a", "a2", "a2a", "a22a",
	"1", "11", "10", "01",
	"a-a", "a--a", "a--a--a",
	"0-0", "0--0", "0--0--0",
	strings.Repeat("a", 63),
}

var invalidSubsetNames = []string{
	"A", "AA", "2A", "A2", "A2A", "A22A",
	"A-A", "A--A", "A--A--A",
	" ", " a", "a ", "a a",
	"_", "_a", "a_", "a_a",
	".", ".a", "a.", "a.a",
	"-", "-a", "a-",
	strings.Repeat("a", 64),
}

func TestValidateServiceSubset(t *testing.T) {
	for _, name := range validSubsetNames {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, validateServiceSubset(name))
		})
	}

	for _, name := range invalidSubsetNames {
		t.Run(name, func(t *testing.T) {
			require.Error(t, validateServiceSubset(name))
		})
	}
}
