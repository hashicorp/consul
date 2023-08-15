// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"bytes"
	"path/filepath"
	"sort"
	"testing"
	"text/template"
	"time"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/testcommon"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

type clusterTestCase struct {
	name               string
	create             func(t testinf.T) *proxycfg.ConfigSnapshot
	overrideGoldenName string
}

func uint32ptr(i uint32) *uint32 {
	return &i
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func makeClusterDiscoChainTests(enterprise bool) []clusterTestCase {
	return []clusterTestCase{
		{
			name: "custom-upstream-default-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", enterprise, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
						customAppClusterJSON(t, customClusterJSONOptions{
							Name: "myservice",
						})
				}, nil)
			},
		},
		{
			name: "connect-proxy-with-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-chain-http2",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["protocol"] = "http2"
				}, nil)
			},
		},
		{
			name: "connect-proxy-with-chain-external-sni",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "external-sni", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-chain-and-overrides",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple-with-overrides", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-chain-and-failover",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway-triggered", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway-triggered", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway-triggered", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway-triggered", enterprise, nil, nil)
			},
		},
		{
			name: "splitter-with-resolver-redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "splitter-with-resolver-redirect-multidc", enterprise, nil, nil)
			},
		},
		{
			name: "connect-proxy-lb-in-resolver",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "lb-resolver", enterprise, nil, nil)
			},
		},
	}
}

