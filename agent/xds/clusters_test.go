package xds

import (
	"bytes"
	"log"
	"os"
	"path"
	"sort"
	"testing"
	"text/template"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
)

func TestClustersFromSnapshot(t *testing.T) {

	tests := []struct {
		name   string
		create func(t testinf.T) *proxycfg.ConfigSnapshot
		// Setup is called before the test starts. It is passed the snapshot from
		// create func and is allowed to modify it in any way to setup the
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
			name:   "custom-local-app",
			create: proxycfg.TestConfigSnapshot,
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
			create:             proxycfg.TestConfigSnapshot,
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
			name:   "custom-upstream",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name:        "myservice",
						IncludeType: false,
					})
			},
		},
		{
			name:   "custom-upstream-default-chain",
			create: proxycfg.TestConfigSnapshotDiscoveryChainDefault,
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
			create:             proxycfg.TestConfigSnapshot,
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
			create:             proxycfg.TestConfigSnapshot,
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
			name:   "custom-timeouts",
			create: proxycfg.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["local_connect_timeout_ms"] = 1234
				snap.Proxy.Upstreams[0].Config["connect_timeout_ms"] = 2345
			},
		},
		{
			name:   "connect-proxy-with-chain",
			create: proxycfg.TestConfigSnapshotDiscoveryChain,
			setup:  nil,
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
			name:   "connect-proxy-with-chain-and-failover",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailover,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-remote-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-local-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-local-gateway-triggered",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-local-gateway",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: proxycfg.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "mesh-gateway",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-service-subsets",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[string]*structs.ServiceResolverConfigEntry{
					"bar": &structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": structs.ServiceResolverSubset{
								Filter: "Service.Meta.Version == 1",
							},
							"v2": structs.ServiceResolverSubset{
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
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
			s := Server{Logger: log.New(os.Stderr, "", log.LstdFlags)}

			clusters, err := s.clustersFromSnapshot(snap, "my-token")
			require.NoError(err)
			sort.Slice(clusters, func(i, j int) bool {
				return clusters[i].(*envoy.Cluster).Name < clusters[j].(*envoy.Cluster).Name
			})
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
				"type": "STATIC",
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
				"name": "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"type": "EDS",
				"edsClusterConfig": {
					"edsConfig": {
						"ads": {

						}
					}
				},
				"outlierDetection": {

				},
				"altStatName": "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"commonLbConfig": {
					"healthyPanicThreshold": {}
				},
				"connectTimeout": "5s",
				"tlsContext": ` + expectedUpstreamTLSContextJSON(t, snap, "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul") + `
			}`,
		"prepared_query:geo-cache": `
			{
				"@type": "type.googleapis.com/envoy.api.v2.Cluster",
				"name": "geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
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
				"tlsContext": ` + expectedUpstreamTLSContextJSON(t, snap, "geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul") + `
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
	"connectTimeout": "15s",
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
