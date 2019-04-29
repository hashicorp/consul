package xds

import (
	"bytes"
	"log"
	"os"
	"path"
	"testing"
	"text/template"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/stretchr/testify/require"
)

func TestClustersFromSnapshot(t *testing.T) {

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
			name: "custom-local-app",
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["envoy_local_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name:        "mylocal",
						IncludeType: false,
					})
			},
		},
		{
			name:               "custom-local-app-typed",
			overrideGoldenName: "custom-local-app",
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["envoy_local_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name:        "mylocal",
						IncludeType: true,
					})
			},
		},
		{
			name: "custom-upstream",
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name:        "myservice",
						IncludeType: false,
					})
			},
		},
		{
			name:               "custom-upstream-typed",
			overrideGoldenName: "custom-upstream",
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name:        "myservice",
						IncludeType: true,
					})
			},
		},
		{
			name:               "custom-upstream-ignores-tls",
			overrideGoldenName: "custom-upstream", // should be the same
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name:        "myservice",
						IncludeType: true,
						// Attempt to override the TLS context should be ignored
						TLSContext: `{"commonTlsContext": {}}`,
					})
			},
		},
		{
			name: "custom-timeouts",
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["local_connect_timeout_ms"] = 1234
				snap.Proxy.Upstreams[0].Config["connect_timeout_ms"] = 2345
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

			clusters, err := s.clustersFromSnapshot(snap, "my-token")
			require.NoError(err)
			r, err := createResponse(ClusterType, "00000001", "00000001", clusters)
			require.NoError(err)

			gotJSON := responseToJSON(t, r)

			gName := tt.name
			if tt.overrideGoldenName != "" {
				gName = tt.overrideGoldenName
			}

			require.JSONEq(golden(t, path.Join("clusters", gName), gotJSON), gotJSON)
		})
	}
}

func expectClustersJSONResources(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64) map[string]string {
	return map[string]string{
		"local_app": `
			{
				"@type": "type.googleapis.com/envoy.api.v2.Cluster",
				"name": "local_app",
				"connectTimeout": "5s",
				"loadAssignment": {
					"clusterName": "local_app",
					"endpoints": [
						{
							"lbEndpoints": [
								{
									"endpoint": {
										"address": {
											"socketAddress": {
												"address": "127.0.0.1",
												"portValue": 8080
											}
										}
									}
								}
							]
						}
					]
				}
			}`,
		"db": `
			{
				"@type": "type.googleapis.com/envoy.api.v2.Cluster",
				"name": "db",
				"type": "EDS",
				"edsClusterConfig": {
					"edsConfig": {
						"ads": {

						}
					}
				},
				"outlierDetection": {

				},
				"connectTimeout": "1s",
				"tlsContext": ` + expectedUpstreamTLSContextJSON(t, snap) + `
			}`,
		"prepared_query:geo-cache": `
			{
				"@type": "type.googleapis.com/envoy.api.v2.Cluster",
				"name": "prepared_query:geo-cache",
				"type": "EDS",
				"edsClusterConfig": {
					"edsConfig": {
						"ads": {

						}
					}
				},
				"outlierDetection": {

				},
				"connectTimeout": "5s",
				"tlsContext": ` + expectedUpstreamTLSContextJSON(t, snap) + `
			}`,
	}
}

func expectClustersJSONFromResources(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64, resourcesJSON map[string]string) string {
	resJSON := ""

	// Sort resources into specific order because that matters in JSONEq
	// comparison later.
	keyOrder := []string{"local_app"}
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
		"typeUrl": "type.googleapis.com/envoy.api.v2.Cluster",
		"nonce": "` + hexString(n) + `"
		}`
}

func expectClustersJSON(t *testing.T, snap *proxycfg.ConfigSnapshot, token string, v, n uint64) string {
	return expectClustersJSONFromResources(t, snap, token, v, n,
		expectClustersJSONResources(t, snap, token, v, n))
}

type customClusterJSONOptions struct {
	Name        string
	IncludeType bool
	TLSContext  string
}

var customEDSClusterJSONTpl = `{
	{{ if .IncludeType -}}
	"@type": "type.googleapis.com/envoy.api.v2.Cluster",
	{{- end }}
	{{ if .TLSContext -}}
	"tlsContext": {{ .TLSContext }},
	{{- end }}
	"name": "{{ .Name }}",
	"type": "EDS",
	"edsClusterConfig": {
		"edsConfig": {
			"ads": {

			}
		}
	},
	"connectTimeout": "5s"
}`

var customEDSClusterJSONTemplate = template.Must(template.New("").Parse(customEDSClusterJSONTpl))

func customEDSClusterJSON(t *testing.T, opts customClusterJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customEDSClusterJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}

var customAppClusterJSONTpl = `{
	{{ if .IncludeType -}}
	"@type": "type.googleapis.com/envoy.api.v2.Cluster",
	{{- end }}
	{{ if .TLSContext -}}
	"tlsContext": {{ .TLSContext }},
	{{- end }}
	"name": "{{ .Name }}",
	"connectTimeout": "5s",
	"hosts": [
		{
			"socketAddress": {
				"address": "127.0.0.1",
				"portValue": 8080
			}
		}
	]
}`

var customAppClusterJSONTemplate = template.Must(template.New("").Parse(customAppClusterJSONTpl))

func customAppClusterJSON(t *testing.T, opts customClusterJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customAppClusterJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}