func TestClustersFromSnapshot(t *testing.T) {
	// TODO: we should move all of these to TestAllResourcesFromSnapshot
	// eventually to test all of the xDS types at once with the same input,
	// just as it would be triggered by our xDS server.
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []clusterTestCase{
		{
			name: "connect-proxy-with-tls-outgoing-min-version-auto",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Outgoing: &structs.MeshDirectionalTLSConfig{
										TLSMinVersion: types.TLSVersionAuto,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "connect-proxy-with-tls-outgoing-min-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Outgoing: &structs.MeshDirectionalTLSConfig{
										TLSMinVersion: types.TLSv1_3,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "connect-proxy-with-tls-outgoing-max-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Outgoing: &structs.MeshDirectionalTLSConfig{
										TLSMaxVersion: types.TLSv1_2,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "connect-proxy-with-tls-outgoing-cipher-suites",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Outgoing: &structs.MeshDirectionalTLSConfig{
										CipherSuites: []types.TLSCipherSuite{
											types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
											types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
										},
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "connect-proxy-with-jwt-config-entry-with-local",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "jwt-provider",
						Result: &structs.IndexedConfigEntries{
							Kind: "jwt-provider",
							Entries: []structs.ConfigEntry{
								&structs.JWTProviderConfigEntry{
									Name: "okta",
									JSONWebKeySet: &structs.JSONWebKeySet{
										Local: &structs.LocalJWKS{
											JWKS: "xxx",
										},
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "connect-proxy-with-jwt-config-entry-with-remote-jwks",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "jwt-provider",
						Result: &structs.IndexedConfigEntries{
							Kind: "jwt-provider",
							Entries: []structs.ConfigEntry{
								&structs.JWTProviderConfigEntry{
									Name: "okta",
									JSONWebKeySet: &structs.JSONWebKeySet{
										Remote: &structs.RemoteJWKS{
											RequestTimeoutMs:    1000,
											FetchAsynchronously: true,
											URI:                 "https://test.test.com",
											JWKSCluster: &structs.JWKSCluster{
												DiscoveryType:  structs.DiscoveryTypeStatic,
												ConnectTimeout: time.Duration(5) * time.Second,
												TLSCertificates: &structs.JWKSTLSCertificate{
													TrustedCA: &structs.JWKSTLSCertTrustedCA{
														Filename: "mycert.crt",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "custom-local-app",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["envoy_local_cluster_json"] =
						customAppClusterJSON(t, customClusterJSONOptions{
							Name: "mylocal",
						})
				}, nil)
			},
		},
		{
			name: "custom-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
						customAppClusterJSON(t, customClusterJSONOptions{
							Name: "myservice",
						})
				}, nil)
			},
		},
		{
			name:               "custom-upstream-ignores-tls",
			overrideGoldenName: "custom-upstream", // should be the same
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
						customAppClusterJSON(t, customClusterJSONOptions{
							Name: "myservice",
							// Attempt to override the TLS context should be ignored
							TLSContext: `"allowRenegotiation": false`,
						})
				}, nil)
			},
		},
		{
			name: "custom-upstream-with-prepared-query",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					for i := range ns.Proxy.Upstreams {

						switch ns.Proxy.Upstreams[i].DestinationName {
						case "db":
							if ns.Proxy.Upstreams[i].Config == nil {
								ns.Proxy.Upstreams[i].Config = map[string]interface{}{}
							}

							uid := proxycfg.NewUpstreamID(&ns.Proxy.Upstreams[i])

							// Triggers an override with the presence of the escape hatch listener
							ns.Proxy.Upstreams[i].DestinationType = structs.UpstreamDestTypePreparedQuery

							ns.Proxy.Upstreams[i].Config["envoy_cluster_json"] =
								customClusterJSON(t, customClusterJSONOptions{
									Name: uid.EnvoyID() + ":custom-upstream",
								})

						// Also test that http2 options are triggered.
						// A separate upstream without an override is required to test
						case "geo-cache":
							if ns.Proxy.Upstreams[i].Config == nil {
								ns.Proxy.Upstreams[i].Config = map[string]interface{}{}
							}
							ns.Proxy.Upstreams[i].Config["protocol"] = "http2"
						default:
							continue
						}
					}
				}, nil)
			},
		},
		{
			name: "custom-timeouts",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["local_connect_timeout_ms"] = 1234
					ns.Proxy.Upstreams[0].Config["connect_timeout_ms"] = 2345
				}, nil)
			},
		},
		{
			name: "custom-passive-healthcheck",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["passive_health_check"] = map[string]interface{}{
						"enforcing_consecutive_5xx": float64(80),
						"max_failures":              float64(5),
						"interval":                  float64(10 * time.Second),
						"max_ejection_percent":      float64(100),
						"base_ejection_time":        float64(10 * time.Second),
					}
				}, nil)
			},
		},
		{
			name: "custom-passive-healthcheck-zero-consecutive_5xx",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["passive_health_check"] = map[string]interface{}{
						"enforcing_consecutive_5xx": float64(0),
						"max_failures":              float64(5),
						"interval":                  float64(10 * time.Second),
						"max_ejection_percent":      float64(100),
						"base_ejection_time":        float64(10 * time.Second),
					}
				}, nil)
			},
		},
		{
			name: "custom-max-inbound-connections",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["max_inbound_connections"] = 3456
				}, nil)
			},
		},
		{
			name: "custom-limits-max-connections-only",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					for i := range ns.Proxy.Upstreams {
						// We check if Config is nil because the prepared_query upstream is
						// initialized without a Config map. Use Upstreams[i] syntax to
						// modify the actual ConfigSnapshot instead of copying the Upstream
						// in the range.
						if ns.Proxy.Upstreams[i].Config == nil {
							ns.Proxy.Upstreams[i].Config = map[string]interface{}{}
						}

						ns.Proxy.Upstreams[i].Config["limits"] = map[string]interface{}{
							"max_connections": 500,
						}
					}
				}, nil)
			},
		},
		{
			name: "custom-limits-set-to-zero",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					for i := range ns.Proxy.Upstreams {
						if ns.Proxy.Upstreams[i].Config == nil {
							ns.Proxy.Upstreams[i].Config = map[string]interface{}{}
						}

						ns.Proxy.Upstreams[i].Config["limits"] = map[string]interface{}{
							"max_connections":         0,
							"max_pending_requests":    0,
							"max_concurrent_requests": 0,
						}
					}
				}, nil)
			},
		},
		{
			name: "custom-limits",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					for i := range ns.Proxy.Upstreams {
						if ns.Proxy.Upstreams[i].Config == nil {
							ns.Proxy.Upstreams[i].Config = map[string]interface{}{}
						}

						ns.Proxy.Upstreams[i].Config["limits"] = map[string]interface{}{
							"max_connections":         500,
							"max_pending_requests":    600,
							"max_concurrent_requests": 700,
						}
					}
				}, nil)
			},
		},
		{
			name: "expose-paths-local-app-paths",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotExposeConfig(t, nil)
			},
		},
		{
			name: "downstream-service-with-unix-sockets",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Address = ""
					ns.Port = 0
					ns.Proxy.LocalServiceAddress = ""
					ns.Proxy.LocalServicePort = 0
					ns.Proxy.LocalServiceSocketPath = "/tmp/downstream_proxy.sock"
				}, nil)
			},
		},
		{
			name: "expose-paths-new-cluster-http2",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotExposeConfig(t, func(ns *structs.NodeService) {
					ns.Proxy.Expose.Paths[1] = structs.ExposePath{
						LocalPathPort: 9090,
						Path:          "/grpc.health.v1.Health/Check",
						ListenerPort:  21501,
						Protocol:      "http2",
					}
				})
			},
		},
		{
			name:   "expose-checks",
			create: proxycfg.TestConfigSnapshotExposeChecks,
		},
		{
			name:   "expose-paths-grpc-new-cluster-http1",
			create: proxycfg.TestConfigSnapshotGRPCExposeHTTP1,
		},
		{
			name: "mesh-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "default", nil, nil)
			},
		},
		{
			name: "mesh-gateway-using-federation-states",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "federation-states", nil, nil)
			},
		},
		{
			name: "mesh-gateway-using-federation-control-plane",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "mesh-gateway-federation", nil, nil)
			},
		},
		{
			name: "mesh-gateway-no-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "no-services", nil, nil)
			},
		},
		{
			name: "mesh-gateway-service-subsets",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "service-subsets", nil, nil)
			},
		},
		{
			name: "mesh-gateway-ignore-extra-resolvers",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "ignore-extra-resolvers", nil, nil)
			},
		},
		{
			name: "mesh-gateway-service-timeouts",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "service-timeouts", nil, nil)
			},
		},
		{
			name: "mesh-gateway-non-hash-lb-injected",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "non-hash-lb-injected", nil, nil)
			},
		},
		{
			name: "mesh-gateway-hash-lb-ignored",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "hash-lb-ignored", nil, nil)
			},
		},
		{
			name: "mesh-gateway-tcp-keepalives",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "default", func(ns *structs.NodeService) {
					ns.Proxy.Config["envoy_gateway_remote_tcp_enable_keepalive"] = true
					ns.Proxy.Config["envoy_gateway_remote_tcp_keepalive_time"] = 120
					ns.Proxy.Config["envoy_gateway_remote_tcp_keepalive_interval"] = 60
					ns.Proxy.Config["envoy_gateway_remote_tcp_keepalive_probes"] = 7
				}, nil)
			},
		},
		{
			name: "ingress-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"default", nil, nil, nil)
			},
		},
		{
			name: "ingress-gateway-nil-config-entry",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway_NilConfigEntry(t)
			},
		},
		{
			name: "ingress-gateway-with-tls-outgoing-min-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Outgoing: &structs.MeshDirectionalTLSConfig{
										TLSMinVersion: types.TLSv1_3,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "ingress-gateway-with-tls-outgoing-max-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Outgoing: &structs.MeshDirectionalTLSConfig{
										TLSMaxVersion: types.TLSv1_2,
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "ingress-gateway-with-tls-outgoing-cipher-suites",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Outgoing: &structs.MeshDirectionalTLSConfig{
										CipherSuites: []types.TLSCipherSuite{
											types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
											types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
										},
									},
								},
							},
						},
					},
				})
			},
		},
		{
			name: "ingress-gateway-no-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, false, "tcp",
					"default", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"simple", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-service-max-connections",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"simple", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.Listeners[0].Services[0].MaxConnections = 4096
					}, nil)
			},
		},
		{
			name: "ingress-with-defaults-service-max-connections",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"simple", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.Defaults = &structs.IngressServiceConfig{
							MaxConnections:        2048,
							MaxPendingRequests:    512,
							MaxConcurrentRequests: 4096,
						}
					}, nil)
			},
		},
		{
			name: "ingress-with-overwrite-defaults-service-max-connections",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"simple", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.Defaults = &structs.IngressServiceConfig{
							MaxConnections:     2048,
							MaxPendingRequests: 512,
						}
						entry.Listeners[0].Services[0].MaxConnections = 4096
						entry.Listeners[0].Services[0].MaxPendingRequests = 2048
					}, nil)
			},
		},
		{
			name: "ingress-with-service-passive-health-check",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"simple", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.Listeners[0].Services[0].MaxConnections = 4096
						entry.Listeners[0].Services[0].PassiveHealthCheck = &structs.PassiveHealthCheck{
							Interval:           5000000000,
							MaxFailures:        10,
							MaxEjectionPercent: uint32ptr(90),
						}
					}, nil)
			},
		},
		{
			name: "ingress-with-defaults-passive-health-check",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"simple", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						enforcingConsecutive5xx := uint32(80)
						entry.Defaults = &structs.IngressServiceConfig{
							MaxConnections:        2048,
							MaxPendingRequests:    512,
							MaxConcurrentRequests: 4096,
							PassiveHealthCheck: &structs.PassiveHealthCheck{
								Interval:                5000000000,
								MaxFailures:             10,
								EnforcingConsecutive5xx: &enforcingConsecutive5xx,
								MaxEjectionPercent:      uint32ptr(90),
							},
						}
					}, nil)
			},
		},
		{
			name: "ingress-with-overwrite-defaults-passive-health-check",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"simple", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						defaultEnforcingConsecutive5xx := uint32(80)
						entry.Defaults = &structs.IngressServiceConfig{
							MaxConnections:     2048,
							MaxPendingRequests: 512,
							PassiveHealthCheck: &structs.PassiveHealthCheck{
								Interval:                5000000000,
								EnforcingConsecutive5xx: &defaultEnforcingConsecutive5xx,
								MaxEjectionPercent:      uint32ptr(80),
							},
						}
						enforcingConsecutive5xx := uint32(50)
						entry.Listeners[0].Services[0].MaxConnections = 4096
						entry.Listeners[0].Services[0].MaxPendingRequests = 2048
						entry.Listeners[0].Services[0].PassiveHealthCheck = &structs.PassiveHealthCheck{
							Interval:                8000000000,
							EnforcingConsecutive5xx: &enforcingConsecutive5xx,
							MaxEjectionPercent:      uint32ptr(90),
							BaseEjectionTime:        durationPtr(12 * time.Second),
						}
					}, nil)
			},
		},
		{
			name: "ingress-with-chain-external-sni",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"external-sni", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-chain-and-failover",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-chain-and-failover-to-cluster-peer",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-to-cluster-peer", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-remote-gateway", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-remote-gateway-triggered", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-double-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-double-remote-gateway", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-double-remote-gateway-triggered", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-local-gateway", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-local-gateway-triggered", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-double-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-double-local-gateway", nil, nil, nil)
			},
		},
		{
			name: "ingress-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-double-local-gateway-triggered", nil, nil, nil)
			},
		},
		{
			name: "ingress-splitter-with-resolver-redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"splitter-with-resolver-redirect-multidc", nil, nil, nil)
			},
		},
		{
			name: "ingress-lb-in-resolver",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"lb-resolver", nil, nil, nil)
			},
		},
		{
			name: "terminating-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
			},
		},
		{
			name: "terminating-gateway-no-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, false, nil, nil)
			},
		},
		{
			name:   "terminating-gateway-service-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayServiceSubsetsWebAndCache,
		},
		{
			name:   "terminating-gateway-hostname-service-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayHostnameSubsets,
		},
		{
			name:   "terminating-gateway-sni",
			create: proxycfg.TestConfigSnapshotTerminatingGatewaySNI,
		},
		{
			name:   "terminating-gateway-http2-upstream",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayHTTP2,
		},
		{
			name:   "terminating-gateway-http2-upstream-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGatewaySubsetsHTTP2,
		},
		{
			name:   "terminating-gateway-ignore-extra-resolvers",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayIgnoreExtraResolvers,
		},
		{
			name:   "terminating-gateway-lb-config",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayLBConfigNoHashPolicies,
		},
		{
			name: "terminating-gateway-tcp-keepalives",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, func(ns *structs.NodeService) {
					if ns.Proxy.Config == nil {
						ns.Proxy.Config = map[string]interface{}{}
					}
					ns.Proxy.Config["envoy_gateway_remote_tcp_enable_keepalive"] = true
					ns.Proxy.Config["envoy_gateway_remote_tcp_keepalive_time"] = 133
					ns.Proxy.Config["envoy_gateway_remote_tcp_keepalive_interval"] = 27
					ns.Proxy.Config["envoy_gateway_remote_tcp_keepalive_probes"] = 5
				}, nil)
			},
		},
		{
			name:   "ingress-multiple-listeners-duplicate-service",
			create: proxycfg.TestConfigSnapshotIngress_MultipleListenersDuplicateService,
		},
		{
			name:   "transparent-proxy-catalog-destinations-only",
			create: proxycfg.TestConfigSnapshotTransparentProxyCatalogDestinationsOnly,
		},
		{
			name:   "transparent-proxy-dial-instances-directly",
			create: proxycfg.TestConfigSnapshotTransparentProxyDialDirectly,
		},
	}

	tests = append(tests, makeClusterDiscoChainTests(false)...)

	latestEnvoyVersion := xdscommon.EnvoyVersions[0]
	for _, envoyVersion := range xdscommon.EnvoyVersions {
		t.Run("envoy-"+envoyVersion, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Sanity check default with no overrides first
					snap := tt.create(t)

					// We need to replace the TLS certs with deterministic ones to make golden
					// files workable. Note we don't update these otherwise they'd change
					// golder files for every test case and so not be any use!
					testcommon.SetupTLSRootsAndLeaf(t, snap)

					// Need server just for logger dependency
					g := NewResourceGenerator(testutil.Logger(t), nil, false)

					clusters, err := g.clustersFromSnapshot(snap)
					require.NoError(t, err)

					sort.Slice(clusters, func(i, j int) bool {
						return clusters[i].(*envoy_cluster_v3.Cluster).Name < clusters[j].(*envoy_cluster_v3.Cluster).Name
					})

					r, err := createResponse(xdscommon.ClusterType, "00000001", "00000001", clusters)
					require.NoError(t, err)

					t.Run("current", func(t *testing.T) {
						gotJSON := protoToJSON(t, r)

						gName := tt.name
						if tt.overrideGoldenName != "" {
							gName = tt.overrideGoldenName
						}

						require.JSONEq(t, goldenEnvoy(t, filepath.Join("clusters", gName), envoyVersion, latestEnvoyVersion, gotJSON), gotJSON)
					})
				})
			}
		})
	}
}

