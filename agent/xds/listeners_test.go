package xds

import (
	"bytes"
	"fmt"
	"path"
	"sort"
	"testing"
	"text/template"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

func TestListenersFromSnapshot(t *testing.T) {

	tests := []struct {
		name   string
		create func(t testinf.T) *proxycfg.ConfigSnapshot
		// Setup is called before the test starts. It is passed the snapshot from
		// TestConfigSnapshot and is allowed to modify it in any way to setup the
		// test input.
		setup              func(snap *proxycfg.ConfigSnapshot)
		overrideGoldenName string
	}{
		{
			name:   "defaults",
			create: proxycfg.TestConfigSnapshot,
			setup:  nil, // Default snapshot
		},
		{
			name:   "listener-bind-address",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["bind_address"] = "127.0.0.2"
			},
		},
		{
			name:   "listener-bind-port",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["bind_port"] = 8888
			},
		},
		{
			name:   "listener-bind-address-port",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["bind_address"] = "127.0.0.2"
				snap.Proxy.Config["bind_port"] = 8888
			},
		},
		{
			name:   "http-public-listener",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["protocol"] = "http"
			},
		},
		{
			name:   "http-upstream",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["protocol"] = "http"
			},
		},
		{
			name:   "custom-public-listener",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["envoy_public_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-public-listen",
						IncludeType: false,
					})
			},
		},
		{
			name:               "custom-public-listener-typed",
			create:             proxycfg.TestConfigSnapshot,
			overrideGoldenName: "custom-public-listener", // should be the same
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["envoy_public_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-public-listen",
						IncludeType: true,
					})
			},
		},
		{
			name:               "custom-public-listener-ignores-tls",
			create:             proxycfg.TestConfigSnapshot,
			overrideGoldenName: "custom-public-listener", // should be the same
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["envoy_public_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-public-listen",
						IncludeType: true,
						// Attempt to override the TLS context should be ignored
						TLSContext: `{"requireClientCertificate": false}`,
					})
			},
		},
		{
			name:   "custom-upstream",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-upstream",
						IncludeType: false,
					})
			},
		},
		{
			name:               "custom-upstream-typed",
			create:             proxycfg.TestConfigSnapshot,
			overrideGoldenName: "custom-upstream", // should be the same
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-upstream",
						IncludeType: true,
					})
			},
		},
		{
			name:   "splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain",
			create: proxycfg.TestConfigSnapshotDiscoveryChain,
			setup:  nil,
		},
		{
			name: "connect-proxy-with-http-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChainWithEntries(t,
					&structs.ProxyConfigEntry{
						Kind: structs.ProxyDefaults,
						Name: structs.ProxyConfigGlobal,
						Config: map[string]interface{}{
							"protocol": "http",
						},
					},
				)
			},
			setup: nil,
		},
		{
			name: "connect-proxy-with-http2-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChainWithEntries(t,
					&structs.ProxyConfigEntry{
						Kind: structs.ProxyDefaults,
						Name: structs.ProxyConfigGlobal,
						Config: map[string]interface{}{
							"protocol": "http2",
						},
					},
				)
			},
			setup: nil,
		},
		{
			name: "connect-proxy-with-grpc-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChainWithEntries(t,
					&structs.ProxyConfigEntry{
						Kind: structs.ProxyDefaults,
						Name: structs.ProxyConfigGlobal,
						Config: map[string]interface{}{
							"protocol": "grpc",
						},
					},
				)
			},
			setup: nil,
		},
		{
			name:   "connect-proxy-with-chain-external-sni",
			create: proxycfg.TestConfigSnapshotDiscoveryChainExternalSNI,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-overrides",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithOverrides,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-remote-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-local-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "expose-paths-local-app-paths",
			create: proxycfg.TestConfigSnapshotExposeConfig,
		},
		{
			name:   "expose-paths-new-cluster-http2",
			create: proxycfg.TestConfigSnapshotExposeConfig,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Expose.Paths[1] = structs.ExposePath{
					LocalPathPort: 9090,
					Path:          "/grpc.health.v1.Health/Check",
					ListenerPort:  21501,
					Protocol:      "http2",
				}
			},
		},
		{
			name:   "mesh-gateway",
			create: proxycfg.TestConfigSnapshotMeshGateway,
		},
		{
			name:   "mesh-gateway-using-federation-states",
			create: proxycfg.TestConfigSnapshotMeshGatewayUsingFederationStates,
		},
		{
			name:   "mesh-gateway-no-services",
			create: proxycfg.TestConfigSnapshotMeshGatewayNoServices,
		},
		{
			name:   "mesh-gateway-tagged-addresses",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config = map[string]interface{}{
					"envoy_mesh_gateway_no_default_bind":       true,
					"envoy_mesh_gateway_bind_tagged_addresses": true,
				}
			},
		},
		{
			name:   "mesh-gateway-custom-addresses",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config = map[string]interface{}{
					"envoy_mesh_gateway_bind_addresses": map[string]structs.ServiceAddress{
						"foo": structs.ServiceAddress{
							Address: "198.17.2.3",
							Port:    8080,
						},
						"bar": structs.ServiceAddress{
							Address: "2001:db8::ff",
							Port:    9999,
						},
						"baz": structs.ServiceAddress{
							Address: "127.0.0.1",
							Port:    8765,
						},
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			// Sanity check default with no overrides first
			snap := tt.create(t)

			// We need to replace the TLS certs with deterministic ones to make golden
			// files workable. Note we don't update these otherwise they'd change
			// golder files for every test case and so not be any use!
			if snap.ConnectProxy.Leaf != nil {
				snap.ConnectProxy.Leaf.CertPEM = golden(t, "test-leaf-cert", "")
				snap.ConnectProxy.Leaf.PrivateKeyPEM = golden(t, "test-leaf-key", "")
			}
			if snap.Roots != nil {
				snap.Roots.Roots[0].RootCert = golden(t, "test-root-cert", "")
			}

			if tt.setup != nil {
				tt.setup(snap)
			}

			// Need server just for logger dependency
			logger := testutil.Logger(t)
			s := Server{
				Logger: logger,
			}

			listeners, err := s.listenersFromSnapshot(snap, "my-token")
			sort.Slice(listeners, func(i, j int) bool {
				return listeners[i].(*envoy.Listener).Name < listeners[j].(*envoy.Listener).Name
			})

			require.NoError(err)
			r, err := createResponse(ListenerType, "00000001", "00000001", listeners)
			require.NoError(err)

			gotJSON := responseToJSON(t, r)

			gName := tt.name
			if tt.overrideGoldenName != "" {
				gName = tt.overrideGoldenName
			}

			require.JSONEq(golden(t, path.Join("listeners", gName), gotJSON), gotJSON)
		})
	}
}

