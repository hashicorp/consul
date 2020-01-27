package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

		entry, qm, err = config_entries.Get(ProxyDefaults, ProxyConfigGlobal, nil)
		require.Error(t, err)
	})

	t.Run("Service Defaults", func(t *testing.T) {
		service := &ServiceConfigEntry{
			Kind:     ServiceDefaults,
			Name:     "foo",
			Protocol: "udp",
		}

		service2 := &ServiceConfigEntry{
			Kind:     ServiceDefaults,
			Name:     "bar",
			Protocol: "tcp",
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
			}
		}

		// delete it
		wm, err = config_entries.Delete(ServiceDefaults, "foo", nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// verify deletion
		entry, qm, err = config_entries.Get(ServiceDefaults, "foo", nil)
		require.Error(t, err)
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
				"Config": {
				  "foo": 19,
				  "bar": "abc",
				  "moreconfig": {
					"moar": "config"
				  }
				},
				"MeshGateway": {
					"Mode": "remote"
				}
			}
			`,
			expect: &ProxyConfigEntry{
				Kind: "proxy-defaults",
				Name: "main",
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
			},
		},
		{
			name: "service-defaults",
			body: `
			{
				"Kind": "service-defaults",
				"Name": "main",
				"Protocol": "http",
				"ExternalSNI": "abc-123",
				"MeshGateway": {
					"Mode": "remote"
				}
			}
			`,
			expect: &ServiceConfigEntry{
				Kind:        "service-defaults",
				Name:        "main",
				Protocol:    "http",
				ExternalSNI: "abc-123",
				MeshGateway: MeshGatewayConfig{
					Mode: MeshGatewayModeRemote,
				},
			},
		},
		{
			name: "service-router: kitchen sink",
			body: `
			{
				"Kind": "service-router",
				"Name": "main",
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
				Kind:           "service-resolver",
				Name:           "main",
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
