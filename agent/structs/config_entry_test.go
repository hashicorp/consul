// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl"
	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

func TestConfigEntries_ACLs(t *testing.T) {
	type testACL = configEntryTestACL
	type testcase = configEntryACLTestCase

	newAuthz := func(t *testing.T, src string) acl.Authorizer {
		policy, err := acl.NewPolicyFromSource(src, nil, nil)
		require.NoError(t, err)

		authorizer, err := acl.NewPolicyAuthorizerWithDefaults(acl.DenyAll(), []*acl.Policy{policy}, nil)
		require.NoError(t, err)
		return authorizer
	}

	cases := []testcase{
		// =================== proxy-defaults ===================
		{
			name:  "proxy-defaults",
			entry: &ProxyConfigEntry{},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newAuthz(t, ``),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "proxy-defaults: operator deny",
					authorizer: newAuthz(t, `operator = "deny"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "proxy-defaults: operator read",
					authorizer: newAuthz(t, `operator = "read"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "proxy-defaults: operator write",
					authorizer: newAuthz(t, `operator = "write"`),
					canRead:    true, // unauthenticated
					canWrite:   true,
				},
				{
					name:       "proxy-defaults: mesh deny",
					authorizer: newAuthz(t, `mesh = "deny"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "proxy-defaults: mesh read",
					authorizer: newAuthz(t, `mesh = "read"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "proxy-defaults: mesh write",
					authorizer: newAuthz(t, `mesh = "write"`),
					canRead:    true, // unauthenticated
					canWrite:   true,
				},
				{
					name:       "proxy-defaults: operator deny and mesh read",
					authorizer: newAuthz(t, `operator = "deny" mesh = "read"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "proxy-defaults: operator deny and mesh write",
					authorizer: newAuthz(t, `operator = "deny" mesh = "write"`),
					canRead:    true, // unauthenticated
					canWrite:   true,
				},
			},
		},
		// =================== mesh ===================
		{
			name:  "mesh",
			entry: &MeshConfigEntry{},
			expectACLs: []testACL{
				{
					name:       "no-authz",
					authorizer: newAuthz(t, ``),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "mesh: operator deny",
					authorizer: newAuthz(t, `operator = "deny"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "mesh: operator read",
					authorizer: newAuthz(t, `operator = "read"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "mesh: operator write",
					authorizer: newAuthz(t, `operator = "write"`),
					canRead:    true, // unauthenticated
					canWrite:   true,
				},
				{
					name:       "mesh: mesh deny",
					authorizer: newAuthz(t, `mesh = "deny"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "mesh: mesh read",
					authorizer: newAuthz(t, `mesh = "read"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "mesh: mesh write",
					authorizer: newAuthz(t, `mesh = "write"`),
					canRead:    true, // unauthenticated
					canWrite:   true,
				},
				{
					name:       "mesh: operator deny and mesh read",
					authorizer: newAuthz(t, `operator = "deny" mesh = "read"`),
					canRead:    true, // unauthenticated
					canWrite:   false,
				},
				{
					name:       "mesh: operator deny and mesh write",
					authorizer: newAuthz(t, `operator = "deny" mesh = "write"`),
					canRead:    true, // unauthenticated
					canWrite:   true,
				},
			},
		},
	}

	testConfigEntries_ListRelatedServices_AndACLs(t, cases)
}

type configEntryTestACL struct {
	name       string
	authorizer acl.Authorizer
	canRead    bool
	canWrite   bool
}

type configEntryACLTestCase struct {
	name           string
	entry          ConfigEntry
	expectServices []ServiceID // optional
	expectACLs     []configEntryTestACL
}

func testConfigEntries_ListRelatedServices_AndACLs(t *testing.T, cases []configEntryACLTestCase) {

	// This test tests both of these because they are related functions.
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// verify test inputs
			require.NoError(t, tc.entry.Normalize())
			require.NoError(t, tc.entry.Validate())

			if dce, ok := tc.entry.(discoveryChainConfigEntry); ok {
				got := dce.ListRelatedServices()
				require.Equal(t, tc.expectServices, got)
			}

			if len(tc.expectACLs) == 1 {
				a := tc.expectACLs[0]
				require.Empty(t, a.name)
			} else {
				for _, a := range tc.expectACLs {
					require.NotEmpty(t, a.name)
					t.Run(a.name, func(t *testing.T) {
						canRead := tc.entry.CanRead(a.authorizer)
						if a.canRead {
							require.Nil(t, canRead)
						} else {
							require.Error(t, canRead)
							require.True(t, acl.IsErrPermissionDenied(canRead))
						}
						canWrite := tc.entry.CanWrite(a.authorizer)
						if a.canWrite {
							require.Nil(t, canWrite)
						} else {
							require.Error(t, canWrite)
							require.True(t, acl.IsErrPermissionDenied(canWrite))
						}
					})
				}
			}
		})
	}
}

func TestDecodeConfigEntry_ServiceDefaults(t *testing.T) {

	for _, tc := range []struct {
		name      string
		camel     string
		snake     string
		expect    ConfigEntry
		expectErr string
	}{
		{
			name: "service-defaults-with-MaxInboundConnections",
			snake: `
				kind = "service-defaults"
				name = "external"
				protocol = "tcp"
				destination {
					addresses = [
						"api.google.com",
						"web.google.com"
					]
					port = 8080
				}
				max_inbound_connections = 14
			`,
			camel: `
				Kind = "service-defaults"
				Name = "external"
				Protocol = "tcp"
				Destination {
					Addresses = [
						"api.google.com",
						"web.google.com"
					]
					Port = 8080
				}
				MaxInboundConnections = 14
			`,
			expect: &ServiceConfigEntry{
				Kind:     "service-defaults",
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: []string{
						"api.google.com",
						"web.google.com",
					},
					Port: 8080,
				},
				MaxInboundConnections: 14,
			},
		},
	} {
		tc := tc

		testbody := func(t *testing.T, body string) {
			var raw map[string]interface{}
			err := hcl.Decode(&raw, body)
			require.NoError(t, err)

			got, err := DecodeConfigEntry(raw)
			if tc.expectErr != "" {
				require.Nil(t, got)
				require.Error(t, err)
				requireContainsLower(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expect, got)
			}
		}

		t.Run(tc.name+" (snake case)", func(t *testing.T) {
			testbody(t, tc.snake)
		})
		t.Run(tc.name+" (camel case)", func(t *testing.T) {
			testbody(t, tc.camel)
		})
	}
}

// TestDecodeConfigEntry is the 'structs' mirror image of
// command/helpers/helpers_test.go:TestParseConfigEntry
func TestDecodeConfigEntry(t *testing.T) {

	for _, tc := range []struct {
		name      string
		camel     string
		snake     string
		expect    ConfigEntry
		expectErr string
	}{
		// TODO(rb): test json?
		{
			name: "proxy-defaults: extra fields or typo",
			snake: `
				kind = "proxy-defaults"
				name = "main"
				cornfig {
				  "foo" = 19
				}
			`,
			camel: `
				Kind = "proxy-defaults"
				Name = "main"
				Cornfig {
				  "foo" = 19
				}
			`,
			expectErr: `invalid config key "cornfig"`,
		},
		{
			name: "proxy-defaults",
			snake: `
				kind = "proxy-defaults"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				config {
				  "foo" = 19
				  "bar" = "abc"
				  "moreconfig" {
					"moar" = "config"
				  }
				  "balance_inbound_connections" = "exact_balance"
				}
				mesh_gateway {
					mode = "remote"
				}
				mutual_tls_mode = "permissive"
			`,
			camel: `
				Kind = "proxy-defaults"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Config {
				  "foo" = 19
				  "bar" = "abc"
				  "moreconfig" {
					"moar" = "config"
				  }
				  "balance_inbound_connections" = "exact_balance"
				}
				MeshGateway {
					Mode = "remote"
				}
				MutualTLSMode = "permissive"
			`,
			expect: &ProxyConfigEntry{
				Kind: "proxy-defaults",
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Config: map[string]interface{}{
					"foo": 19,
					"bar": "abc",
					"moreconfig": map[string]interface{}{
						"moar": "config",
					},
					"balance_inbound_connections": "exact_balance",
				},
				MeshGateway: MeshGatewayConfig{
					Mode: MeshGatewayModeRemote,
				},
				MutualTLSMode: MutualTLSModePermissive,
			},
		},
		{
			name: "service-defaults",
			snake: `
				kind = "service-defaults"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				protocol = "http"
				external_sni = "abc-123"
				mesh_gateway {
					mode = "remote"
				}
				mutual_tls_mode = "permissive"
				balance_inbound_connections = "exact_balance"
				upstream_config {
					overrides = [
						{
							name = "redis"
							passive_health_check {
								interval = "2s"
								max_failures = 3
								enforcing_consecutive_5xx = 4
								max_ejection_percent = 5
								base_ejection_time = "6s"
							}
						},
						{
							name = "finance--billing"
							mesh_gateway {
								mode = "remote"
							}
						},
					]
					defaults {
						connect_timeout_ms = 5
						protocol = "http"
						balance_outbound_connections = "exact_balance"
						envoy_listener_json = "foo"
						envoy_cluster_json = "bar"
						limits {
							max_connections = 3
							max_pending_requests = 4
							max_concurrent_requests = 5
						}
					}
				}
			`,
			camel: `
				Kind = "service-defaults"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Protocol = "http"
				ExternalSNI = "abc-123"
				MeshGateway {
					Mode = "remote"
				}
				MutualTLSMode = "permissive"
				BalanceInboundConnections = "exact_balance"
				UpstreamConfig {
					Overrides = [
						{
							Name = "redis"
							PassiveHealthCheck {
								MaxFailures = 3
								Interval = "2s"
								EnforcingConsecutive5xx = 4
								MaxEjectionPercent = 5
								BaseEjectionTime = "6s"
							}
						},
						{
							Name = "finance--billing"
							MeshGateway {
								Mode = "remote"
							}
						},
					]
					Defaults {
						EnvoyListenerJSON = "foo"
						EnvoyClusterJSON = "bar"
						ConnectTimeoutMs = 5
						Protocol = "http"
						Limits {
							MaxConnections = 3
							MaxPendingRequests = 4
							MaxConcurrentRequests = 5
						}
						BalanceOutboundConnections = "exact_balance"
					}
				}
			`,
			expect: &ServiceConfigEntry{
				Kind: "service-defaults",
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Protocol:    "http",
				ExternalSNI: "abc-123",
				MeshGateway: MeshGatewayConfig{
					Mode: MeshGatewayModeRemote,
				},
				MutualTLSMode:             MutualTLSModePermissive,
				BalanceInboundConnections: "exact_balance",
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name: "redis",
							PassiveHealthCheck: &PassiveHealthCheck{
								MaxFailures:             3,
								Interval:                2 * time.Second,
								EnforcingConsecutive5xx: uintPointer(4),
								MaxEjectionPercent:      uintPointer(5),
								BaseEjectionTime:        durationPointer(6 * time.Second),
							},
						},
						{
							Name:        "finance--billing",
							MeshGateway: MeshGatewayConfig{Mode: MeshGatewayModeRemote},
						},
					},
					Defaults: &UpstreamConfig{
						EnvoyListenerJSON: "foo",
						EnvoyClusterJSON:  "bar",
						ConnectTimeoutMs:  5,
						Protocol:          "http",
						Limits: &UpstreamLimits{
							MaxConnections:        intPointer(3),
							MaxPendingRequests:    intPointer(4),
							MaxConcurrentRequests: intPointer(5),
						},
						BalanceOutboundConnections: "exact_balance",
					},
				},
			},
		},
		{
			name: "service-defaults-with-destination",
			snake: `
				kind = "service-defaults"
				name = "external"
				protocol = "tcp"
				destination {
					addresses = [
						"api.google.com",
						"web.google.com"
					]
					port = 8080
				}
			`,
			camel: `
				Kind = "service-defaults"
				Name = "external"
				Protocol = "tcp"
				Destination {
					Addresses = [
						"api.google.com",
						"web.google.com"
					]
					Port = 8080
				}
			`,
			expect: &ServiceConfigEntry{
				Kind:     "service-defaults",
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: []string{
						"api.google.com",
						"web.google.com",
					},
					Port: 8080,
				},
			},
		},
		{
			name: "service-router: kitchen sink",
			snake: `
				kind = "service-router"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				routes = [
					{
						match {
							http {
								path_exact = "/foo"
								header = [
									{
										name = "debug1"
										present = true
									},
									{
										name = "debug2"
										present = false
										invert = true
									},
									{
										name = "debug3"
										exact = "1"
									},
									{
										name = "debug4"
										prefix = "aaa"
									},
									{
										name = "debug5"
										suffix = "bbb"
									},
									{
										name = "debug6"
										regex = "a.*z"
									},
								]
							}
						}
						destination {
						  service               = "carrot"
						  service_subset         = "kale"
						  namespace             = "leek"
						  prefix_rewrite         = "/alternate"
						  request_timeout        = "99s"
						  idle_timeout           = "99s"
						  num_retries            = 12345
						  retry_on_connect_failure = true
						  retry_on_status_codes    = [401, 209]
							request_headers {
								add {
									x-foo = "bar"
								}
								set {
									bar = "baz"
								}
								remove = ["qux"]
							}
							response_headers {
								add {
									x-foo = "bar"
								}
								set {
									bar = "baz"
								}
								remove = ["qux"]
							}
						}
					},
					{
						match {
							http {
								path_prefix = "/foo"
								methods = [ "GET", "DELETE" ]
								query_param = [
									{
										name = "hack1"
										present = true
									},
									{
										name = "hack2"
										exact = "1"
									},
									{
										name = "hack3"
										regex = "a.*z"
									},
								]
							}
						}
					},
					{
						match {
							http {
								path_regex = "/foo"
							}
						}
					},
				]
			`,
			camel: `
				Kind = "service-router"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Routes = [
					{
						Match {
							HTTP {
								PathExact = "/foo"
								Header = [
									{
										Name = "debug1"
										Present = true
									},
									{
										Name = "debug2"
										Present = false
										Invert = true
									},
									{
										Name = "debug3"
										Exact = "1"
									},
									{
										Name = "debug4"
										Prefix = "aaa"
									},
									{
										Name = "debug5"
										Suffix = "bbb"
									},
									{
										Name = "debug6"
										Regex = "a.*z"
									},
								]
							}
						}
						Destination {
						  Service               = "carrot"
						  ServiceSubset         = "kale"
						  Namespace             = "leek"
						  PrefixRewrite         = "/alternate"
						  RequestTimeout        = "99s"
						  IdleTimeout           = "99s"
						  NumRetries            = 12345
						  RetryOnConnectFailure = true
						  RetryOnStatusCodes    = [401, 209]
							RequestHeaders {
								Add {
									x-foo = "bar"
								}
								Set {
									bar = "baz"
								}
								Remove = ["qux"]
							}
							ResponseHeaders {
								Add {
									x-foo = "bar"
								}
								Set {
									bar = "baz"
								}
								Remove = ["qux"]
							}
						}
					},
					{
						Match {
							HTTP {
								PathPrefix = "/foo"
								Methods = [ "GET", "DELETE" ]
								QueryParam = [
									{
										Name = "hack1"
										Present = true
									},
									{
										Name = "hack2"
										Exact = "1"
									},
									{
										Name = "hack3"
										Regex = "a.*z"
									},
								]
							}
						}
					},
					{
						Match {
							HTTP {
								PathRegex = "/foo"
							}
						}
					},
				]
			`,
			expect: &ServiceRouterConfigEntry{
				Kind: "service-router",
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Routes: []ServiceRoute{
					{
						Match: &ServiceRouteMatch{
							HTTP: &ServiceRouteHTTPMatch{
								PathExact: "/foo",
								Header: []ServiceRouteHTTPMatchHeader{
									{
										Name:    "debug1",
										Present: true,
									},
									{
										Name:    "debug2",
										Present: false,
										Invert:  true,
									},
									{
										Name:  "debug3",
										Exact: "1",
									},
									{
										Name:   "debug4",
										Prefix: "aaa",
									},
									{
										Name:   "debug5",
										Suffix: "bbb",
									},
									{
										Name:  "debug6",
										Regex: "a.*z",
									},
								},
							},
						},
						Destination: &ServiceRouteDestination{
							Service:               "carrot",
							ServiceSubset:         "kale",
							Namespace:             "leek",
							PrefixRewrite:         "/alternate",
							RequestTimeout:        99 * time.Second,
							IdleTimeout:           99 * time.Second,
							NumRetries:            12345,
							RetryOnConnectFailure: true,
							RetryOnStatusCodes:    []uint32{401, 209},
							RequestHeaders: &HTTPHeaderModifiers{
								Add:    map[string]string{"x-foo": "bar"},
								Set:    map[string]string{"bar": "baz"},
								Remove: []string{"qux"},
							},
							ResponseHeaders: &HTTPHeaderModifiers{
								Add:    map[string]string{"x-foo": "bar"},
								Set:    map[string]string{"bar": "baz"},
								Remove: []string{"qux"},
							},
						},
					},
					{
						Match: &ServiceRouteMatch{
							HTTP: &ServiceRouteHTTPMatch{
								PathPrefix: "/foo",
								Methods:    []string{"GET", "DELETE"},
								QueryParam: []ServiceRouteHTTPMatchQueryParam{
									{
										Name:    "hack1",
										Present: true,
									},
									{
										Name:  "hack2",
										Exact: "1",
									},
									{
										Name:  "hack3",
										Regex: "a.*z",
									},
								},
							},
						},
					},
					{
						Match: &ServiceRouteMatch{
							HTTP: &ServiceRouteHTTPMatch{
								PathRegex: "/foo",
							},
						},
					},
				},
			},
		},
		{
			name: "service-splitter: kitchen sink",
			snake: `
				kind = "service-splitter"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				splits = [
				  {
						weight        = 99.1
						service_subset = "v1"
						request_headers {
							add {
								foo = "bar"
							}
							set {
								bar = "baz"
							}
							remove = ["qux"]
						}
						response_headers {
							add {
								foo = "bar"
							}
							set {
								bar = "baz"
							}
							remove = ["qux"]
						}
				  },
				  {
						weight    = 0.9
						service   = "other"
						namespace = "alt"
				  },
				]
			`,
			camel: `
				Kind = "service-splitter"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Splits = [
				  {
						Weight        = 99.1
						ServiceSubset = "v1"
						RequestHeaders {
							Add {
								foo = "bar"
							}
							Set {
								bar = "baz"
							}
							Remove = ["qux"]
						}
						ResponseHeaders {
							Add {
								foo = "bar"
							}
							Set {
								bar = "baz"
							}
							Remove = ["qux"]
						}
				  },
				  {
						Weight    = 0.9
						Service   = "other"
						Namespace = "alt"
				  },
				]
			`,
			expect: &ServiceSplitterConfigEntry{
				Kind: ServiceSplitter,
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Splits: []ServiceSplit{
					{
						Weight:        99.1,
						ServiceSubset: "v1",
						RequestHeaders: &HTTPHeaderModifiers{
							Add:    map[string]string{"foo": "bar"},
							Set:    map[string]string{"bar": "baz"},
							Remove: []string{"qux"},
						},
						ResponseHeaders: &HTTPHeaderModifiers{
							Add:    map[string]string{"foo": "bar"},
							Set:    map[string]string{"bar": "baz"},
							Remove: []string{"qux"},
						},
					},
					{
						Weight:    0.9,
						Service:   "other",
						Namespace: "alt",
					},
				},
			},
		},
		{
			name: "service-resolver: subsets with failover",
			snake: `
				kind = "service-resolver"
				name = "main"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				default_subset = "v1"
				connect_timeout = "15s"
				subsets = {
					"v1" = {
						filter = "Service.Meta.version == v1"
					},
					"v2" = {
						filter = "Service.Meta.version == v2"
						only_passing = true
					},
				}
				failover = {
					"v2" = {
						service = "failcopy"
						service_subset = "sure"
						namespace = "neighbor"
						datacenters = ["dc5", "dc14"]
					},
					"*" = {
						datacenters = ["dc7"]
					}
				}`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				DefaultSubset = "v1"
				ConnectTimeout = "15s"
				Subsets = {
					"v1" = {
						Filter = "Service.Meta.version == v1"
					},
					"v2" = {
						Filter = "Service.Meta.version == v2"
						OnlyPassing = true
					},
				}
				Failover = {
					"v2" = {
						Service = "failcopy"
						ServiceSubset = "sure"
						Namespace = "neighbor"
						Datacenters = ["dc5", "dc14"]
					},
					"*" = {
						Datacenters = ["dc7"]
					}
				}`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				DefaultSubset:  "v1",
				ConnectTimeout: 15 * time.Second,
				Subsets: map[string]ServiceResolverSubset{
					"v1": {
						Filter: "Service.Meta.version == v1",
					},
					"v2": {
						Filter:      "Service.Meta.version == v2",
						OnlyPassing: true,
					},
				},
				Failover: map[string]ServiceResolverFailover{
					"v2": {
						Service:       "failcopy",
						ServiceSubset: "sure",
						Namespace:     "neighbor",
						Datacenters:   []string{"dc5", "dc14"},
					},
					"*": {
						Datacenters: []string{"dc7"},
					},
				},
			},
		},
		{
			name: "service-resolver: redirect",
			snake: `
				kind = "service-resolver"
				name = "main"
				redirect {
					service = "other"
					service_subset = "backup"
					namespace = "alt"
					datacenter = "dc9"
				}
			`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
				Redirect {
					Service = "other"
					ServiceSubset = "backup"
					Namespace = "alt"
					Datacenter = "dc9"
				}
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				Redirect: &ServiceResolverRedirect{
					Service:       "other",
					ServiceSubset: "backup",
					Namespace:     "alt",
					Datacenter:    "dc9",
				},
			},
		},
		{
			name: "service-resolver: default",
			snake: `
				kind = "service-resolver"
				name = "main"
			`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
			},
		},
		{
			name: "service-resolver: envoy hash lb kitchen sink",
			snake: `
				kind = "service-resolver"
				name = "main"
				load_balancer = {
					policy = "ring_hash"
					ring_hash_config = {
						minimum_ring_size = 1
						maximum_ring_size = 2
					}
					hash_policies = [
						{
							field = "cookie"
							field_value = "good-cookie"
							cookie_config = {
								ttl = "1s"
								path = "/oven"
							}
							terminal = true
						},
						{
							field = "cookie"
							field_value = "less-good-cookie"
							cookie_config = {
								session = true
								path = "/toaster"
							}
							terminal = true
						},
						{
							field = "header"
							field_value = "x-user-id"
						},
						{
							source_ip = true
						}
					]
				}
			`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
				LoadBalancer = {
					Policy = "ring_hash"
					RingHashConfig = {
						MinimumRingSize = 1
						MaximumRingSize = 2
					}
					HashPolicies = [
						{
							Field = "cookie"
							FieldValue = "good-cookie"
							CookieConfig = {
								TTL = "1s"
								Path = "/oven"
							}
							Terminal = true
						},
						{
							Field = "cookie"
							FieldValue = "less-good-cookie"
							CookieConfig = {
								Session = true
								Path = "/toaster"
							}
							Terminal = true
						},
						{
							Field = "header"
							FieldValue = "x-user-id"
						},
						{
							SourceIP = true
						}
					]
				}
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyRingHash,
					RingHashConfig: &RingHashConfig{
						MinimumRingSize: 1,
						MaximumRingSize: 2,
					},
					HashPolicies: []HashPolicy{
						{
							Field:      HashPolicyCookie,
							FieldValue: "good-cookie",
							CookieConfig: &CookieConfig{
								TTL:  1 * time.Second,
								Path: "/oven",
							},
							Terminal: true,
						},
						{
							Field:      HashPolicyCookie,
							FieldValue: "less-good-cookie",
							CookieConfig: &CookieConfig{
								Session: true,
								Path:    "/toaster",
							},
							Terminal: true,
						},
						{
							Field:      HashPolicyHeader,
							FieldValue: "x-user-id",
						},
						{
							SourceIP: true,
						},
					},
				},
			},
		},
		{
			name: "service-resolver: envoy least request kitchen sink",
			snake: `
				kind = "service-resolver"
				name = "main"
				load_balancer = {
					policy = "least_request"
					least_request_config = {
						choice_count = 2
					}
				}
			`,
			camel: `
				Kind = "service-resolver"
				Name = "main"
				LoadBalancer = {
					Policy = "least_request"
					LeastRequestConfig = {
						ChoiceCount = 2
					}
				}
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				LoadBalancer: &LoadBalancer{
					Policy: LBPolicyLeastRequest,
					LeastRequestConfig: &LeastRequestConfig{
						ChoiceCount: 2,
					},
				},
			},
		},
		{
			// TODO(rb): test SDS stuff here in both places (global/service)
			name: "ingress-gateway: kitchen sink",
			snake: `
				kind = "ingress-gateway"
				name = "ingress-web"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}

				tls {
					enabled = true
					tls_min_version = "TLSv1_1"
					tls_max_version = "TLSv1_2"
					cipher_suites = [
						"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
						"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
					]
				}

				listeners = [
					{
						port = 8080
						protocol = "http"
						services = [
							{
								name = "web"
								hosts = ["test.example.com", "test2.example.com"]
							},
							{
								name = "db"
								request_headers {
									add {
										foo = "bar"
									}
									set {
										bar = "baz"
									}
									remove = ["qux"]
								}
								response_headers {
									add {
										foo = "bar"
									}
									set {
										bar = "baz"
									}
									remove = ["qux"]
								}
							}
						]
					},
					{
						port = 9999
						protocol = "tcp"
						services = [
							{
								name = "mysql"
							}
						]
					},
					{
						port = 2234
						protocol = "tcp"
						services = [
							{
								name = "postgres"
							}
						]
					}
				]
			`,
			camel: `
				Kind = "ingress-gateway"
				Name = "ingress-web"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				TLS {
					Enabled = true
					TLSMinVersion = "TLSv1_1"
					TLSMaxVersion = "TLSv1_2"
					CipherSuites = [
						"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
						"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
					]
				}
				Listeners = [
					{
						Port = 8080
						Protocol = "http"
						Services = [
							{
								Name = "web"
								Hosts = ["test.example.com", "test2.example.com"]
							},
							{
								Name = "db"
								RequestHeaders {
									Add {
										foo = "bar"
									}
									Set {
										bar = "baz"
									}
									Remove = ["qux"]
								}
								ResponseHeaders {
									Add {
										foo = "bar"
									}
									Set {
										bar = "baz"
									}
									Remove = ["qux"]
								}
							}
						]
					},
					{
						Port = 9999
						Protocol = "tcp"
						Services = [
							{
								Name = "mysql"
							}
						]
					},
					{
						Port = 2234
						Protocol = "tcp"
						Services = [
							{
								Name = "postgres"
							}
						]
					}
				]
			`,
			expect: &IngressGatewayConfigEntry{
				Kind: "ingress-gateway",
				Name: "ingress-web",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				TLS: GatewayTLSConfig{
					Enabled:       true,
					TLSMinVersion: types.TLSv1_1,
					TLSMaxVersion: types.TLSv1_2,
					CipherSuites: []types.TLSCipherSuite{
						types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
						types.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					},
				},
				Listeners: []IngressListener{
					{
						Port:     8080,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:  "web",
								Hosts: []string{"test.example.com", "test2.example.com"},
							},
							{
								Name: "db",
								RequestHeaders: &HTTPHeaderModifiers{
									Add:    map[string]string{"foo": "bar"},
									Set:    map[string]string{"bar": "baz"},
									Remove: []string{"qux"},
								},
								ResponseHeaders: &HTTPHeaderModifiers{
									Add:    map[string]string{"foo": "bar"},
									Set:    map[string]string{"bar": "baz"},
									Remove: []string{"qux"},
								},
							},
						},
					},
					{
						Port:     9999,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "mysql",
							},
						},
					},
					{
						Port:     2234,
						Protocol: "tcp",
						Services: []IngressService{
							{
								Name: "postgres",
							},
						},
					},
				},
			},
		},
		{
			name: "terminating-gateway: kitchen sink",
			snake: `
				kind = "terminating-gateway"
				name = "terminating-gw-west"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				services = [
					{
						name = "payments",
						ca_file = "/etc/payments/ca.pem",
						cert_file = "/etc/payments/cert.pem",
						key_file = "/etc/payments/tls.key",
						sni = "mydomain",
					},
					{
						name = "*",
						ca_file = "/etc/all/ca.pem",
						cert_file = "/etc/all/cert.pem",
						key_file = "/etc/all/tls.key",
						sni = "my-alt-domain",
					},
				]
			`,
			camel: `
				Kind = "terminating-gateway"
				Name = "terminating-gw-west"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Services = [
					{
						Name = "payments",
						CAFile = "/etc/payments/ca.pem",
						CertFile = "/etc/payments/cert.pem",
						KeyFile = "/etc/payments/tls.key",
						SNI = "mydomain",
					},
					{
						Name = "*",
						CAFile = "/etc/all/ca.pem",
						CertFile = "/etc/all/cert.pem",
						KeyFile = "/etc/all/tls.key",
						SNI = "my-alt-domain",
					},
				]
			`,
			expect: &TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-gw-west",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Services: []LinkedService{
					{
						Name:     "payments",
						CAFile:   "/etc/payments/ca.pem",
						CertFile: "/etc/payments/cert.pem",
						KeyFile:  "/etc/payments/tls.key",
						SNI:      "mydomain",
					},
					{
						Name:     "*",
						CAFile:   "/etc/all/ca.pem",
						CertFile: "/etc/all/cert.pem",
						KeyFile:  "/etc/all/tls.key",
						SNI:      "my-alt-domain",
					},
				},
			},
		},
		{
			name: "service-intentions: kitchen sink",
			snake: `
				kind = "service-intentions"
				name = "web"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				sources = [
				  {
					name        = "foo"
					action      = "deny"
					type        = "consul"
					description = "foo desc"
				  },
				  {
					name        = "bar"
					action      = "allow"
					description = "bar desc"
				  },
				  {
					name = "l7"
					permissions = [
					  {
						action = "deny"
						http {
						  path_exact = "/admin"
						  header = [
							{
							  name    = "hdr-present"
							  present = true
							},
							{
							  name  = "hdr-exact"
							  exact = "exact"
							},
							{
							  name   = "hdr-prefix"
							  prefix = "prefix"
							},
							{
							  name   = "hdr-suffix"
							  suffix = "suffix"
							},
							{
							  name  = "hdr-regex"
							  regex = "regex"
							},
							{
							  name    = "hdr-absent"
							  present = true
							  invert  = true
							}
						  ]
						}
					  },
					  {
						action = "allow"
						http {
						  path_prefix = "/v3/"
						}
					  },
					  {
						action = "allow"
						http {
						  path_regex = "/v[12]/.*"
						  methods    = ["GET", "POST"]
						}
					  }
					]
				  }
				]
				sources {
				  name        = "*"
				  action      = "deny"
				  description = "wild desc"
				}
			`,
			camel: `
				Kind = "service-intentions"
				Name = "web"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Sources = [
				  {
					Name        = "foo"
					Action      = "deny"
					Type        = "consul"
					Description = "foo desc"
				  },
				  {
					Name        = "bar"
					Action      = "allow"
					Description = "bar desc"
				  },
				  {
					Name = "l7"
					Permissions = [
					  {
						Action = "deny"
						HTTP {
						  PathExact = "/admin"
						  Header = [
							{
							  Name    = "hdr-present"
							  Present = true
							},
							{
							  Name  = "hdr-exact"
							  Exact = "exact"
							},
							{
							  Name   = "hdr-prefix"
							  Prefix = "prefix"
							},
							{
							  Name   = "hdr-suffix"
							  Suffix = "suffix"
							},
							{
							  Name  = "hdr-regex"
							  Regex = "regex"
							},
							{
							  Name    = "hdr-absent"
							  Present = true
							  Invert  = true
							}
						  ]
						}
					  },
					  {
						Action = "allow"
						HTTP {
						  PathPrefix = "/v3/"
						}
					  },
					  {
						Action = "allow"
						HTTP {
						  PathRegex = "/v[12]/.*"
						  Methods   = ["GET", "POST"]
						}
					  }
					]
				  }
				]
				Sources {
				  Name        = "*"
				  Action      = "deny"
				  Description = "wild desc"
				}
			`,
			expect: &ServiceIntentionsConfigEntry{
				Kind: "service-intentions",
				Name: "web",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Sources: []*SourceIntention{
					{
						Name:        "foo",
						Action:      "deny",
						Type:        "consul",
						Description: "foo desc",
					},
					{
						Name:        "bar",
						Action:      "allow",
						Description: "bar desc",
					},
					{
						Name: "l7",
						Permissions: []*IntentionPermission{
							{
								Action: "deny",
								HTTP: &IntentionHTTPPermission{
									PathExact: "/admin",
									Header: []IntentionHTTPHeaderPermission{
										{
											Name:    "hdr-present",
											Present: true,
										},
										{
											Name:  "hdr-exact",
											Exact: "exact",
										},
										{
											Name:   "hdr-prefix",
											Prefix: "prefix",
										},
										{
											Name:   "hdr-suffix",
											Suffix: "suffix",
										},
										{
											Name:  "hdr-regex",
											Regex: "regex",
										},
										{
											Name:    "hdr-absent",
											Present: true,
											Invert:  true,
										},
									},
								},
							},
							{
								Action: "allow",
								HTTP: &IntentionHTTPPermission{
									PathPrefix: "/v3/",
								},
							},
							{
								Action: "allow",
								HTTP: &IntentionHTTPPermission{
									PathRegex: "/v[12]/.*",
									Methods:   []string{"GET", "POST"},
								},
							},
						},
					},
					{
						Name:        "*",
						Action:      "deny",
						Description: "wild desc",
					},
				},
			},
		},
		{
			name: "service-intentions: wildcard destination",
			snake: `
				kind = "service-intentions"
				name = "*"
				sources {
				  name   = "foo"
				  action = "deny"
				  # should be parsed, but we'll ignore it later
				  precedence = 6
				}
			`,
			camel: `
				Kind = "service-intentions"
				Name = "*"
				Sources {
				  Name   = "foo"
				  Action = "deny"
				  # should be parsed, but we'll ignore it later
				  Precedence = 6
				}
			`,
			expect: &ServiceIntentionsConfigEntry{
				Kind: "service-intentions",
				Name: "*",
				Sources: []*SourceIntention{
					{
						Name:       "foo",
						Action:     "deny",
						Precedence: 6,
					},
				},
			},
		},
		{
			name: "mesh",
			snake: `
				kind = "mesh"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				transparent_proxy {
					mesh_destinations_only = true
				}
				allow_enabling_permissive_mutual_tls = true
				tls {
					incoming {
						tls_min_version = "TLSv1_1"
						tls_max_version = "TLSv1_2"
						cipher_suites = [
							"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
						]
					}
					outgoing {
						tls_min_version = "TLSv1_1"
						tls_max_version = "TLSv1_2"
						cipher_suites = [
							"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
						]
					}
				}
				http {
					sanitize_x_forwarded_client_cert = true
				}
				peering {
					peer_through_mesh_gateways = true
				}
			`,
			camel: `
				Kind = "mesh"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				TransparentProxy {
					MeshDestinationsOnly = true
				}
				AllowEnablingPermissiveMutualTLS = true
				TLS {
					Incoming {
						TLSMinVersion = "TLSv1_1"
						TLSMaxVersion = "TLSv1_2"
						CipherSuites = [
							"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
						]
					}
					Outgoing {
						TLSMinVersion = "TLSv1_1"
						TLSMaxVersion = "TLSv1_2"
						CipherSuites = [
							"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
						]
					}
				}
				HTTP {
					SanitizeXForwardedClientCert = true
				}
				Peering {
					PeerThroughMeshGateways = true
				}
			`,
			expect: &MeshConfigEntry{
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				TransparentProxy: TransparentProxyMeshConfig{
					MeshDestinationsOnly: true,
				},
				AllowEnablingPermissiveMutualTLS: true,
				TLS: &MeshTLSConfig{
					Incoming: &MeshDirectionalTLSConfig{
						TLSMinVersion: types.TLSv1_1,
						TLSMaxVersion: types.TLSv1_2,
						CipherSuites: []types.TLSCipherSuite{
							types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
							types.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						},
					},
					Outgoing: &MeshDirectionalTLSConfig{
						TLSMinVersion: types.TLSv1_1,
						TLSMaxVersion: types.TLSv1_2,
						CipherSuites: []types.TLSCipherSuite{
							types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
							types.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						},
					},
				},
				HTTP: &MeshHTTPConfig{
					SanitizeXForwardedClientCert: true,
				},
				Peering: &PeeringMeshConfig{
					PeerThroughMeshGateways: true,
				},
			},
		},
		{
			name: "api-gateway",
			snake: `
				kind = "api-gateway"
				name = "foo"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
			`,
			camel: `
				Kind = "api-gateway"
				Name = "foo"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
			`,
			expect: &APIGatewayConfigEntry{
				Kind: "api-gateway",
				Name: "foo",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
			},
		},
		{
			name: "inline-certificate",
			snake: `
				kind = "inline-certificate"
				name = "foo"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
			`,
			camel: `
				Kind = "inline-certificate"
				Name = "foo"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
			`,
			expect: &InlineCertificateConfigEntry{
				Kind: "inline-certificate",
				Name: "foo",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
			},
		},
		{
			name: "http-route",
			snake: `
				kind = "http-route"
				name = "foo"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
			`,
			camel: `
				Kind = "http-route"
				Name = "foo"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
			`,
			expect: &HTTPRouteConfigEntry{
				Kind: "http-route",
				Name: "foo",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
			},
		},
		{
			name: "tcp-route",
			snake: `
				kind = "tcp-route"
				name = "foo"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
			`,
			camel: `
				Kind = "tcp-route"
				Name = "foo"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
			`,
			expect: &TCPRouteConfigEntry{
				Kind: "tcp-route",
				Name: "foo",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
			},
		},
		{
			name: "exported-services",
			snake: `
				kind = "exported-services"
				name = "foo"
				meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				services = [
					{
						name = "web"
						namespace = "foo"
						consumers = [
							{
								partition = "bar"
							},
							{
								partition = "baz"
							},
							{
								peer_name = "flarm"
							}
						]
					},
					{
						name = "db"
						namespace = "bar"
						consumers = [
							{
								partition = "zoo"
							}
						]
					}
				]
			`,
			camel: `
				Kind = "exported-services"
				Name = "foo"
				Meta {
					"foo" = "bar"
					"gir" = "zim"
				}
				Services = [
					{
						Name = "web"
						Namespace = "foo"
						Consumers = [
							{
								Partition = "bar"
							},
							{
								Partition = "baz"
							},
							{
								Peer = "flarm"
							}
						]
					},
					{
						Name = "db"
						Namespace = "bar"
						Consumers = [
							{
								Partition = "zoo"
							}
						]
					}
				]
			`,
			expect: &ExportedServicesConfigEntry{
				Name: "foo",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Services: []ExportedService{
					{
						Name:      "web",
						Namespace: "foo",
						Consumers: []ServiceConsumer{
							{
								Partition: "bar",
							},
							{
								Partition: "baz",
							},
							{
								Peer: "flarm",
							},
						},
					},
					{
						Name:      "db",
						Namespace: "bar",
						Consumers: []ServiceConsumer{
							{
								Partition: "zoo",
							},
						},
					},
				},
			},
		},
	} {
		tc := tc

		testbody := func(t *testing.T, body string) {
			var raw map[string]interface{}
			err := hcl.Decode(&raw, body)
			require.NoError(t, err)

			got, err := DecodeConfigEntry(raw)
			if tc.expectErr != "" {
				require.Nil(t, got)
				require.Error(t, err)
				requireContainsLower(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expect, got)
			}
		}

		t.Run(tc.name+" (snake case)", func(t *testing.T) {
			testbody(t, tc.snake)
		})
		t.Run(tc.name+" (camel case)", func(t *testing.T) {
			testbody(t, tc.camel)
		})
	}
}

func TestServiceConfigRequest(t *testing.T) {

	tests := []struct {
		name     string
		req      ServiceConfigRequest
		mutate   func(req *ServiceConfigRequest)
		want     *cache.RequestInfo
		wantSame bool
	}{
		{
			name: "basic params",
			req: ServiceConfigRequest{
				QueryOptions: QueryOptions{Token: "foo"},
				Datacenter:   "dc1",
			},
			want: &cache.RequestInfo{
				Token:      "foo",
				Datacenter: "dc1",
			},
			wantSame: true,
		},
		{
			name: "name should be considered",
			req: ServiceConfigRequest{
				Name: "web",
			},
			mutate: func(req *ServiceConfigRequest) {
				req.Name = "db"
			},
			wantSame: false,
		},
		{
			name: "upstreams should be different",
			req: ServiceConfigRequest{
				Name: "web",
				UpstreamServiceNames: []PeeredServiceName{
					{ServiceName: NewServiceName("foo", nil)},
				},
			},
			mutate: func(req *ServiceConfigRequest) {
				req.UpstreamServiceNames = []PeeredServiceName{
					{ServiceName: NewServiceName("foo", nil)},
					{ServiceName: NewServiceName("bar", nil)},
				}
			},
			wantSame: false,
		},
		{
			name: "upstreams should not depend on order",
			req: ServiceConfigRequest{
				Name: "web",
				UpstreamServiceNames: []PeeredServiceName{
					{ServiceName: NewServiceName("foo", nil)},
					{ServiceName: NewServiceName("bar", nil)},
				},
			},
			mutate: func(req *ServiceConfigRequest) {
				req.UpstreamServiceNames = []PeeredServiceName{
					{ServiceName: NewServiceName("foo", nil)},
					{ServiceName: NewServiceName("bar", nil)},
				}
			},
			wantSame: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := tc.req.CacheInfo()
			if tc.mutate != nil {
				tc.mutate(&tc.req)
			}
			afterInfo := tc.req.CacheInfo()

			// Check key matches or not
			if tc.wantSame {
				require.Equal(t, info, afterInfo)
			} else {
				require.NotEqual(t, info, afterInfo)
			}

			if tc.want != nil {
				// Reset key since we don't care about the actual hash value as long as
				// it does/doesn't change appropriately (asserted with wantSame above).
				info.Key = ""
				require.Equal(t, *tc.want, info)
			}
		})
	}
}

func TestServiceConfigResponse_MsgPack(t *testing.T) {
	a := ServiceConfigResponse{
		ProxyConfig: map[string]interface{}{
			"string": "foo",
			"map": map[string]interface{}{
				"baz": "bar",
			},
		},
		UpstreamConfigs: []OpaqueUpstreamConfig{
			{
				Upstream: PeeredServiceName{
					ServiceName: NewServiceName("a", acl.DefaultEnterpriseMeta()),
				},
				Config: map[string]interface{}{
					"string": "aaaa",
					"map": map[string]interface{}{
						"baz": "aa",
					},
				},
			},
			{
				Upstream: PeeredServiceName{
					ServiceName: NewServiceName("b", acl.DefaultEnterpriseMeta()),
				},
				Config: map[string]interface{}{
					"string": "bbbb",
					"map": map[string]interface{}{
						"baz": "bb",
					},
				},
			},
		},
	}

	var buf bytes.Buffer

	// Encode as msgPack using a regular handle i.e. NOT one with RawAsString
	// since our RPC codec doesn't use that.
	enc := codec.NewEncoder(&buf, MsgpackHandle)
	require.NoError(t, enc.Encode(&a))

	var b ServiceConfigResponse

	dec := codec.NewDecoder(&buf, MsgpackHandle)
	require.NoError(t, dec.Decode(&b))

	require.Equal(t, a, b)
}

func TestConfigEntryResponseMarshalling(t *testing.T) {

	cases := map[string]ConfigEntryResponse{
		"nil entry": {},
		"proxy-default entry": {
			Entry: &ProxyConfigEntry{
				Kind: ProxyDefaults,
				Name: ProxyConfigGlobal,
				Config: map[string]interface{}{
					"foo": "bar",
				},
			},
		},
		"service-default entry": {
			Entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "foo",
				Protocol: "tcp",
				// Connect:  ConnectConfiguration{SideCarProxy: true},
			},
		},
	}

	for name, tcase := range cases {
		name := name
		tcase := tcase
		t.Run(name, func(t *testing.T) {

			data, err := tcase.MarshalBinary()
			require.NoError(t, err)
			require.NotEmpty(t, data)

			var resp ConfigEntryResponse
			require.NoError(t, resp.UnmarshalBinary(data))

			require.Equal(t, tcase, resp)
		})
	}
}

func TestPassiveHealthCheck_Validate(t *testing.T) {
	tt := []struct {
		name    string
		input   PassiveHealthCheck
		wantErr bool
		wantMsg string
	}{
		{
			name:    "valid interval",
			input:   PassiveHealthCheck{Interval: 0 * time.Second},
			wantErr: false,
		},
		{
			name:    "negative interval",
			input:   PassiveHealthCheck{Interval: -1 * time.Second},
			wantErr: true,
			wantMsg: "cannot be negative",
		},
		{
			name:    "valid enforcing_consecutive_5xx",
			input:   PassiveHealthCheck{EnforcingConsecutive5xx: uintPointer(100)},
			wantErr: false,
		},
		{
			name:    "invalid enforcing_consecutive_5xx",
			input:   PassiveHealthCheck{EnforcingConsecutive5xx: uintPointer(101)},
			wantErr: true,
			wantMsg: "must be a percentage",
		},
		{
			name:    "valid max_ejection_percent",
			input:   PassiveHealthCheck{MaxEjectionPercent: uintPointer(100)},
			wantErr: false,
		},
		{
			name:    "invalid max_ejection_percent",
			input:   PassiveHealthCheck{MaxEjectionPercent: uintPointer(101)},
			wantErr: true,
			wantMsg: "must be a percentage",
		},
		{
			name:    "valid base_ejection_time",
			input:   PassiveHealthCheck{BaseEjectionTime: durationPointer(0 * time.Second)},
			wantErr: false,
		},
		{
			name:    "negative base_ejection_time",
			input:   PassiveHealthCheck{BaseEjectionTime: durationPointer(-1 * time.Second)},
			wantErr: true,
			wantMsg: "cannot be negative",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if err == nil {
				require.False(t, tc.wantErr)
				return
			}
			require.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestUpstreamLimits_Validate(t *testing.T) {
	tt := []struct {
		name    string
		input   UpstreamLimits
		wantErr bool
		wantMsg string
	}{
		{
			name:    "valid-max-conns",
			input:   UpstreamLimits{MaxConnections: intPointer(0)},
			wantErr: false,
		},
		{
			name:    "negative-max-conns",
			input:   UpstreamLimits{MaxConnections: intPointer(-1)},
			wantErr: true,
			wantMsg: "cannot be negative",
		},
		{
			name:    "valid-max-concurrent",
			input:   UpstreamLimits{MaxConcurrentRequests: intPointer(0)},
			wantErr: false,
		},
		{
			name:    "negative-max-concurrent",
			input:   UpstreamLimits{MaxConcurrentRequests: intPointer(-1)},
			wantErr: true,
			wantMsg: "cannot be negative",
		},
		{
			name:    "valid-max-pending",
			input:   UpstreamLimits{MaxPendingRequests: intPointer(0)},
			wantErr: false,
		},
		{
			name:    "negative-max-pending",
			input:   UpstreamLimits{MaxPendingRequests: intPointer(-1)},
			wantErr: true,
			wantMsg: "cannot be negative",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if err == nil {
				require.False(t, tc.wantErr)
				return
			}
			require.Contains(t, err.Error(), tc.wantMsg)
		})
	}
}

func TestServiceConfigEntry(t *testing.T) {
	cases := map[string]configEntryTestcase{
		"normalize: upstream config override no name": {
			// This will do nothing to normalization, but it will fail at validation later
			entry: &ServiceConfigEntry{
				Name: "web",
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name:     "good",
							Protocol: "grpc",
						},
						{
							Protocol: "http2",
						},
						{
							Name:     "also-good",
							Protocol: "http",
						},
					},
				},
			},
			expected: &ServiceConfigEntry{
				Kind:           ServiceDefaults,
				Name:           "web",
				EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name:           "good",
							EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
							Protocol:       "grpc",
						},
						{
							EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
							Protocol:       "http2",
						},
						{
							Name:           "also-good",
							EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
							Protocol:       "http",
						},
					},
				},
			},
			normalizeOnly: true,
		},
		"normalize: upstream config defaults with name": {
			// This will do nothing to normalization, but it will fail at validation later
			entry: &ServiceConfigEntry{
				Name: "web",
				UpstreamConfig: &UpstreamConfiguration{
					Defaults: &UpstreamConfig{
						Name:     "also-good",
						Protocol: "http2",
					},
				},
			},
			expected: &ServiceConfigEntry{
				Kind:           ServiceDefaults,
				Name:           "web",
				EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
				UpstreamConfig: &UpstreamConfiguration{
					Defaults: &UpstreamConfig{
						Name:     "also-good",
						Protocol: "http2",
					},
				},
			},
			normalizeOnly: true,
		},
		"normalize: fill-in-kind": {
			entry: &ServiceConfigEntry{
				Name: "web",
			},
			expected: &ServiceConfigEntry{
				Kind:           ServiceDefaults,
				Name:           "web",
				EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
			},
			normalizeOnly: true,
		},
		"normalize: lowercase-protocol": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "web",
				Protocol: "PrOtoCoL",
			},
			expected: &ServiceConfigEntry{
				Kind:           ServiceDefaults,
				Name:           "web",
				Protocol:       "protocol",
				EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
			},
			normalizeOnly: true,
		},
		"normalize: connect-kitchen-sink": {
			entry: &ServiceConfigEntry{
				Kind: ServiceDefaults,
				Name: "web",
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name:     "redis",
							Protocol: "TcP",
						},
						{
							Name:             "memcached",
							ConnectTimeoutMs: -1,
						},
					},
					Defaults: &UpstreamConfig{ConnectTimeoutMs: -20},
				},
				EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
			},
			expected: &ServiceConfigEntry{
				Kind: ServiceDefaults,
				Name: "web",
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name:             "redis",
							EnterpriseMeta:   *DefaultEnterpriseMetaInDefaultPartition(),
							Protocol:         "tcp",
							ConnectTimeoutMs: 0,
						},
						{
							Name:             "memcached",
							EnterpriseMeta:   *DefaultEnterpriseMetaInDefaultPartition(),
							ConnectTimeoutMs: 0,
						},
					},
					Defaults: &UpstreamConfig{
						ConnectTimeoutMs: 0,
					},
				},
				EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
			},
			normalizeOnly: true,
		},
		"wildcard name is not allowed": {
			entry: &ServiceConfigEntry{
				Name: WildcardSpecifier,
			},
			validateErr: `must be the name of a service, and not a wildcard`,
		},
		"upstream config override no name": {
			entry: &ServiceConfigEntry{
				Name: "web",
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name:     "good",
							Protocol: "grpc",
						},
						{
							Protocol: "http2",
						},
						{
							Name:     "also-good",
							Protocol: "http",
						},
					},
				},
			},
			validateErr: `Name is required`,
		},
		"upstream config defaults with name": {
			entry: &ServiceConfigEntry{
				Name: "web",
				UpstreamConfig: &UpstreamConfiguration{
					Defaults: &UpstreamConfig{
						Name:     "also-good",
						Protocol: "http2",
					},
				},
			},
			validateErr: `error in upstream defaults: Name must be empty`,
		},
		"connect-kitchen-sink": {
			entry: &ServiceConfigEntry{
				Kind: ServiceDefaults,
				Name: "web",
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name:     "redis",
							Protocol: "TcP",
						},
						{
							Name:             "memcached",
							ConnectTimeoutMs: -1,
						},
					},
					Defaults: &UpstreamConfig{ConnectTimeoutMs: -20},
				},
				EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
			},
			expected: &ServiceConfigEntry{
				Kind: ServiceDefaults,
				Name: "web",
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name:             "redis",
							EnterpriseMeta:   *DefaultEnterpriseMetaInDefaultPartition(),
							Protocol:         "tcp",
							ConnectTimeoutMs: 0,
						},
						{
							Name:             "memcached",
							EnterpriseMeta:   *DefaultEnterpriseMetaInDefaultPartition(),
							ConnectTimeoutMs: 0,
						},
					},
					Defaults: &UpstreamConfig{ConnectTimeoutMs: 0},
				},
				EnterpriseMeta: *DefaultEnterpriseMetaInDefaultPartition(),
			},
		},
		"validate: nil destination address": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: nil,
					Port:      443,
				},
			},
			validateErr: "must contain at least one valid address",
		},
		"validate: empty destination address": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: []string{},
					Port:      443,
				},
			},
			validateErr: "must contain at least one valid address",
		},
		"validate: destination ipv4 address": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: []string{"1.2.3.4"},
					Port:      443,
				},
			},
		},
		"validate: destination ipv6 address": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: []string{"2001:0db8:0000:8a2e:0370:7334:1234:5678"},
					Port:      443,
				},
			},
		},
		"valid destination shortened ipv6 address": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: []string{"2001:db8::8a2e:370:7334"},
					Port:      443,
				},
			},
		},
		"validate: invalid destination port": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: []string{"2001:db8::8a2e:370:7334"},
				},
			},
			validateErr: "Invalid Port number",
		},
		"validate: invalid hostname 1": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: []string{"*external.com"},
					Port:      443,
				},
			},
			validateErr: "Could not validate address",
		},
		"validate: invalid hostname 2": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "tcp",
				Destination: &DestinationConfig{
					Addresses: []string{"..hello."},
					Port:      443,
				},
			},
			validateErr: "Could not validate address",
		},
		"validate: all web traffic allowed": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "http",
				Destination: &DestinationConfig{
					Addresses: []string{"*"},
					Port:      443,
				},
			},
			validateErr: "Could not validate address",
		},
		"validate: multiple hostnames": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "http",
				Destination: &DestinationConfig{
					Addresses: []string{
						"api.google.com",
						"web.google.com",
					},
					Port: 443,
				},
			},
		},
		"validate: duplicate addresses not allowed": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "http",
				Destination: &DestinationConfig{
					Addresses: []string{
						"api.google.com",
						"api.google.com",
					},
					Port: 443,
				},
			},
			validateErr: "Duplicate address",
		},
		"validate: invalid inbound connection balance": {
			entry: &ServiceConfigEntry{
				Kind:                      ServiceDefaults,
				Name:                      "external",
				Protocol:                  "http",
				BalanceInboundConnections: "invalid",
			},
			validateErr: "invalid value for balance_inbound_connections",
		},
		"validate: invalid default outbound connection balance": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "http",
				UpstreamConfig: &UpstreamConfiguration{
					Defaults: &UpstreamConfig{
						BalanceOutboundConnections: "invalid",
					},
				},
			},
			validateErr: "invalid value for balance_outbound_connections",
		},
		"validate: invalid override outbound connection balance": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "http",
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name:                       "upstream",
							BalanceOutboundConnections: "invalid",
						},
					},
				},
			},
			validateErr: "invalid value for balance_outbound_connections",
		},
		"validate: invalid extension": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "http",
				EnvoyExtensions: []EnvoyExtension{
					{},
				},
			},
			validateErr: "invalid EnvoyExtensions[0]: Name is required",
		},
		"validate: invalid extension name": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "http",
				EnvoyExtensions: []EnvoyExtension{
					{
						Name: "not-a-builtin",
					},
				},
			},
			validateErr: `name "not-a-builtin" is not a built-in extension`,
		},
		"validate: valid extension": {
			entry: &ServiceConfigEntry{
				Kind:     ServiceDefaults,
				Name:     "external",
				Protocol: "http",
				EnvoyExtensions: []EnvoyExtension{
					{
						Name: api.BuiltinAWSLambdaExtension,
						Arguments: map[string]interface{}{
							"ARN": "some-arn",
						},
					},
				},
			},
		},
		"validate: invalid MutualTLSMode in service-defaults": {
			entry: &ServiceConfigEntry{
				Kind:          ServiceDefaults,
				Name:          "web",
				MutualTLSMode: MutualTLSMode("invalid-mtls-mode"),
			},
			validateErr: `Invalid MutualTLSMode "invalid-mtls-mode". Must be one of "", "strict", or "permissive".`,
		},
		"validate: invalid MutualTLSMode in proxy-defaults": {
			entry: &ServiceConfigEntry{
				Kind:          ProxyDefaults,
				Name:          ProxyConfigGlobal,
				MutualTLSMode: MutualTLSMode("invalid-mtls-mode"),
			},
			validateErr: `Invalid MutualTLSMode "invalid-mtls-mode". Must be one of "", "strict", or "permissive".`,
		},
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}

func TestUpstreamConfig_MergeInto(t *testing.T) {
	tt := []struct {
		name        string
		source      UpstreamConfig
		destination map[string]interface{}
		want        map[string]interface{}
	}{
		{
			name: "kitchen sink",
			source: UpstreamConfig{
				BalanceOutboundConnections: "exact_balance",
				EnvoyListenerJSON:          "foo",
				EnvoyClusterJSON:           "bar",
				ConnectTimeoutMs:           5,
				Protocol:                   "http",
				Limits: &UpstreamLimits{
					MaxConnections:        intPointer(3),
					MaxPendingRequests:    intPointer(4),
					MaxConcurrentRequests: intPointer(5),
				},
				PassiveHealthCheck: &PassiveHealthCheck{
					Interval:                2 * time.Second,
					MaxFailures:             3,
					EnforcingConsecutive5xx: uintPointer(4),
					MaxEjectionPercent:      uintPointer(5),
					BaseEjectionTime:        durationPointer(6 * time.Second),
				},
				MeshGateway: MeshGatewayConfig{Mode: MeshGatewayModeRemote},
			},
			destination: make(map[string]interface{}),
			want: map[string]interface{}{
				"balance_outbound_connections": "exact_balance",
				"envoy_listener_json":          "foo",
				"envoy_cluster_json":           "bar",
				"connect_timeout_ms":           5,
				"protocol":                     "http",
				"limits": &UpstreamLimits{
					MaxConnections:        intPointer(3),
					MaxPendingRequests:    intPointer(4),
					MaxConcurrentRequests: intPointer(5),
				},
				"passive_health_check": &PassiveHealthCheck{
					Interval:                2 * time.Second,
					MaxFailures:             3,
					EnforcingConsecutive5xx: uintPointer(4),
					MaxEjectionPercent:      uintPointer(5),
					BaseEjectionTime:        durationPointer(6 * time.Second),
				},
				"mesh_gateway": MeshGatewayConfig{Mode: MeshGatewayModeRemote},
			},
		},
		{
			name: "kitchen sink override of destination",
			source: UpstreamConfig{
				BalanceOutboundConnections: "exact_balance",
				EnvoyListenerJSON:          "foo",
				EnvoyClusterJSON:           "bar",
				ConnectTimeoutMs:           5,
				Protocol:                   "http",
				Limits: &UpstreamLimits{
					MaxConnections:        intPointer(3),
					MaxPendingRequests:    intPointer(4),
					MaxConcurrentRequests: intPointer(5),
				},
				PassiveHealthCheck: &PassiveHealthCheck{
					Interval:                2 * time.Second,
					MaxFailures:             3,
					EnforcingConsecutive5xx: uintPointer(4),
					MaxEjectionPercent:      uintPointer(5),
					BaseEjectionTime:        durationPointer(6 * time.Second),
				},
				MeshGateway: MeshGatewayConfig{Mode: MeshGatewayModeRemote},
			},
			destination: map[string]interface{}{
				"balance_outbound_connections": "",
				"envoy_listener_json":          "zip",
				"envoy_cluster_json":           "zap",
				"connect_timeout_ms":           10,
				"protocol":                     "grpc",
				"limits": &UpstreamLimits{
					MaxConnections:        intPointer(10),
					MaxPendingRequests:    intPointer(11),
					MaxConcurrentRequests: intPointer(12),
				},
				"passive_health_check": &PassiveHealthCheck{
					MaxFailures:             13,
					Interval:                14 * time.Second,
					EnforcingConsecutive5xx: uintPointer(15),
					MaxEjectionPercent:      uintPointer(16),
					BaseEjectionTime:        durationPointer(17 * time.Second),
				},
				"mesh_gateway": MeshGatewayConfig{Mode: MeshGatewayModeLocal},
			},
			want: map[string]interface{}{
				"balance_outbound_connections": "exact_balance",
				"envoy_listener_json":          "foo",
				"envoy_cluster_json":           "bar",
				"connect_timeout_ms":           5,
				"protocol":                     "http",
				"limits": &UpstreamLimits{
					MaxConnections:        intPointer(3),
					MaxPendingRequests:    intPointer(4),
					MaxConcurrentRequests: intPointer(5),
				},
				"passive_health_check": &PassiveHealthCheck{
					Interval:                2 * time.Second,
					MaxFailures:             3,
					EnforcingConsecutive5xx: uintPointer(4),
					MaxEjectionPercent:      uintPointer(5),
					BaseEjectionTime:        durationPointer(6 * time.Second),
				},
				"mesh_gateway": MeshGatewayConfig{Mode: MeshGatewayModeRemote},
			},
		},
		{
			name:   "empty source leaves destination intact",
			source: UpstreamConfig{},
			destination: map[string]interface{}{
				"balance_outbound_connections": "exact_balance",
				"envoy_listener_json":          "zip",
				"envoy_cluster_json":           "zap",
				"connect_timeout_ms":           10,
				"protocol":                     "grpc",
				"limits": &UpstreamLimits{
					MaxConnections:        intPointer(10),
					MaxPendingRequests:    intPointer(11),
					MaxConcurrentRequests: intPointer(12),
				},
				"passive_health_check": &PassiveHealthCheck{
					MaxFailures:             13,
					Interval:                14 * time.Second,
					EnforcingConsecutive5xx: uintPointer(15),
					MaxEjectionPercent:      uintPointer(16),
					BaseEjectionTime:        durationPointer(17 * time.Second),
				},
				"mesh_gateway": MeshGatewayConfig{Mode: MeshGatewayModeLocal},
			},
			want: map[string]interface{}{
				"balance_outbound_connections": "exact_balance",
				"envoy_listener_json":          "zip",
				"envoy_cluster_json":           "zap",
				"connect_timeout_ms":           10,
				"protocol":                     "grpc",
				"limits": &UpstreamLimits{
					MaxConnections:        intPointer(10),
					MaxPendingRequests:    intPointer(11),
					MaxConcurrentRequests: intPointer(12),
				},
				"passive_health_check": &PassiveHealthCheck{
					MaxFailures:             13,
					Interval:                14 * time.Second,
					EnforcingConsecutive5xx: uintPointer(15),
					MaxEjectionPercent:      uintPointer(16),
					BaseEjectionTime:        durationPointer(17 * time.Second),
				},
				"mesh_gateway": MeshGatewayConfig{Mode: MeshGatewayModeLocal},
			},
		},
		{
			name:        "empty source and destination is a noop",
			source:      UpstreamConfig{},
			destination: make(map[string]interface{}),
			want:        map[string]interface{}{},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tc.source.MergeInto(tc.destination)
			assert.Equal(t, tc.want, tc.destination)
		})
	}
}

func TestParseUpstreamConfig(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]interface{}
		want  UpstreamConfig
	}{
		{
			name:  "defaults - nil",
			input: nil,
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "tcp",
			},
		},
		{
			name:  "defaults - empty",
			input: map[string]interface{}{},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "tcp",
			},
		},
		{
			name: "defaults - other stuff",
			input: map[string]interface{}{
				"foo":       "bar",
				"envoy_foo": "envoy_bar",
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "tcp",
			},
		},
		{
			name: "protocol override",
			input: map[string]interface{}{
				"protocol": "http",
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Protocol:         "http",
			},
		},
		{
			name: "connect timeout override, string",
			input: map[string]interface{}{
				"connect_timeout_ms": "1000",
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 1000,
				Protocol:         "tcp",
			},
		},
		{
			name: "connect timeout override, float ",
			input: map[string]interface{}{
				"connect_timeout_ms": float64(1000.0),
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 1000,
				Protocol:         "tcp",
			},
		},
		{
			name: "connect timeout override, int ",
			input: map[string]interface{}{
				"connect_timeout_ms": 1000,
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 1000,
				Protocol:         "tcp",
			},
		},
		{
			name: "connect limits map",
			input: map[string]interface{}{
				"limits": map[string]interface{}{
					"max_connections":         50,
					"max_pending_requests":    60,
					"max_concurrent_requests": 70,
				},
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Limits: &UpstreamLimits{
					MaxConnections:        intPointer(50),
					MaxPendingRequests:    intPointer(60),
					MaxConcurrentRequests: intPointer(70),
				},
				Protocol: "tcp",
			},
		},
		{
			name: "connect limits map zero",
			input: map[string]interface{}{
				"limits": map[string]interface{}{
					"max_connections":         0,
					"max_pending_requests":    0,
					"max_concurrent_requests": 0,
				},
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				Limits: &UpstreamLimits{
					MaxConnections:        intPointer(0),
					MaxPendingRequests:    intPointer(0),
					MaxConcurrentRequests: intPointer(0),
				},
				Protocol: "tcp",
			},
		},
		{
			name: "passive health check map",
			input: map[string]interface{}{
				"passive_health_check": map[string]interface{}{
					"interval":                  "22s",
					"max_failures":              7,
					"enforcing_consecutive_5xx": 8,
					"max_ejection_percent":      9,
					"base_ejection_time":        "10s",
				},
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				PassiveHealthCheck: &PassiveHealthCheck{
					Interval:                22 * time.Second,
					MaxFailures:             7,
					EnforcingConsecutive5xx: uintPointer(8),
					MaxEjectionPercent:      uintPointer(9),
					BaseEjectionTime:        durationPointer(10 * time.Second),
				},
				Protocol: "tcp",
			},
		},
		{
			name: "mesh gateway map",
			input: map[string]interface{}{
				"mesh_gateway": map[string]interface{}{
					"Mode": "remote",
				},
			},
			want: UpstreamConfig{
				ConnectTimeoutMs: 5000,
				MeshGateway: MeshGatewayConfig{
					Mode: MeshGatewayModeRemote,
				},
				Protocol: "tcp",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUpstreamConfig(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestProxyConfigEntry(t *testing.T) {
	cases := map[string]configEntryTestcase{
		"proxy config name provided is not global": {
			entry: &ProxyConfigEntry{
				Name: "foo",
			},
			normalizeErr: `invalid name ("foo"), only "global" is supported`,
		},
		"proxy config has no name": {
			entry: &ProxyConfigEntry{
				Name: "",
			},
			expected: &ProxyConfigEntry{
				Name:           ProxyConfigGlobal,
				Kind:           ProxyDefaults,
				EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
			},
		},
		"proxy config entry has invalid opaque config": {
			entry: &ProxyConfigEntry{
				Name: "global",
				Config: map[string]interface{}{
					"envoy_hcp_metrics_bind_socket_dir": "/Consul/is/a/networking/platform/that/enables/securing/your/networking/",
				},
			},
			validateErr: "Config: envoy_hcp_metrics_bind_socket_dir length 71 exceeds max",
		},
		"proxy config has invalid failover policy": {
			entry: &ProxyConfigEntry{
				Name:           "global",
				FailoverPolicy: &ServiceResolverFailoverPolicy{Mode: "bad"},
			},
			validateErr: `Failover-policy mode must be one of '', 'sequential', or 'order-by-locality'`,
		},
		"proxy config with valid failover policy": {
			entry: &ProxyConfigEntry{
				Name:           "global",
				FailoverPolicy: &ServiceResolverFailoverPolicy{Mode: "order-by-locality"},
			},
			expected: &ProxyConfigEntry{
				Name:           ProxyConfigGlobal,
				Kind:           ProxyDefaults,
				FailoverPolicy: &ServiceResolverFailoverPolicy{Mode: "order-by-locality"},
				EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
			},
		},
		"proxy config has invalid access log type": {
			entry: &ProxyConfigEntry{
				Name: "global",
				AccessLogs: AccessLogsConfig{
					Enabled: true,
					Type:    "stdin",
				},
			},
			validateErr: "invalid access log type: stdin",
		},
		"proxy config has invalid access log config - both text and json formats": {
			entry: &ProxyConfigEntry{
				Name: "global",
				AccessLogs: AccessLogsConfig{
					Enabled:    true,
					JSONFormat: "[%START_TIME%]",
					TextFormat: "{\"start_time\": \"[%START_TIME%]\"}",
				},
			},
			validateErr: "cannot specify both access log JSONFormat and TextFormat",
		},
		"proxy config has invalid access log config - file path with wrong type": {
			entry: &ProxyConfigEntry{
				Name: "global",
				AccessLogs: AccessLogsConfig{
					Enabled: true,
					Path:    "/tmp/logs.txt",
				},
			},
			validateErr: "path is only valid for file type access logs",
		},
		"proxy config has invalid access log config - no file path specified": {
			entry: &ProxyConfigEntry{
				Name: "global",
				AccessLogs: AccessLogsConfig{
					Enabled: true,
					Type:    FileLogSinkType,
				},
			},
			validateErr: "path must be specified when using file type access logs",
		},
		"proxy config has invalid access log JSON format": {
			entry: &ProxyConfigEntry{
				Name: "global",
				AccessLogs: AccessLogsConfig{
					Enabled:    true,
					JSONFormat: "{\"start_time\": \"[%START_TIME%]\"", // Missing trailing brace
				},
			},
			validateErr: "invalid access log json for JSON format",
		},
	}
	testConfigEntryNormalizeAndValidate(t, cases)
}

func requireContainsLower(t *testing.T, haystack, needle string) {
	t.Helper()
	require.Contains(t, strings.ToLower(haystack), strings.ToLower(needle))
}

func TestConfigEntryQuery_CacheInfoKey(t *testing.T) {
	assertCacheInfoKeyIsComplete(t, &ConfigEntryQuery{})
}

func TestServiceConfigRequest_CacheInfoKey(t *testing.T) {
	assertCacheInfoKeyIsComplete(t, &ServiceConfigRequest{})
}

func TestDiscoveryChainRequest_CacheInfoKey(t *testing.T) {
	assertCacheInfoKeyIsComplete(t, &DiscoveryChainRequest{})
}

type configEntryTestcase struct {
	entry         ConfigEntry
	normalizeOnly bool

	normalizeErr string
	validateErr  string

	// Only one of expected, expectUnchanged or check can be set.
	expected        ConfigEntry
	expectUnchanged bool
	// check is called between normalize and validate
	check func(t *testing.T, entry ConfigEntry)
}

func testConfigEntryNormalizeAndValidate(t *testing.T, cases map[string]configEntryTestcase) {
	t.Helper()

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			beforeNormalize, err := copystructure.Copy(tc.entry)
			require.NoError(t, err)

			err = tc.entry.Normalize()
			if tc.normalizeErr != "" {
				testutil.RequireErrorContains(t, err, tc.normalizeErr)
				return
			}
			require.NoError(t, err)

			checkMethods := 0
			if tc.expected != nil {
				checkMethods++
			}
			if tc.expectUnchanged {
				checkMethods++
			}
			if tc.check != nil {
				checkMethods++
			}

			if checkMethods > 1 {
				t.Fatal("cannot set more than one of 'expected', 'expectUnchanged' and 'check' test case fields")
			}

			if tc.expected != nil {
				require.Equal(t, tc.expected, tc.entry)
			}

			if tc.expectUnchanged {
				// EnterpriseMeta.Normalize behaves differently in Ent and OSS which
				// causes an exact comparison to fail. It's still useful to assert that
				// nothing else changes though during Normalize. So we ignore
				// EnterpriseMeta Defaults.
				opts := cmp.Options{
					cmp.Comparer(func(a, b acl.EnterpriseMeta) bool {
						return a.IsSame(&b)
					}),
				}
				if diff := cmp.Diff(beforeNormalize, tc.entry, opts); diff != "" {
					t.Fatalf("expect unchanged after Normalize, got diff:\n%s", diff)
				}
			}

			if tc.check != nil {
				tc.check(t, tc.entry)
			}

			if tc.normalizeOnly {
				return
			}

			err = tc.entry.Validate()
			if tc.validateErr != "" {
				testutil.RequireErrorContains(t, err, tc.validateErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func intPointer(i int) *int {
	return &i
}

func uintPointer(v uint32) *uint32 {
	return &v
}

func durationPointer(d time.Duration) *time.Duration {
	return &d
}

func TestValidateOpaqueConfigMap(t *testing.T) {
	tt := map[string]struct {
		input     map[string]interface{}
		expectErr string
	}{
		"hcp metrics socket dir is valid": {
			input: map[string]interface{}{
				"envoy_hcp_metrics_bind_socket_dir": "/etc/consul.d/hcp"},
			expectErr: "",
		},
		"hcp metrics socket dir is too long": {
			input: map[string]interface{}{
				"envoy_hcp_metrics_bind_socket_dir": "/Consul/is/a/networking/platform/that/enables/securing/your/networking/",
			},
			expectErr: "envoy_hcp_metrics_bind_socket_dir length 71 exceeds max 70",
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			err := validateOpaqueProxyConfig(tc.input)
			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