func expectListenerJSONResources(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64) map[string]string {
	tokenVal := ""
	if token != "" {
		tokenVal = fmt.Sprintf(",\n"+`"value": "%s"`, token)
	}
	return map[string]string{
		"public_listener": `{
				"@type": "type.googleapis.com/envoy.api.v2.Listener",
				"name": "public_listener:0.0.0.0:9999",
				"address": {
					"socketAddress": {
						"address": "0.0.0.0",
						"portValue": 9999
					}
				},
				"filterChains": [
					{
						"tlsContext": ` + expectedPublicTLSContextJSON(t, snap) + `,
						"filters": [
							{
								"name": "envoy.ext_authz",
								"config": {
										"grpc_service": {
												"envoy_grpc": {
													"cluster_name": "local_agent"
												},
												"initial_metadata": [
													{
														"key": "x-consul-token"
														` + tokenVal + `
													}
												]
											},
										"stat_prefix": "connect_authz"
									}
							},
							{
								"name": "envoy.tcp_proxy",
								"config": {
									"cluster": "local_app",
									"stat_prefix": "public_listener_tcp"
								}
							}
						]
					}
				]
			}`,
		"db": `{
			"@type": "type.googleapis.com/envoy.api.v2.Listener",
			"name": "db:127.0.0.1:9191",
			"address": {
				"socketAddress": {
					"address": "127.0.0.1",
					"portValue": 9191
				}
			},
			"filterChains": [
				{
					"filters": [
						{
							"name": "envoy.tcp_proxy",
							"config": {
								"cluster": "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
								"stat_prefix": "upstream_db_tcp"
							}
						}
					]
				}
			]
		}`,
		"prepared_query:geo-cache": `{
			"@type": "type.googleapis.com/envoy.api.v2.Listener",
			"name": "prepared_query:geo-cache:127.10.10.10:8181",
			"address": {
				"socketAddress": {
					"address": "127.10.10.10",
					"portValue": 8181
				}
			},
			"filterChains": [
				{
					"filters": [
						{
							"name": "envoy.tcp_proxy",
							"config": {
								"cluster": "geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
								"stat_prefix": "upstream_prepared_query_geo-cache_tcp"
							}
						}
					]
				}
			]
		}`,
	}
}

func expectListenerJSONFromResources(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64, resourcesJSON map[string]string) string {
	resJSON := ""
	// Sort resources into specific order because that matters in JSONEq
	// comparison later.
	keyOrder := []string{"public_listener"}
	for _, u := range snap.Proxy.Upstreams {
		keyOrder = append(keyOrder, u.Identifier())
	}
	for _, k := range keyOrder {
		j, ok := resourcesJSON[k]
		if !ok {
			continue
		}
		if resJSON != "" {
			resJSON += ",\n"
		}
		resJSON += j
	}
	return `{
		"versionInfo": "` + hexString(v) + `",
		"resources": [` + resJSON + `],
		"typeUrl": "type.googleapis.com/envoy.api.v2.Listener",
		"nonce": "` + hexString(n) + `"
		}`
}

func expectListenerJSON(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64) string {
	return expectListenerJSONFromResources(t, snap, token, v, n,
		expectListenerJSONResources(t, snap, token, v, n))
}

type customListenerJSONOptions struct {
	Name          string
	IncludeType   bool
	OverrideAuthz bool
	TLSContext    string
}

const customListenerJSONTpl = `{
	{{ if .IncludeType -}}
	"@type": "type.googleapis.com/envoy.api.v2.Listener",
	{{- end }}
	"name": "{{ .Name }}",
	"address": {
		"socketAddress": {
			"address": "11.11.11.11",
			"portValue": 11111
		}
	},
	"filterChains": [
		{
			{{ if .TLSContext -}}
			"tlsContext": {{ .TLSContext }},
			{{- end }}
			"filters": [
				{{ if .OverrideAuthz -}}
				{
					"name": "envoy.ext_authz",
					"config": {
							"grpc_service": {
										"envoy_grpc": {
													"cluster_name": "local_agent"
												},
										"initial_metadata": [
													{
																"key": "x-consul-token",
																"value": "my-token"
															}
												]
									},
							"stat_prefix": "connect_authz"
						}
				},
				{{- end }}
				{
					"name": "envoy.tcp_proxy",
					"config": {
							"cluster": "random-cluster",
							"stat_prefix": "foo-stats"
						}
				}
			]
		}
	]
}`

var customListenerJSONTemplate = template.Must(template.New("").Parse(customListenerJSONTpl))

func customListenerJSON(t *testing.T, opts customListenerJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customListenerJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}
