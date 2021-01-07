package xds

import (
	"bytes"
	"path/filepath"
	"sort"
	"testing"
	"text/template"
	"time"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/proxycfg/proxycfgtest"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestClustersFromSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []struct {
		name   string
		create func(t *testing.T) *proxycfg.ConfigSnapshot
		// Setup is called before the test starts. It is passed the snapshot from
		// create func and is allowed to modify it in any way to setup the
		// test input.
		setup              func(snap *proxycfg.ConfigSnapshot)
		overrideGoldenName string
	}{
		{
			name:   "defaults",
			create: proxycfgtest.TestConfigSnapshot,
			setup:  nil, // Default snapshot
		},
		{
			name:   "custom-local-app",
			create: proxycfgtest.TestConfigSnapshot,
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
			create:             proxycfgtest.TestConfigSnapshot,
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
			create: proxycfgtest.TestConfigSnapshot,
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
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainDefault,
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
			create:             proxycfgtest.TestConfigSnapshot,
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
			create:             proxycfgtest.TestConfigSnapshot,
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
			create: proxycfgtest.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Config["local_connect_timeout_ms"] = 1234
				snap.Proxy.Upstreams[0].Config["connect_timeout_ms"] = 2345
			},
		},
		{
			name:   "custom-limits-max-connections-only",
			create: proxycfgtest.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				for i := range snap.Proxy.Upstreams {
					// We check if Config is nil because the prepared_query upstream is
					// initialized without a Config map. Use Upstreams[i] syntax to
					// modify the actual ConfigSnapshot instead of copying the Upstream
					// in the range.
					if snap.Proxy.Upstreams[i].Config == nil {
						snap.Proxy.Upstreams[i].Config = map[string]interface{}{}
					}

					snap.Proxy.Upstreams[i].Config["limits"] = map[string]interface{}{
						"max_connections": 500,
					}
				}
			},
		},
		{
			name:   "custom-limits-set-to-zero",
			create: proxycfgtest.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				for i := range snap.Proxy.Upstreams {
					if snap.Proxy.Upstreams[i].Config == nil {
						snap.Proxy.Upstreams[i].Config = map[string]interface{}{}
					}

					snap.Proxy.Upstreams[i].Config["limits"] = map[string]interface{}{
						"max_connections":         0,
						"max_pending_requests":    0,
						"max_concurrent_requests": 0,
					}
				}
			},
		},
		{
			name:   "custom-limits",
			create: proxycfgtest.TestConfigSnapshot,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				for i := range snap.Proxy.Upstreams {
					if snap.Proxy.Upstreams[i].Config == nil {
						snap.Proxy.Upstreams[i].Config = map[string]interface{}{}
					}

					snap.Proxy.Upstreams[i].Config["limits"] = map[string]interface{}{
						"max_connections":         500,
						"max_pending_requests":    600,
						"max_concurrent_requests": 700,
					}
				}
			},
		},
		{
			name:   "connect-proxy-with-chain",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChain,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-external-sni",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainExternalSNI,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-overrides",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithOverrides,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-chain-and-failover",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithFailover,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-remote-gateway",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-local-gateway",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-failover-through-local-gateway-triggered",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-local-gateway",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "connect-proxy-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithDoubleFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "splitter-with-resolver-redirect",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "connect-proxy-lb-in-resolver",
			create: proxycfgtest.TestConfigSnapshotDiscoveryChainWithLB,
			setup:  nil,
		},
		{
			name:   "expose-paths-local-app-paths",
			create: proxycfgtest.TestConfigSnapshotExposeConfig,
		},
		{
			name:   "expose-paths-new-cluster-http2",
			create: proxycfgtest.TestConfigSnapshotExposeConfig,
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
			name:   "expose-paths-grpc-new-cluster-http1",
			create: proxycfgtest.TestConfigSnapshotGRPCExposeHTTP1,
		},
		{
			name:   "mesh-gateway",
			create: proxycfgtest.TestConfigSnapshotMeshGateway,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-using-federation-states",
			create: proxycfgtest.TestConfigSnapshotMeshGatewayUsingFederationStates,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-no-services",
			create: proxycfgtest.TestConfigSnapshotMeshGatewayNoServices,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-service-subsets",
			create: proxycfgtest.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "mesh-gateway-ignore-extra-resolvers",
			create: proxycfgtest.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "bar",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("notfound", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "notfound",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "mesh-gateway-service-timeouts",
			create: proxycfgtest.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind:           structs.ServiceResolver,
						Name:           "bar",
						ConnectTimeout: 10 * time.Second,
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "mesh-gateway-non-hash-lb-injected",
			create: proxycfgtest.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
						LoadBalancer: &structs.LoadBalancer{
							Policy: "least_request",
							LeastRequestConfig: &structs.LeastRequestConfig{
								ChoiceCount: 5,
							},
						},
					},
				}
			},
		},
		{
			name:   "mesh-gateway-hash-lb-ignored",
			create: proxycfgtest.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
						LoadBalancer: &structs.LoadBalancer{
							Policy: "ring_hash",
							RingHashConfig: &structs.RingHashConfig{
								MinimumRingSize: 20,
								MaximumRingSize: 50,
							},
						},
					},
				}
			},
		},
		{
			name:   "ingress-gateway",
			create: proxycfgtest.TestConfigSnapshotIngressGateway,
			setup:  nil,
		},
		{
			name:   "ingress-gateway-no-services",
			create: proxycfgtest.TestConfigSnapshotIngressGatewayNoServices,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain",
			create: proxycfgtest.TestConfigSnapshotIngress,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-external-sni",
			create: proxycfgtest.TestConfigSnapshotIngressExternalSNI,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-overrides",
			create: proxycfgtest.TestConfigSnapshotIngressWithOverrides,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-failover",
			create: proxycfgtest.TestConfigSnapshotIngressWithFailover,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-remote-gateway",
			create: proxycfgtest.TestConfigSnapshotIngressWithFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: proxycfgtest.TestConfigSnapshotIngressWithFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-remote-gateway",
			create: proxycfgtest.TestConfigSnapshotIngressWithDoubleFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: proxycfgtest.TestConfigSnapshotIngressWithDoubleFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-local-gateway",
			create: proxycfgtest.TestConfigSnapshotIngressWithFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-local-gateway-triggered",
			create: proxycfgtest.TestConfigSnapshotIngressWithFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-local-gateway",
			create: proxycfgtest.TestConfigSnapshotIngressWithDoubleFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: proxycfgtest.TestConfigSnapshotIngressWithDoubleFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-splitter-with-resolver-redirect",
			create: proxycfgtest.TestConfigSnapshotIngress_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "ingress-lb-in-resolver",
			create: proxycfgtest.TestConfigSnapshotIngressWithLB,
			setup:  nil,
		},
		{
			name:   "terminating-gateway",
			create: proxycfgtest.TestConfigSnapshotTerminatingGateway,
			setup:  nil,
		},
		{
			name:   "terminating-gateway-no-services",
			create: proxycfgtest.TestConfigSnapshotTerminatingGatewayNoServices,
			setup:  nil,
		},
		{
			name:   "terminating-gateway-service-subsets",
			create: proxycfgtest.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("web", nil): {
						Kind: structs.ServiceResolver,
						Name: "web",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("cache", nil): {
						Kind: structs.ServiceResolver,
						Name: "cache",
						Subsets: map[string]structs.ServiceResolverSubset{
							"prod": {
								Filter: "Service.Meta.Env == prod",
							},
						},
					},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("web", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("cache", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
			},
		},
		{
			name:   "terminating-gateway-hostname-service-subsets",
			create: proxycfgtest.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("api", nil): {
						Kind: structs.ServiceResolver,
						Name: "api",
						Subsets: map[string]structs.ServiceResolverSubset{
							"alt": {
								Filter: "Service.Meta.domain == alt",
							},
						},
					},
					structs.NewServiceName("cache", nil): {
						Kind: structs.ServiceResolver,
						Name: "cache",
						Subsets: map[string]structs.ServiceResolverSubset{
							"prod": {
								Filter: "Service.Meta.Env == prod",
							},
						},
					},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("api", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("cache", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
			},
		},
		{
			name:   "terminating-gateway-ignore-extra-resolvers",
			create: proxycfgtest.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("web", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "web",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("notfound", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "notfound",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("web", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
			},
		},
		{
			name:   "terminating-gateway-lb-config",
			create: proxycfgtest.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("web", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "web",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.Version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.Version == 2",
								OnlyPassing: true,
							},
						},
						LoadBalancer: &structs.LoadBalancer{
							Policy: "ring_hash",
							RingHashConfig: &structs.RingHashConfig{
								MinimumRingSize: 20,
								MaximumRingSize: 50,
							},
						},
					},
				}
				snap.TerminatingGateway.ServiceConfigs[structs.NewServiceName("web", nil)] = &structs.ServiceConfigResponse{
					ProxyConfig: map[string]interface{}{"protocol": "http"},
				}
			},
		},
		{
			name:   "ingress-multiple-listeners-duplicate-service",
			create: proxycfgtest.TestConfigSnapshotIngress_MultipleListenersDuplicateService,
			setup:  nil,
		},
	}

	for _, envoyVersion := range proxysupport.EnvoyVersions {
		sf, err := determineSupportedProxyFeaturesFromString(envoyVersion)
		require.NoError(t, err)
		t.Run("envoy-"+envoyVersion, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					require := require.New(t)

					// Sanity check default with no overrides first
					snap := tt.create(t)

					// We need to replace the TLS certs with deterministic ones to make golden
					// files workable. Note we don't update these otherwise they'd change
					// golder files for every test case and so not be any use!
					setupTLSRootsAndLeaf(t, snap)

					if tt.setup != nil {
						tt.setup(snap)
					}

					// Need server just for logger dependency
					logger := testutil.Logger(t)
					s := Server{
						Logger: logger,
					}

					cInfo := connectionInfo{
						Token:         "my-token",
						ProxyFeatures: sf,
					}
					clusters, err := s.clustersFromSnapshot(cInfo, snap)
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

					require.JSONEq(goldenEnvoy(t, filepath.Join("clusters", gName), envoyVersion, gotJSON), gotJSON)
				})
			}
		})
	}
}