type customClusterJSONOptions struct {
	Name       string
	TLSContext string
}

var customAppClusterJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.cluster.v3.Cluster",
	{{ if .TLSContext -}}
	"transport_socket": {
		"name": "tls",
		"typed_config": {
			"@type": "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext",
			{{ .TLSContext }}
		}
	},
	{{- end }}
	"name": "{{ .Name }}",
	"connectTimeout": "15s",
	"loadAssignment": {
		"clusterName": "{{ .Name }}",
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
}`

var customAppClusterJSONTemplate = template.Must(template.New("").Parse(customAppClusterJSONTpl))

func customAppClusterJSON(t testinf.T, opts customClusterJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customAppClusterJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}

var customClusterJSONTpl = `{
	"@type": "type.googleapis.com/envoy.config.cluster.v3.Cluster",
	"name": "{{ .Name }}",
	"connectTimeout": "15s",
	"loadAssignment": {
		"clusterName": "{{ .Name }}",
		"endpoints": [
			{
				"lbEndpoints": [
					{
						"endpoint": {
							"address": {
								"socketAddress": {
									"address": "1.2.3.4",
									"portValue": 8443
								}
							}
						}
					}
				]
			}
		]
	}
}`

var customClusterJSONTemplate = template.Must(template.New("").Parse(customClusterJSONTpl))

func customClusterJSON(t testinf.T, opts customClusterJSONOptions) string {
	t.Helper()
	var buf bytes.Buffer
	err := customClusterJSONTemplate.Execute(&buf, opts)
	require.NoError(t, err)
	return buf.String()
}

func TestEnvoyLBConfig_InjectToCluster(t *testing.T) {
	var tests = []struct {
		name     string
		lb       *structs.LoadBalancer
		expected *envoy_cluster_v3.Cluster
	}{
		{
			name: "skip empty",
			lb: &structs.LoadBalancer{
				Policy: "",
			},
			expected: &envoy_cluster_v3.Cluster{},
		},
		{
			name: "round robin",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRoundRobin,
			},
			expected: &envoy_cluster_v3.Cluster{LbPolicy: envoy_cluster_v3.Cluster_ROUND_ROBIN},
		},
		{
			name: "random",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyRandom,
			},
			expected: &envoy_cluster_v3.Cluster{LbPolicy: envoy_cluster_v3.Cluster_RANDOM},
		},
		{
			name: "maglev",
			lb: &structs.LoadBalancer{
				Policy: structs.LBPolicyMaglev,
			},
			expected: &envoy_cluster_v3.Cluster{LbPolicy: envoy_cluster_v3.Cluster_MAGLEV},
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
			expected: &envoy_cluster_v3.Cluster{
				LbPolicy: envoy_cluster_v3.Cluster_RING_HASH,
				LbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig_{
					RingHashLbConfig: &envoy_cluster_v3.Cluster_RingHashLbConfig{
						MinimumRingSize: &wrapperspb.UInt64Value{Value: 3},
						MaximumRingSize: &wrapperspb.UInt64Value{Value: 7},
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
			expected: &envoy_cluster_v3.Cluster{
				LbPolicy: envoy_cluster_v3.Cluster_LEAST_REQUEST,
				LbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig_{
					LeastRequestLbConfig: &envoy_cluster_v3.Cluster_LeastRequestLbConfig{
						ChoiceCount: &wrapperspb.UInt32Value{Value: 3},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var c envoy_cluster_v3.Cluster
			err := injectLBToCluster(tc.lb, &c)
			require.NoError(t, err)

			require.Equal(t, tc.expected, &c)
		})
	}
}

func TestMakeJWTProviderCluster(t *testing.T) {
	// All tests here depend on golden files located under: agent/xds/testdata/jwt_authn_cluster/*
	tests := map[string]struct {
		provider      *structs.JWTProviderConfigEntry
		expectedError string
	}{
		"remote-jwks-not-configured": {
			provider: &structs.JWTProviderConfigEntry{
				Kind:          "jwt-provider",
				Name:          "okta",
				JSONWebKeySet: &structs.JSONWebKeySet{},
			},
			expectedError: "cannot create JWKS cluster for non remote JWKS. Provider Name: okta",
		},
		"local-jwks-configured": {
			provider: &structs.JWTProviderConfigEntry{
				Kind: "jwt-provider",
				Name: "okta",
				JSONWebKeySet: &structs.JSONWebKeySet{
					Local: &structs.LocalJWKS{
						Filename: "filename",
					},
				},
			},
			expectedError: "cannot create JWKS cluster for non remote JWKS. Provider Name: okta",
		},
		"https-provider-with-hostname-no-port": {
			provider: makeTestProviderWithJWKS("https://example-okta.com/.well-known/jwks.json"),
		},
		"http-provider-with-hostname-no-port": {
			provider: makeTestProviderWithJWKS("http://example-okta.com/.well-known/jwks.json"),
		},
		"https-provider-with-hostname-and-port": {
			provider: makeTestProviderWithJWKS("https://example-okta.com:90/.well-known/jwks.json"),
		},
		"http-provider-with-hostname-and-port": {
			provider: makeTestProviderWithJWKS("http://example-okta.com:90/.well-known/jwks.json"),
		},
		"https-provider-with-ip-no-port": {
			provider: makeTestProviderWithJWKS("https://127.0.0.1"),
		},
		"http-provider-with-ip-no-port": {
			provider: makeTestProviderWithJWKS("http://127.0.0.1"),
		},
		"https-provider-with-ip-and-port": {
			provider: makeTestProviderWithJWKS("https://127.0.0.1:9091"),
		},
		"http-provider-with-ip-and-port": {
			provider: makeTestProviderWithJWKS("http://127.0.0.1:9091"),
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			cluster, err := makeJWTProviderCluster(tt.provider)
			if tt.expectedError != "" {
				require.Error(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
				gotJSON := protoToJSON(t, cluster)
				require.JSONEq(t, goldenSimple(t, filepath.Join("jwt_authn_clusters", name), gotJSON), gotJSON)
			}

		})
	}
}

func makeTestProviderWithJWKS(uri string) *structs.JWTProviderConfigEntry {
	return &structs.JWTProviderConfigEntry{
		Kind:   "jwt-provider",
		Name:   "okta",
		Issuer: "test-issuer",
		JSONWebKeySet: &structs.JSONWebKeySet{
			Remote: &structs.RemoteJWKS{
				RequestTimeoutMs:    1000,
				FetchAsynchronously: true,
				URI:                 uri,
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType:  structs.DiscoveryTypeStatic,
					ConnectTimeout: time.Duration(5) * time.Second,
					TLSCertificates: &structs.JWKSTLSCertificate{
						TrustedCA: &structs.JWKSTLSCertTrustedCA{
							Filename: "mycert.crt",
						},
					},
				},
			},
		},
	}
}

func TestMakeJWKSDiscoveryClusterType(t *testing.T) {
	tests := map[string]struct {
		remoteJWKS          *structs.RemoteJWKS
		expectedClusterType *envoy_cluster_v3.Cluster_Type
	}{
		"nil remote jwks": {
			remoteJWKS:          nil,
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{},
		},
		"nil jwks cluster": {
			remoteJWKS:          &structs.RemoteJWKS{},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{},
		},
		"jwks cluster defaults to Strict DNS": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_STRICT_DNS,
			},
		},
		"jwks with cluster EDS": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "EDS",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_EDS,
			},
		},
		"jwks with static dns": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "STATIC",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_STATIC,
			},
		},

		"jwks with original dst": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "ORIGINAL_DST",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_ORIGINAL_DST,
			},
		},
		"jwks with strict dns": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "STRICT_DNS",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_STRICT_DNS,
			},
		},
		"jwks with logical dns": {
			remoteJWKS: &structs.RemoteJWKS{
				JWKSCluster: &structs.JWKSCluster{
					DiscoveryType: "LOGICAL_DNS",
				},
			},
			expectedClusterType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_LOGICAL_DNS,
			},
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			clusterType := makeJWKSDiscoveryClusterType(tt.remoteJWKS)

			require.Equal(t, tt.expectedClusterType, clusterType)
		})
	}
}

func TestParseJWTRemoteURL(t *testing.T) {
	tests := map[string]struct {
		uri            string
		expectedHost   string
		expectedPort   int
		expectedScheme string
		expectError    bool
	}{
		"invalid-url": {
			uri:         ".com",
			expectError: true,
		},
		"https-hostname-no-port": {
			uri:            "https://test.test.com",
			expectedHost:   "test.test.com",
			expectedPort:   443,
			expectedScheme: "https",
		},
		"https-hostname-with-port": {
			uri:            "https://test.test.com:4545",
			expectedHost:   "test.test.com",
			expectedPort:   4545,
			expectedScheme: "https",
		},
		"https-hostname-with-port-and-path": {
			uri:            "https://test.test.com:4545/test",
			expectedHost:   "test.test.com",
			expectedPort:   4545,
			expectedScheme: "https",
		},
		"http-hostname-no-port": {
			uri:            "http://test.test.com",
			expectedHost:   "test.test.com",
			expectedPort:   80,
			expectedScheme: "http",
		},
		"http-hostname-with-port": {
			uri:            "http://test.test.com:4636",
			expectedHost:   "test.test.com",
			expectedPort:   4636,
			expectedScheme: "http",
		},
		"https-ip-no-port": {
			uri:            "https://127.0.0.1",
			expectedHost:   "127.0.0.1",
			expectedPort:   443,
			expectedScheme: "https",
		},
		"https-ip-with-port": {
			uri:            "https://127.0.0.1:3434",
			expectedHost:   "127.0.0.1",
			expectedPort:   3434,
			expectedScheme: "https",
		},
		"http-ip-no-port": {
			uri:            "http://127.0.0.1",
			expectedHost:   "127.0.0.1",
			expectedPort:   80,
			expectedScheme: "http",
		},
		"http-ip-with-port": {
			uri:            "http://127.0.0.1:9190",
			expectedHost:   "127.0.0.1",
			expectedPort:   9190,
			expectedScheme: "http",
		},
		"http-ip-with-port-and-path": {
			uri:            "http://127.0.0.1:9190/some/where",
			expectedHost:   "127.0.0.1",
			expectedPort:   9190,
			expectedScheme: "http",
		},
		"http-ip-no-port-with-path": {
			uri:            "http://127.0.0.1/test/path",
			expectedHost:   "127.0.0.1",
			expectedPort:   80,
			expectedScheme: "http",
		},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			host, scheme, port, err := parseJWTRemoteURL(tt.uri)
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, host, tt.expectedHost)
				require.Equal(t, scheme, tt.expectedScheme)
				require.Equal(t, port, tt.expectedPort)
			}
		})
	}
}

// UID is just a convenience function to aid in writing tests less verbosely.
func UID(input string) proxycfg.UpstreamID {
	return proxycfg.UpstreamIDFromString(input)
}

func TestMakeJWTCertValidationContext(t *testing.T) {
	tests := map[string]struct {
		jwksCluster *structs.JWKSCluster
		expected    *envoy_tls_v3.CertificateValidationContext
	}{
		"when nil": {
			jwksCluster: nil,
			expected:    &envoy_tls_v3.CertificateValidationContext{},
		},
		"when trustedCA with filename": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					TrustedCA: &structs.JWKSTLSCertTrustedCA{
						Filename: "file.crt",
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_Filename{
						Filename: "file.crt",
					},
				},
			},
		},
		"when trustedCA with environment variable": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					TrustedCA: &structs.JWKSTLSCertTrustedCA{
						EnvironmentVariable: "MY_ENV",
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_EnvironmentVariable{
						EnvironmentVariable: "MY_ENV",
					},
				},
			},
		},
		"when trustedCA with inline string": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					TrustedCA: &structs.JWKSTLSCertTrustedCA{
						InlineString: "<my ca cert>",
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineString{
						InlineString: "<my ca cert>",
					},
				},
			},
		},
		"when trustedCA with inline bytes": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					TrustedCA: &structs.JWKSTLSCertTrustedCA{
						InlineBytes: []byte{1, 2, 3},
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: &envoy_core_v3.DataSource{
					Specifier: &envoy_core_v3.DataSource_InlineBytes{
						InlineBytes: []byte{1, 2, 3},
					},
				},
			},
		},
		"when caCertificateProviderInstance": {
			jwksCluster: &structs.JWKSCluster{
				TLSCertificates: &structs.JWKSTLSCertificate{
					CaCertificateProviderInstance: &structs.JWKSTLSCertProviderInstance{
						InstanceName:    "<my-instance-name>",
						CertificateName: "<my-cert>.crt",
					},
				},
			},
			expected: &envoy_tls_v3.CertificateValidationContext{
				CaCertificateProviderInstance: &envoy_tls_v3.CertificateProviderPluginInstance{
					InstanceName:    "<my-instance-name>",
					CertificateName: "<my-cert>.crt",
				},
			},
		},
	}
	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			vc := makeJWTCertValidationContext(tt.jwksCluster)

			require.Equal(t, tt.expected, vc)
		})
	}
}
