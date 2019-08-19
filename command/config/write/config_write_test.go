package write

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestConfigWrite_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestConfigWrite(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	t.Run("File", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)

		f := testutil.TempFile(t, "config-write-svc-web.hcl")
		defer os.Remove(f.Name())
		_, err := f.WriteString(`
      Kind = "service-defaults"
      Name = "web"
      Protocol = "udp"
      `)

		require.NoError(t, err)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			f.Name(),
		}

		code := c.Run(args)
		require.Empty(t, ui.ErrorWriter.String())
		require.Equal(t, 0, code)

		entry, _, err := client.ConfigEntries().Get("service-defaults", "web", nil)
		require.NoError(t, err)
		svc, ok := entry.(*api.ServiceConfigEntry)
		require.True(t, ok)
		require.Equal(t, api.ServiceDefaults, svc.Kind)
		require.Equal(t, "web", svc.Name)
		require.Equal(t, "udp", svc.Protocol)
	})

	t.Run("Stdin", func(t *testing.T) {
		stdinR, stdinW := io.Pipe()

		ui := cli.NewMockUi()
		c := New(ui)
		c.testStdin = stdinR

		go func() {
			stdinW.Write([]byte(`{
            "Kind": "proxy-defaults",
            "Name": "global",
            "Config": {
               "foo": "bar",
               "bar": 1.0
            }
         }`))
			stdinW.Close()
		}()

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-",
		}

		code := c.Run(args)
		require.Empty(t, ui.ErrorWriter.String())
		require.Equal(t, 0, code)

		entry, _, err := client.ConfigEntries().Get(api.ProxyDefaults, api.ProxyConfigGlobal, nil)
		require.NoError(t, err)
		proxy, ok := entry.(*api.ProxyConfigEntry)
		require.True(t, ok)
		require.Equal(t, api.ProxyDefaults, proxy.Kind)
		require.Equal(t, api.ProxyConfigGlobal, proxy.Name)
		require.Equal(t, map[string]interface{}{"foo": "bar", "bar": 1.0}, proxy.Config)
	})

	t.Run("No config", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)

		code := c.Run([]string{})
		require.NotEqual(t, 0, code)
		require.NotEmpty(t, ui.ErrorWriter.String())
	})
}

