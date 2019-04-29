package xds

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"
	"testing"
	"text/template"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/stretchr/testify/require"
)

func TestListenersFromSnapshot(t *testing.T) {

	tests := []struct {
		name string
		// Setup is called before the test starts. It is passed the snapshot from
		// TestConfigSnapshot and is allowed to modify it in any way to setup the
		// test input.
		setup              func(snap *proxycfg.ConfigSnapshot)
		overrideGoldenName string
	}{
		{
			name:  "defaults",
			setup: nil, // Default snapshot
		},
		{
			name: "http-public-listener",
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["protocol"] = "http"
			},
		},
		{
			name: "http-upstream",
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["protocol"] = "http"
			},
		},
		{
			name: "custom-public-listener",
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
			name: "custom-upstream",
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
			overrideGoldenName: "custom-upstream", // should be the same
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_listener_json"] =
					customListenerJSON(t, customListenerJSONOptions{
						Name:        "custom-upstream",
						IncludeType: true,
					})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			// Sanity check default with no overrides first
			snap := proxycfg.TestConfigSnapshot(t)

			// We need to replace the TLS certs with deterministic ones to make golden
			// files workable. Note we don't update these otherwise they'd change
			// golder files for every test case and so not be any use!
			snap.Leaf.CertPEM = golden(t, "test-leaf-cert", "")
			snap.Leaf.PrivateKeyPEM = golden(t, "test-leaf-key", "")
			snap.Roots.Roots[0].RootCert = golden(t, "test-root-cert", "")

			if tt.setup != nil {
				tt.setup(snap)
			}

			// Need server just for logger dependency
			s := Server{Logger: log.New(os.Stderr, "", log.LstdFlags)}

			listeners, err := s.listenersFromSnapshot(snap, "my-token")
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
								"cluster": "db",
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
								"cluster": "prepared_query:geo-cache",
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
