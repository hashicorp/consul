package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestAPI_ConfigEntries(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	config_entries := c.ConfigEntries()

	t.Run("Proxy Defaults", func(t *testing.T) {
		global_proxy := &ProxyConfigEntry{
			Kind: ProxyDefaults,
			Name: ProxyConfigGlobal,
			Config: map[string]interface{}{
				"foo": "bar",
				"bar": 1.0,
			},
			Meta: map[string]string{
				"foo": "bar",
				"gir": "zim",
			},
		}

		// set it
		_, wm, err := config_entries.Set(global_proxy, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// get it
		entry, qm, err := config_entries.Get(ProxyDefaults, ProxyConfigGlobal, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		// verify it
		readProxy, ok := entry.(*ProxyConfigEntry)
		require.True(t, ok)
		require.Equal(t, global_proxy.Kind, readProxy.Kind)
		require.Equal(t, global_proxy.Name, readProxy.Name)
		require.Equal(t, global_proxy.Config, readProxy.Config)
		require.Equal(t, global_proxy.Meta, readProxy.Meta)
		require.Equal(t, global_proxy.Meta, readProxy.GetMeta())

		global_proxy.Config["baz"] = true
		// CAS update fail
		written, _, err := config_entries.CAS(global_proxy, 0, nil)
		require.NoError(t, err)
		require.False(t, written)

		// CAS update success
		written, wm, err = config_entries.CAS(global_proxy, readProxy.ModifyIndex, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)
		require.NoError(t, err)
		require.True(t, written)

		// Non CAS update
		global_proxy.Config["baz"] = "baz"
		_, wm, err = config_entries.Set(global_proxy, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// list it
		entries, qm, err := config_entries.List(ProxyDefaults, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)
		require.Len(t, entries, 1)
		readProxy, ok = entries[0].(*ProxyConfigEntry)
		require.True(t, ok)
		require.Equal(t, global_proxy.Kind, readProxy.Kind)
		require.Equal(t, global_proxy.Name, readProxy.Name)
		require.Equal(t, global_proxy.Config, readProxy.Config)

		// delete it
		wm, err = config_entries.Delete(ProxyDefaults, ProxyConfigGlobal, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		_, _, err = config_entries.Get(ProxyDefaults, ProxyConfigGlobal, nil)
		require.Error(t, err)
	})

	t.Run("Service Defaults", func(t *testing.T) {
		service := &ServiceConfigEntry{
			Kind:     ServiceDefaults,
			Name:     "foo",
			Protocol: "udp",
			Meta: map[string]string{
				"foo": "bar",
				"gir": "zim",
			},
			MaxInboundConnections:     5,
			BalanceInboundConnections: "exact_balance",
			LocalConnectTimeoutMs:     5000,
			LocalRequestTimeoutMs:     7000,
		}

		dest := &DestinationConfig{
			Addresses: []string{"my.example.com"},
			Port:      80,
		}

		service2 := &ServiceConfigEntry{
			Kind:        ServiceDefaults,
			Name:        "bar",
			Protocol:    "tcp",
			Destination: dest,
		}

		// set it
		_, wm, err := config_entries.Set(service, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// also set the second one
		_, wm, err = config_entries.Set(service2, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// get it
		entry, qm, err := config_entries.Get(ServiceDefaults, "foo", nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		// verify it
		readService, ok := entry.(*ServiceConfigEntry)
		require.True(t, ok)
		require.Equal(t, service.Kind, readService.Kind)
		require.Equal(t, service.Name, readService.Name)
		require.Equal(t, service.Protocol, readService.Protocol)
		require.Equal(t, service.Meta, readService.Meta)
		require.Equal(t, service.Meta, readService.GetMeta())
		require.Equal(t, service.MaxInboundConnections, readService.MaxInboundConnections)
		require.Equal(t, service.BalanceInboundConnections, readService.BalanceInboundConnections)
		require.Equal(t, service.LocalConnectTimeoutMs, readService.LocalConnectTimeoutMs)
		require.Equal(t, service.LocalRequestTimeoutMs, readService.LocalRequestTimeoutMs)

		// update it
		service.Protocol = "tcp"

		// CAS fail
		written, _, err := config_entries.CAS(service, 0, nil)
		require.NoError(t, err)
		require.False(t, written)

		// CAS success
		written, wm, err = config_entries.CAS(service, readService.ModifyIndex, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)
		require.True(t, written)

		// update no cas
		service.Protocol = "http"

		_, wm, err = config_entries.Set(service, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// list them
		entries, qm, err := config_entries.List(ServiceDefaults, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)
		require.Len(t, entries, 2)

		for _, entry = range entries {
			switch entry.GetName() {
			case "foo":
				// this also verifies that the update value was persisted and
				// the updated values are seen
				readService, ok = entry.(*ServiceConfigEntry)
				require.True(t, ok)
				require.Equal(t, service.Kind, readService.Kind)
				require.Equal(t, service.Name, readService.Name)
				require.Equal(t, service.Protocol, readService.Protocol)
			case "bar":
				readService, ok = entry.(*ServiceConfigEntry)
				require.True(t, ok)
				require.Equal(t, service2.Kind, readService.Kind)
				require.Equal(t, service2.Name, readService.Name)
				require.Equal(t, service2.Protocol, readService.Protocol)
				require.Equal(t, dest, readService.Destination)
			}
		}

		// delete it
		wm, err = config_entries.Delete(ServiceDefaults, "foo", nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// verify deletion
		_, _, err = config_entries.Get(ServiceDefaults, "foo", nil)
		require.Error(t, err)
	})

	t.Run("Mesh", func(t *testing.T) {
		mesh := &MeshConfigEntry{
			TransparentProxy: TransparentProxyMeshConfig{MeshDestinationsOnly: true},
			Meta: map[string]string{
				"foo": "bar",
				"gir": "zim",
			},
			Partition: defaultPartition,
			Namespace: defaultNamespace,
		}
		ce := c.ConfigEntries()

		testutil.RunStep(t, "set and get", func(t *testing.T) {
			_, wm, err := ce.Set(mesh, nil)
			require.NoError(t, err)
			require.NotNil(t, wm)
			require.NotEqual(t, 0, wm.RequestTime)

			entry, qm, err := ce.Get(MeshConfig, MeshConfigMesh, nil)
			require.NoError(t, err)
			require.NotNil(t, qm)
			require.NotEqual(t, 0, qm.RequestTime)

			result, ok := entry.(*MeshConfigEntry)
			require.True(t, ok)

			// ignore indexes
			result.CreateIndex = 0
			result.ModifyIndex = 0
			require.Equal(t, mesh, result)
		})

		testutil.RunStep(t, "list", func(t *testing.T) {
			entries, qm, err := ce.List(MeshConfig, nil)
			require.NoError(t, err)
			require.NotNil(t, qm)
			require.NotEqual(t, 0, qm.RequestTime)
			require.Len(t, entries, 1)
		})

		testutil.RunStep(t, "delete", func(t *testing.T) {
			wm, err := ce.Delete(MeshConfig, MeshConfigMesh, nil)
			require.NoError(t, err)
			require.NotNil(t, wm)
			require.NotEqual(t, 0, wm.RequestTime)

			// verify deletion
			_, _, err = ce.Get(MeshConfig, MeshConfigMesh, nil)
			require.Error(t, err)
		})
	})

	t.Run("CAS deletion", func(t *testing.T) {

		entry := &ProxyConfigEntry{
			Kind: ProxyDefaults,
			Name: ProxyConfigGlobal,
			Config: map[string]interface{}{
				"foo": "bar",
			},
		}

		// Create a config entry.
		created, _, err := config_entries.Set(entry, nil)
		require.NoError(t, err)
		require.True(t, created, "entry should have been created")

		// Read it back to get the ModifyIndex.
		result, _, err := config_entries.Get(entry.Kind, entry.Name, nil)
		require.NoError(t, err)
		require.NotNil(t, entry)

		// Attempt a deletion with an invalid index.
		deleted, _, err := config_entries.DeleteCAS(entry.Kind, entry.Name, result.GetModifyIndex()-1, nil)
		require.NoError(t, err)
		require.False(t, deleted, "entry should not have been deleted")

		// Attempt a deletion with a valid index.
		deleted, _, err = config_entries.DeleteCAS(entry.Kind, entry.Name, result.GetModifyIndex(), nil)
		require.NoError(t, err)
		require.True(t, deleted, "entry should have been deleted")
	})
}

func TestDecodeConfigEntry(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		body      string
		expect    ConfigEntry
		expectErr string
	}{
		{
			name: "expose-paths: kitchen sink proxy",
			body: `
			{
				"Kind": "proxy-defaults",
				"Name": "global",
				"Expose": {
					"Checks": true,
					"Paths": [
						{
							"LocalPathPort": 8080,
							"ListenerPort": 21500,
							"Path": "/healthz",
							"Protocol": "http2"
						}
					]
				}
			}
			`,
			expect: &ProxyConfigEntry{
				Kind: "proxy-defaults",
				Name: "global",
				Expose: ExposeConfig{
					Checks: true,
					Paths: []ExposePath{
						{
							LocalPathPort: 8080,
							ListenerPort:  21500,
							Path:          "/healthz",
							Protocol:      "http2",
						},
					},
				},
			},
		},
		{
			name: "expose-paths: kitchen sink service default",
			body: `
			{
				"Kind": "service-defaults",
				"Name": "global",
				"Expose": {
					"Checks": true,
					"Paths": [
						{
							"LocalPathPort": 8080,
							"ListenerPort": 21500,
							"Path": "/healthz",
							"Protocol": "http2"
						}
					]
				}
			}
			`,
			expect: &ServiceConfigEntry{
				Kind: "service-defaults",
				Name: "global",
				Expose: ExposeConfig{
					Checks: true,
					Paths: []ExposePath{
						{
							LocalPathPort: 8080,
							ListenerPort:  21500,
							Path:          "/healthz",
							Protocol:      "http2",
						},
					},
				},
			},
		},
		{
			name: "proxy-defaults",
			body: `
			{
				"Kind": "proxy-defaults",
				"Name": "main",
				"Meta" : {
					"foo": "bar",
					"gir": "zim"
				},
				"Config": {
				  "foo": 19,
				  "bar": "abc",
				  "moreconfig": {
					"moar": "config"
				  }
				},
				"MeshGateway": {
					"Mode": "remote"
				},
				"Mode": "transparent",
				"TransparentProxy": {
					"OutboundListenerPort": 808,
					"DialedDirectly": true
				},
				"AccessLogs": {
					"Enabled": true,
					"DisableListenerLogs": true,
					"Type": "file",
					"Path": "/tmp/logs.txt",
					"TextFormat": "[%START_TIME%]"
				}
			}
			`,
			expect: &ProxyConfigEntry{
				Kind: "proxy-defaults",
				Name: "main",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Config: map[string]interface{}{
					"foo": float64(19),
					"bar": "abc",
					"moreconfig": map[string]interface{}{
						"moar": "config",
					},
				},
				MeshGateway: MeshGatewayConfig{
					Mode: MeshGatewayModeRemote,
				},
				Mode: ProxyModeTransparent,
				TransparentProxy: &TransparentProxyConfig{
					OutboundListenerPort: 808,
					DialedDirectly:       true,
				},
				AccessLogs: &AccessLogsConfig{
					Enabled:             true,
					DisableListenerLogs: true,
					Type:                FileLogSinkType,
					Path:                "/tmp/logs.txt",
					TextFormat:          "[%START_TIME%]",
				},
			},
		},
		{
			name: "service-defaults",
			body: `
			{
				"Kind": "service-defaults",
				"Name": "main",
				"Meta" : {
					"foo": "bar",
					"gir": "zim"
				},
				"Protocol": "http",
				"ExternalSNI": "abc-123",
				"MeshGateway": {
					"Mode": "remote"
				},
				"Mode": "transparent",
				"TransparentProxy": {
					"OutboundListenerPort": 808,
					"DialedDirectly": true
				},
				"BalanceInboundConnections": "exact_balance",
				"UpstreamConfig": {
					"Overrides": [
						{
							"Name": "redis",
							"PassiveHealthCheck": {
								"MaxFailures": 3,
								"Interval": "2s",
								"EnforcingConsecutive5xx": 60,
								"MaxEjectionPercent": 4,
								"BaseEjectionTime": "5s"
							},
							"BalanceOutboundConnections": "exact_balance"
						},
						{
							"Name": "finance--billing",
							"MeshGateway": {
								"Mode": "remote"
							}
						}
					],
					"Defaults": {
						"EnvoyClusterJSON": "zip",
						"EnvoyListenerJSON": "zop",
						"ConnectTimeoutMs": 5000,
						"Protocol": "http",
						"Limits": {
							"MaxConnections": 3,
							"MaxPendingRequests": 4,
							"MaxConcurrentRequests": 5
						},
						"PassiveHealthCheck": {
								"MaxFailures": 5,
								"Interval": "4s",
								"EnforcingConsecutive5xx": 61,
								"MaxEjectionPercent": 5,
								"BaseEjectionTime": "6s"
						}
					}
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
				Mode: ProxyModeTransparent,
				TransparentProxy: &TransparentProxyConfig{
					OutboundListenerPort: 808,
					DialedDirectly:       true,
				},
				BalanceInboundConnections: "exact_balance",
				UpstreamConfig: &UpstreamConfiguration{
					Overrides: []*UpstreamConfig{
						{
							Name: "redis",
							PassiveHealthCheck: &PassiveHealthCheck{
								MaxFailures:             3,
								Interval:                2 * time.Second,
								EnforcingConsecutive5xx: uint32Pointer(60),
								MaxEjectionPercent:      uint32Pointer(4),
								BaseEjectionTime:        durationPointer(5 * time.Second),
							},
							BalanceOutboundConnections: "exact_balance",
						},
						{
							Name:        "finance--billing",
							MeshGateway: MeshGatewayConfig{Mode: "remote"},
						},
					},
					Defaults: &UpstreamConfig{
						EnvoyClusterJSON:  "zip",
						EnvoyListenerJSON: "zop",
						Protocol:          "http",
						ConnectTimeoutMs:  5000,
						Limits: &UpstreamLimits{
							MaxConnections:        intPointer(3),
							MaxPendingRequests:    intPointer(4),
							MaxConcurrentRequests: intPointer(5),
						},
						PassiveHealthCheck: &PassiveHealthCheck{
							MaxFailures:             5,
							Interval:                4 * time.Second,
							EnforcingConsecutive5xx: uint32Pointer(61),
							MaxEjectionPercent:      uint32Pointer(5),
							BaseEjectionTime:        durationPointer(6 * time.Second),
						},
					},
				},
			},
		},
		{
			name: "service-defaults-endpoint",
			body: `
			{
				"Kind": "service-defaults",
				"Name": "external",
				"Protocol": "http",
				"Destination": {
					"Addresses": [
						"1.2.3.4"
					],
					"Port": 443
				}
			}
			`,
			expect: &ServiceConfigEntry{
				Kind:     "service-defaults",
				Name:     "external",
				Protocol: "http",
				Destination: &DestinationConfig{
					Addresses: []string{"1.2.3.4"},
					Port:      443,
				},
			},
		},
		{
			name: "service-router: kitchen sink",
			body: `
			{
				"Kind": "service-router",
				"Name": "main",
				"Meta" : {
					"foo": "bar",
					"gir": "zim"
				},
				"Routes": [
					{
						"Match": {
							"HTTP": {
								"PathExact": "/foo",
								"Header": [
									{
										"Name": "debug1",
										"Present": true
									},
									{
										"Name": "debug2",
										"Present": false,
										"Invert": true
									},
									{
										"Name": "debug3",
										"Exact": "1"
									},
									{
										"Name": "debug4",
										"Prefix": "aaa"
									},
									{
										"Name": "debug5",
										"Suffix": "bbb"
									},
									{
										"Name": "debug6",
										"Regex": "a.*z"
									}
								]
							}
						},
						"Destination": {
						  "Service": "carrot",
						  "ServiceSubset": "kale",
						  "Namespace": "leek",
						  "PrefixRewrite": "/alternate",
						  "RequestTimeout": "99s",
						  "IdleTimeout": "99s",
						  "NumRetries": 12345,
						  "RetryOnConnectFailure": true,
						  "RetryOnStatusCodes": [401, 209]
						}
					},
					{
						"Match": {
							"HTTP": {
								"PathPrefix": "/foo",
								"Methods": [ "GET", "DELETE" ],
								"QueryParam": [
									{
										"Name": "hack1",
										"Present": true
									},
									{
										"Name": "hack2",
										"Exact": "1"
									},
									{
										"Name": "hack3",
										"Regex": "a.*z"
									}
								]
							}
						}
					},
					{
						"Match": {
							"HTTP": {
								"PathRegex": "/foo"
							}
						}
					}
				]
			}
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
			body: `
			{
				"Kind": "service-splitter",
				"Name": "main",
				"Meta" : {
					"foo": "bar",
					"gir": "zim"
				},
				"Splits": [
				  {
					"Weight": 99.1,
					"ServiceSubset": "v1"
				  },
				  {
					"Weight": 0.9,
					"Service": "other",
					"Namespace": "alt"
				  }
				]
			}
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
			body: `
			{
				"Kind": "service-resolver",
				"Name": "main",
				"Meta" : {
					"foo": "bar",
					"gir": "zim"
				},
				"DefaultSubset": "v1",
				"ConnectTimeout": "15s",
				"Subsets": {
					"v1": {
						"Filter": "Service.Meta.version == v1"
					},
					"v2": {
						"Filter": "Service.Meta.version == v2",
						"OnlyPassing": true
					}
				},
				"Failover": {
					"v2": {
						"Service": "failcopy",
						"ServiceSubset": "sure",
						"Namespace": "neighbor",
						"Datacenters": ["dc5", "dc14"]
					},
					"*": {
						"Datacenters": ["dc7"]
					}
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
			body: `
			{
				"Kind": "service-resolver",
				"Name": "main",
				"Redirect": {
					"Service": "other",
					"ServiceSubset": "backup",
					"Namespace": "alt",
					"Datacenter": "dc9"
				}
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
			body: `
			{
				"Kind": "service-resolver",
				"Name": "main"
			}
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
			},
		},
		{
			name: "service-resolver: envoy hash lb kitchen sink",
			body: `
			{
				"Kind": "service-resolver",
				"Name": "main",
				"LoadBalancer": {
					"Policy": "ring_hash",
					"RingHashConfig": {
						"MinimumRingSize": 1,
						"MaximumRingSize": 2
					},
					"HashPolicies": [
						{
							"Field": "cookie",
							"FieldValue": "good-cookie",
							"CookieConfig": {
								"TTL": "1s",
								"Path": "/oven"
							},
							"Terminal": true
						},
						{
							"Field": "cookie",
							"FieldValue": "less-good-cookie",
							"CookieConfig": {
								"Session": true,
								"Path": "/toaster"
							},
							"Terminal": true
						},
						{
							"Field": "header",
							"FieldValue": "x-user-id"
						},
						{
							"SourceIP": true
						}
					]
				}
			}
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				LoadBalancer: &LoadBalancer{
					Policy: "ring_hash",
					RingHashConfig: &RingHashConfig{
						MinimumRingSize: 1,
						MaximumRingSize: 2,
					},
					HashPolicies: []HashPolicy{
						{
							Field:      "cookie",
							FieldValue: "good-cookie",
							CookieConfig: &CookieConfig{
								TTL:  1 * time.Second,
								Path: "/oven",
							},
							Terminal: true,
						},
						{
							Field:      "cookie",
							FieldValue: "less-good-cookie",
							CookieConfig: &CookieConfig{
								Session: true,
								Path:    "/toaster",
							},
							Terminal: true,
						},
						{
							Field:      "header",
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
			body: `
			{
				"Kind": "service-resolver",
				"Name": "main",
				"LoadBalancer": {
					"Policy": "least_request",
					"LeastRequestConfig": {
						"ChoiceCount": 2
					}
				}
			}
			`,
			expect: &ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				LoadBalancer: &LoadBalancer{
					Policy: "least_request",
					LeastRequestConfig: &LeastRequestConfig{
						ChoiceCount: 2,
					},
				},
			},
		},
		{
			// TODO(rb): test SDS stuff here in both places (global/service)
			name: "ingress-gateway",
			body: `
			{
				"Kind": "ingress-gateway",
				"Name": "ingress-web",
				"Meta" : {
					"foo": "bar",
					"gir": "zim"
				},
				"Tls": {
					"Enabled": true,
					"TLSMinVersion": "TLSv1_1",
					"TLSMaxVersion": "TLSv1_2",
					"CipherSuites": [
						"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
						"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
					]
				},
				"Listeners": [
					{
						"Port": 8080,
						"Protocol": "http",
						"Services": [
							{
								"Name": "web",
								"Namespace": "foo",
								"Partition": "bar"
							},
							{
								"Name": "db"
							}
						]
					},
					{
						"Port": 9999,
						"Protocol": "tcp",
						"Services": [
							{
								"Name": "mysql"
							}
						]
					}
				]
			}
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
					TLSMinVersion: "TLSv1_1",
					TLSMaxVersion: "TLSv1_2",
					CipherSuites: []string{
						"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
						"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
					},
				},
				Listeners: []IngressListener{
					{
						Port:     8080,
						Protocol: "http",
						Services: []IngressService{
							{
								Name:      "web",
								Namespace: "foo",
								Partition: "bar",
							},
							{
								Name: "db",
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
				},
			},
		},
		{
			name: "terminating-gateway",
			body: `
			{
				"Kind": "terminating-gateway",
				"Name": "terminating-west",
				"Meta" : {
					"foo": "bar",
					"gir": "zim"
				},
				"Services": [
					{
						"Namespace": "foo",
						"Name": "web",
						"CAFile": "/etc/ca.pem",
						"CertFile": "/etc/cert.pem",
						"KeyFile": "/etc/tls.key",
						"SNI": "mydomain"
					},
					{
						"Name": "api"
					},
					{
						"Namespace": "bar",
						"Name": "*"
					}
				]
			}`,
			expect: &TerminatingGatewayConfigEntry{
				Kind: "terminating-gateway",
				Name: "terminating-west",
				Meta: map[string]string{
					"foo": "bar",
					"gir": "zim",
				},
				Services: []LinkedService{
					{
						Namespace: "foo",
						Name:      "web",
						CAFile:    "/etc/ca.pem",
						CertFile:  "/etc/cert.pem",
						KeyFile:   "/etc/tls.key",
						SNI:       "mydomain",
					},
					{
						Name: "api",
					},
					{
						Namespace: "bar",
						Name:      "*",
					},
				},
			},
		},
		{
			name: "service-intentions: kitchen sink",
			body: `
			{
				"Kind": "service-intentions",
				"Name": "web",
				"Meta" : {
					"foo": "bar",
					"gir": "zim"
				},
				"Sources": [
					{
						"Name": "foo",
						"Action": "deny",
						"Type": "consul",
						"Description": "foo desc"
					},
					{
						"Name": "bar",
						"Action": "allow",
						"Description": "bar desc"
					},
					{
						"Name": "l7",
						"Permissions": [
							{
								"Action": "deny",
								"HTTP": {
									"PathExact": "/admin",
									"Header": [
										{
											"Name": "hdr-present",
											"Present": true
										},
										{
											"Name": "hdr-exact",
											"Exact": "exact"
										},
										{
											"Name": "hdr-prefix",
											"Prefix": "prefix"
										},
										{
											"Name": "hdr-suffix",
											"Suffix": "suffix"
										},
										{
											"Name": "hdr-regex",
											"Regex": "regex"
										},
										{
											"Name": "hdr-absent",
											"Present": true,
											"Invert": true
										}
									]
								}
							},
							{
								"Action": "allow",
								"HTTP": {
									"PathPrefix": "/v3/"
								}
							},
							{
								"Action": "allow",
								"HTTP": {
									"PathRegex": "/v[12]/.*",
									"Methods": [
										"GET",
										"POST"
									]
								}
							}
						]
					},
					{
						"Name": "*",
						"Action": "deny",
						"Description": "wild desc"
					}
				]
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
			name: "mesh",
			body: `
			{
				"Kind": "mesh",
				"Meta" : {
					"foo": "bar",
					"gir": "zim"
				},
				"TransparentProxy": {
					"MeshDestinationsOnly": true
				},
				"TLS": {
					"Incoming": {
						"TLSMinVersion": "TLSv1_1",
						"TLSMaxVersion": "TLSv1_2",
						"CipherSuites": [
							"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
						]
					},
					"Outgoing": {
						"TLSMinVersion": "TLSv1_1",
						"TLSMaxVersion": "TLSv1_2",
						"CipherSuites": [
							"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
						]
					}
				},
				"HTTP": {
					"SanitizeXForwardedClientCert": true
				},
				"Peering": {
					"PeerThroughMeshGateways": true
				}
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
				TLS: &MeshTLSConfig{
					Incoming: &MeshDirectionalTLSConfig{
						TLSMinVersion: "TLSv1_1",
						TLSMaxVersion: "TLSv1_2",
						CipherSuites: []string{
							"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
						},
					},
					Outgoing: &MeshDirectionalTLSConfig{
						TLSMinVersion: "TLSv1_1",
						TLSMaxVersion: "TLSv1_2",
						CipherSuites: []string{
							"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
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
	} {
		tc := tc

		t.Run(tc.name+": DecodeConfigEntry", func(t *testing.T) {
			var raw map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(tc.body), &raw))

			got, err := DecodeConfigEntry(raw)
			require.NoError(t, err)
			require.Equal(t, tc.expect, got)
		})

		t.Run(tc.name+": DecodeConfigEntryFromJSON", func(t *testing.T) {
			got, err := DecodeConfigEntryFromJSON([]byte(tc.body))
			require.NoError(t, err)
			require.Equal(t, tc.expect, got)
		})

		t.Run(tc.name+": DecodeConfigEntrySlice", func(t *testing.T) {
			var raw []map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte("["+tc.body+"]"), &raw))

			got, err := decodeConfigEntrySlice(raw)
			require.NoError(t, err)
			require.Len(t, got, 1)
			require.Equal(t, tc.expect, got[0])
		})
	}
}

func intPointer(v int) *int {
	return &v
}

func uint32Pointer(v uint32) *uint32 {
	return &v
}

func durationPointer(d time.Duration) *time.Duration {
	return &d
}