// TestParseConfigEntry is the 'api' mirror image of
// agent/structs/config_entry_test.go:TestDecodeConfigEntry
func TestParseConfigEntry(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		camel, camelJSON string
		snake, snakeJSON string
		expect           api.ConfigEntry
		expectJSON       api.ConfigEntry
		expectErr        string
	}{
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
			snakeJSON: `
			{
				"kind": "proxy-defaults",
				"name": "main",
				"cornfig": {
					"foo": 19
				}
			}
			`,
			camelJSON: `
			{
				"Kind": "proxy-defaults",
				"Name": "main",
				"Cornfig": {
					"foo": 19
				}
			}
			`,
			expectErr: `invalid config key "cornfig"`,
		},
		{
			name: "proxy-defaults",
			snake: `
				kind = "proxy-defaults"
				name = "main"
				config {
				  "foo" = 19
				  "bar" = "abc"
				  "moreconfig" {
					"moar" = "config"
				  }
				}
				mesh_gateway {
					mode = "remote"
				}
			`,
			camel: `
				Kind = "proxy-defaults"
				Name = "main"
				Config {
				  "foo" = 19
				  "bar" = "abc"
				  "moreconfig" {
					"moar" = "config"
				  }
				}
				MeshGateway {
					Mode = "remote"
				}
			`,
			snakeJSON: `
			{
				"kind": "proxy-defaults",
				"name": "main",
				"config": {
					"foo": 19,
					"bar": "abc",
					"moreconfig": {
						"moar": "config"
					}
				},
				"mesh_gateway": {
					"mode": "remote"
				}
			}
			`,
			camelJSON: `
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
			expect: &api.ProxyConfigEntry{
				Kind: "proxy-defaults",
				Name: "main",
				Config: map[string]interface{}{
					"foo": 19,
					"bar": "abc",
					"moreconfig": map[string]interface{}{
						"moar": "config",
					},
				},
				MeshGateway: api.MeshGatewayConfig{
					Mode: api.MeshGatewayModeRemote,
				},
			},
			expectJSON: &api.ProxyConfigEntry{
				Kind: "proxy-defaults",
				Name: "main",
				Config: map[string]interface{}{
					"foo": float64(19), // json decoding gives float64 instead of int here
					"bar": "abc",
					"moreconfig": map[string]interface{}{
						"moar": "config",
					},
				},
				MeshGateway: api.MeshGatewayConfig{
					Mode: api.MeshGatewayModeRemote,
				},
			},
		},
		{
			name: "service-defaults",
			snake: `
				kind = "service-defaults"
				name = "main"
				protocol = "http"
				external_sni = "abc-123"
				mesh_gateway {
					mode = "remote"
				}
			`,
			camel: `
				Kind = "service-defaults"
				Name = "main"
				Protocol = "http"
				ExternalSNI = "abc-123"
				MeshGateway {
					Mode = "remote"
				}
			`,
			snakeJSON: `
			{
				"kind": "service-defaults",
				"name": "main",
				"protocol": "http",
				"external_sni": "abc-123",
				"mesh_gateway": {
					"mode": "remote"
				}
			}
			`,
			camelJSON: `
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
			expect: &api.ServiceConfigEntry{
				Kind:        "service-defaults",
				Name:        "main",
				Protocol:    "http",
				ExternalSNI: "abc-123",
				MeshGateway: api.MeshGatewayConfig{
					Mode: api.MeshGatewayModeRemote,
				},
			},
		},
		{
			name: "service-router: kitchen sink",
			snake: `
				kind = "service-router"
				name = "main"
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
						  num_retries            = 12345
						  retry_on_connect_failure = true
						  retry_on_status_codes    = [401, 209]
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
						  NumRetries            = 12345
						  RetryOnConnectFailure = true
						  RetryOnStatusCodes    = [401, 209]
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
			snakeJSON: `
			{
				"kind": "service-router",
				"name": "main",
				"routes": [
					{
						"match": {
							"http": {
								"path_exact": "/foo",
								"header": [
									{
										"name": "debug1",
										"present": true
									},
									{
										"name": "debug2",
										"present": false,
										"invert": true
									},
									{
										"name": "debug3",
										"exact": "1"
									},
									{
										"name": "debug4",
										"prefix": "aaa"
									},
									{
										"name": "debug5",
										"suffix": "bbb"
									},
									{
										"name": "debug6",
										"regex": "a.*z"
									}
								]
							}
						},
						"destination": {
							"service": "carrot",
							"service_subset": "kale",
							"namespace": "leek",
							"prefix_rewrite": "/alternate",
							"request_timeout": "99s",
							"num_retries": 12345,
							"retry_on_connect_failure": true,
							"retry_on_status_codes": [
								401,
								209
							]
						}
					},
					{
						"match": {
							"http": {
								"path_prefix": "/foo",
								"methods": [
									"GET",
									"DELETE"
								],
								"query_param": [
									{
										"name": "hack1",
										"present": true
									},
									{
										"name": "hack2",
										"exact": "1"
									},
									{
										"name": "hack3",
										"regex": "a.*z"
									}
								]
							}
						}
					},
					{
						"match": {
							"http": {
								"path_regex": "/foo"
							}
						}
					}
				]
			}
			`,
			camelJSON: `
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
							"RetryOnStatusCodes": [
								401,
								209
							]
						}
					},
					{
						"Match": {
							"HTTP": {
								"PathPrefix": "/foo",
								"Methods": [
									"GET",
									"DELETE"
								],
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
			expect: &api.ServiceRouterConfigEntry{
				Kind: "service-router",
				Name: "main",
				Routes: []api.ServiceRoute{
					{
						Match: &api.ServiceRouteMatch{
							HTTP: &api.ServiceRouteHTTPMatch{
								PathExact: "/foo",
								Header: []api.ServiceRouteHTTPMatchHeader{
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
						Destination: &api.ServiceRouteDestination{
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
						Match: &api.ServiceRouteMatch{
							HTTP: &api.ServiceRouteHTTPMatch{
								PathPrefix: "/foo",
								Methods:    []string{"GET", "DELETE"},
								QueryParam: []api.ServiceRouteHTTPMatchQueryParam{
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
						Match: &api.ServiceRouteMatch{
							HTTP: &api.ServiceRouteHTTPMatch{
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
				splits = [
				  {
					weight        = 97.1
					service_subset = "v1"
				  },
				  {
					weight        = 2
					service_subset = "v2"
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
				Splits = [
				  {
					Weight        = 97.1
					ServiceSubset = "v1"
				  },
				  {
					Weight        = 2,
					ServiceSubset = "v2"
				  },
				  {
					Weight    = 0.9
					Service   = "other"
					Namespace = "alt"
				  },
				]
			`,
			snakeJSON: `
			{
				"kind": "service-splitter",
				"name": "main",
				"splits": [
					{
						"weight": 97.1,
						"service_subset": "v1"
					},
					{
						"weight": 2,
						"service_subset": "v2"
					},
					{
						"weight": 0.9,
						"service": "other",
						"namespace": "alt"
					}
				]
			}
			`,
			camelJSON: `
			{
				"Kind": "service-splitter",
				"Name": "main",
				"Splits": [
					{
						"Weight": 97.1,
						"ServiceSubset": "v1"
					},
					{
						"Weight": 2,
						"ServiceSubset": "v2"
					},
					{
						"Weight": 0.9,
						"Service": "other",
						"Namespace": "alt"
					}
				]
			}
			`,
			expect: &api.ServiceSplitterConfigEntry{
				Kind: api.ServiceSplitter,
				Name: "main",
				Splits: []api.ServiceSplit{
					{
						Weight:        97.1,
						ServiceSubset: "v1",
					},
					{
						Weight:        2,
						ServiceSubset: "v2",
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
			snakeJSON: `
			{
				"kind": "service-resolver",
				"name": "main",
				"default_subset": "v1",
				"connect_timeout": "15s",
				"subsets": {
					"v1": {
						"filter": "Service.Meta.version == v1"
					},
					"v2": {
						"filter": "Service.Meta.version == v2",
						"only_passing": true
					}
				},
				"failover": {
					"v2": {
						"service": "failcopy",
						"service_subset": "sure",
						"namespace": "neighbor",
						"datacenters": [
							"dc5",
							"dc14"
						]
					},
					"*": {
						"datacenters": [
							"dc7"
						]
					}
				}
			}
			`,
			camelJSON: `
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
						"Datacenters": [
							"dc5",
							"dc14"
						]
					},
					"*": {
						"Datacenters": [
							"dc7"
						]
					}
				}
			}
			`,
			expect: &api.ServiceResolverConfigEntry{
				Kind:           "service-resolver",
				Name:           "main",
				DefaultSubset:  "v1",
				ConnectTimeout: 15 * time.Second,
				Subsets: map[string]api.ServiceResolverSubset{
					"v1": {
						Filter: "Service.Meta.version == v1",
					},
					"v2": {
						Filter:      "Service.Meta.version == v2",
						OnlyPassing: true,
					},
				},
				Failover: map[string]api.ServiceResolverFailover{
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
			snakeJSON: `
			{
				"kind": "service-resolver",
				"name": "main",
				"redirect": {
					"service": "other",
					"service_subset": "backup",
					"namespace": "alt",
					"datacenter": "dc9"
				}
			}
			`,
			camelJSON: `
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
			expect: &api.ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
				Redirect: &api.ServiceResolverRedirect{
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
			snakeJSON: `
			{
				"kind": "service-resolver",
				"name": "main"
			}
			`,
			camelJSON: `
			{
				"Kind": "service-resolver",
				"Name": "main"
			}
			`,
			expect: &api.ServiceResolverConfigEntry{
				Kind: "service-resolver",
				Name: "main",
			},
		},
	} {
		tc := tc

		testbody := func(t *testing.T, body string, expect api.ConfigEntry) {
			t.Helper()
			got, err := parseConfigEntry(body)
			if tc.expectErr != "" {
				require.Nil(t, got)
				require.Error(t, err)
				requireContainsLower(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, expect, got)
			}
		}

		t.Run(tc.name+" (hcl snake case)", func(t *testing.T) {
			testbody(t, tc.snake, tc.expect)
		})
		t.Run(tc.name+" (hcl camel case)", func(t *testing.T) {
			testbody(t, tc.camel, tc.expect)
		})
		if tc.snakeJSON != "" {
			t.Run(tc.name+" (json snake case)", func(t *testing.T) {
				if tc.expectJSON != nil {
					testbody(t, tc.snakeJSON, tc.expectJSON)
				} else {
					testbody(t, tc.snakeJSON, tc.expect)
				}
			})
		}
		if tc.camelJSON != "" {
			t.Run(tc.name+" (json camel case)", func(t *testing.T) {
				if tc.expectJSON != nil {
					testbody(t, tc.camelJSON, tc.expectJSON)
				} else {
					testbody(t, tc.camelJSON, tc.expect)
				}
			})
		}
	}
}

func requireContainsLower(t *testing.T, haystack, needle string) {
	t.Helper()
	require.Contains(t, strings.ToLower(haystack), strings.ToLower(needle))
}