func expectClustersJSONResources(snap *proxycfg.ConfigSnapshot) map[string]string {
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
				"circuitBreakers": {

				},
				"altStatName": "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
				"commonLbConfig": {
					"healthyPanicThreshold": {}
				},
				"connectTimeout": "5s",
				"tlsContext": ` + expectedUpstreamTLSContextJSON(snap, "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul") + `
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
				"circuitBreakers": {

				},
				"connectTimeout": "5s",
				"tlsContext": ` + expectedUpstreamTLSContextJSON(snap, "geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul") + `
			}`,
	}
}

func expectClustersJSONFromResources(snap *proxycfg.ConfigSnapshot, v, n uint64, resourcesJSON map[string]string) string {
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

func expectClustersJSON(snap *proxycfg.ConfigSnapshot, v, n uint64) string {
	return expectClustersJSONFromResources(snap, v, n, expectClustersJSONResources(snap))
}

type customClusterJSONOptions struct {
	Name        string
	IncludeType bool
	TLSContext  string
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

func setupTLSRootsAndLeaf(t *testing.T, snap *proxycfg.ConfigSnapshot) {
	if snap.Leaf() != nil {
		switch snap.Kind {
		case structs.ServiceKindConnectProxy:
			snap.ConnectProxy.Leaf.CertPEM = golden(t, "test-leaf-cert", "", "")
			snap.ConnectProxy.Leaf.PrivateKeyPEM = golden(t, "test-leaf-key", "", "")
		case structs.ServiceKindIngressGateway:
			snap.IngressGateway.Leaf.CertPEM = golden(t, "test-leaf-cert", "", "")
			snap.IngressGateway.Leaf.PrivateKeyPEM = golden(t, "test-leaf-key", "", "")
		}
	}
	if snap.Roots != nil {
		snap.Roots.Roots[0].RootCert = golden(t, "test-root-cert", "", "")
	}
}

func TestEnvoyLBConfig_InjectToCluster(t *testing.T) {
	var tests = []struct {
		name     string
		lb       *structs.LoadBalancer
		expected envoy.Cluster
	}{
		{
			name: "skip empty",
			lb: &structs.LoadBalancer{
				Policy: "",
			},
			expected: envoy.Cluster{},
		},
		{
			name: "round robin",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRoundRobin,
			},
			expected: envoy.Cluster{LbPolicy: envoy.Cluster_ROUND_ROBIN},
		},
		{
			name: "random",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRandom,
			},
			expected: envoy.Cluster{LbPolicy: envoy.Cluster_RANDOM},
		},
		{
			name: "maglev",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
			},
			expected: envoy.Cluster{LbPolicy: envoy.Cluster_MAGLEV},
		},
		{
			name: "ring_hash",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRingHash,
				RingHashConfig: &structs.RingHashConfig{
					MinimumRingSize: 3,
					MaximumRingSize: 7,
				},
			},
			expected: envoy.Cluster{
				LbPolicy: envoy.Cluster_RING_HASH,
				LbConfig: &envoy.Cluster_RingHashLbConfig_{
					RingHashLbConfig: &envoy.Cluster_RingHashLbConfig{
						MinimumRingSize: &wrappers.UInt64Value{Value: 3},
						MaximumRingSize: &wrappers.UInt64Value{Value: 7},
					},
				},
			},
		},
		{
			name: "least_request",
			lb: &structs.LoadBalancer{
				Policy: "least_request",
				LeastRequestConfig: &structs.LeastRequestConfig{
					ChoiceCount: 3,
				},
			},
			expected: envoy.Cluster{
				LbPolicy: envoy.Cluster_LEAST_REQUEST,
				LbConfig: &envoy.Cluster_LeastRequestLbConfig_{
					LeastRequestLbConfig: &envoy.Cluster_LeastRequestLbConfig{
						ChoiceCount: &wrappers.UInt32Value{Value: 3},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var c envoy.Cluster
			err := injectLBToCluster(tc.lb, &c)
			require.NoError(t, err)

			require.Equal(t, tc.expected, c)
		})
	}
}
