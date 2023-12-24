// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/xds/proxystateconverter"
	"github.com/hashicorp/consul/agent/xdsv2"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"google.golang.org/protobuf/proto"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/xds/testcommon"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/types"

	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/sdk/testutil"
)

var testTypeUrlToPrettyName = map[string]string{
	xdscommon.ListenerType: "listeners",
	xdscommon.RouteType:    "routes",
	xdscommon.ClusterType:  "clusters",
	xdscommon.EndpointType: "endpoints",
	xdscommon.SecretType:   "secrets",
}

type goldenTestCase struct {
	name   string
	create func(t testinf.T) *proxycfg.ConfigSnapshot
	// Setup is called before the test starts. It is passed the snapshot from
	// TestConfigSnapshot and is allowed to modify it in any way to setup the
	// test input.
	setup              func(snap *proxycfg.ConfigSnapshot)
	overrideGoldenName string
	generatorSetup     func(*ResourceGenerator)
	alsoRunTestForV2   bool
}

func TestAllResourcesFromSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	type testcase = goldenTestCase

	run := func(
		t *testing.T,
		sf xdscommon.SupportedProxyFeatures,
		envoyVersion string,
		latestEnvoyVersion string,
		tt testcase,
	) {
		// Sanity check default with no overrides first
		snap := tt.create(t)

		// TODO: it would be nice to be able to ensure these snapshots are always valid before we use them in a test.
		// require.True(t, snap.Valid())

		// We need to replace the TLS certs with deterministic ones to make golden
		// files workable. Note we don't update these otherwise they'd change
		// golden files for every test case and so not be any use!
		testcommon.SetupTLSRootsAndLeaf(t, snap)

		if tt.setup != nil {
			tt.setup(snap)
		}

		typeUrls := []string{
			xdscommon.ListenerType,
			xdscommon.RouteType,
			xdscommon.ClusterType,
			xdscommon.EndpointType,
			xdscommon.SecretType,
		}

		resourceSortingFunc := func(items []proto.Message, typeURL string) func(i, j int) bool {
			return func(i, j int) bool {
				switch typeURL {
				case xdscommon.ListenerType:
					return items[i].(*envoy_listener_v3.Listener).Name < items[j].(*envoy_listener_v3.Listener).Name
				case xdscommon.RouteType:
					return items[i].(*envoy_route_v3.RouteConfiguration).Name < items[j].(*envoy_route_v3.RouteConfiguration).Name
				case xdscommon.ClusterType:
					return items[i].(*envoy_cluster_v3.Cluster).Name < items[j].(*envoy_cluster_v3.Cluster).Name
				case xdscommon.EndpointType:
					return items[i].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName < items[j].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName
				case xdscommon.SecretType:
					return items[i].(*envoy_tls_v3.Secret).Name < items[j].(*envoy_tls_v3.Secret).Name
				default:
					panic("not possible")
				}
			}
		}

		// Need server just for logger dependency
		g := NewResourceGenerator(testutil.Logger(t), nil, false)
		g.ProxyFeatures = sf
		if tt.generatorSetup != nil {
			tt.generatorSetup(g)
		}

		resources, err := g.AllResourcesFromSnapshot(snap)
		require.NoError(t, err)

		require.Len(t, resources, len(typeUrls))

		for _, typeUrl := range typeUrls {
			prettyName := testTypeUrlToPrettyName[typeUrl]
			t.Run(fmt.Sprintf("xdsv1-%s", prettyName), func(t *testing.T) {
				items, ok := resources[typeUrl]
				require.True(t, ok)

				sort.Slice(items, resourceSortingFunc(items, typeUrl))

				r, err := response.CreateResponse(typeUrl, "00000001", "00000001", items)
				require.NoError(t, err)

				gotJSON := protoToJSON(t, r)

				gName := tt.name
				if tt.overrideGoldenName != "" {
					gName = tt.overrideGoldenName
				}

				expectedJSON := goldenEnvoy(t, filepath.Join(prettyName, gName), envoyVersion, latestEnvoyVersion, gotJSON)
				require.JSONEq(t, expectedJSON, gotJSON)
			})
		}

		if tt.alsoRunTestForV2 {
			generator := xdsv2.NewResourceGenerator(testutil.Logger(t))

			converter := proxystateconverter.NewConverter(testutil.Logger(t), &mockCfgFetcher{addressLan: "192.0.2.1"})
			proxyState, err := converter.ProxyStateFromSnapshot(snap)
			require.NoError(t, err)

			v2Resources, err := generator.AllResourcesFromIR(proxyState)
			require.NoError(t, err)
			require.Len(t, v2Resources, len(typeUrls)-1) // secrets are not currently implemented in V2.
			for _, typeUrl := range typeUrls {
				prettyName := testTypeUrlToPrettyName[typeUrl]
				t.Run(fmt.Sprintf("xdsv2-%s", prettyName), func(t *testing.T) {
					if typeUrl == xdscommon.SecretType {
						t.Skip("skipping. secrets are not yet implemented in xdsv2")
					}
					items, ok := v2Resources[typeUrl]
					require.True(t, ok)

					sort.Slice(items, resourceSortingFunc(items, typeUrl))

					r, err := response.CreateResponse(typeUrl, "00000001", "00000001", items)
					require.NoError(t, err)

					gotJSON := protoToJSON(t, r)

					gName := tt.name
					if tt.overrideGoldenName != "" {
						gName = tt.overrideGoldenName
					}

					expectedJSON := goldenEnvoy(t, filepath.Join(prettyName, gName), envoyVersion, latestEnvoyVersion, gotJSON)
					require.JSONEq(t, expectedJSON, gotJSON)
				})
			}
		}
	}

	tests := []testcase{
		{
			name: "defaults",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name:             "telemetry-collector",
			create:           proxycfg.TestConfigSnapshotTelemetryCollector,
			alsoRunTestForV2: false,
		},
		{
			name: "grpc-public-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "grpc"
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "listener-bind-address",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["bind_address"] = "127.0.0.2"
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "listener-bind-port",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["bind_port"] = 8888
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "listener-bind-address-port",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["bind_address"] = "127.0.0.2"
					ns.Proxy.Config["bind_port"] = 8888
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "listener-unix-domain-socket",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].LocalBindAddress = ""
					ns.Proxy.Upstreams[0].LocalBindPort = 0
					ns.Proxy.Upstreams[0].LocalBindSocketPath = "/tmp/service-mesh/client-1/grpc-employee-server"
					ns.Proxy.Upstreams[0].LocalBindSocketMode = "0640"
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "listener-max-inbound-connections",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["max_inbound_connections"] = 222
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "http2-public-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http2"
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "listener-balance-inbound-connections",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["balance_inbound_connections"] = "exact_balance"
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "listener-balance-outbound-connections-bind-port",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["balance_outbound_connections"] = "exact_balance"
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "http-public-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "http-public-listener-no-xfcc",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t,
					func(ns *structs.NodeService) {
						ns.Proxy.Config["protocol"] = "http"
					},
					[]proxycfg.UpdateEvent{
						{
							CorrelationID: "mesh",
							Result: &structs.ConfigEntryResponse{
								Entry: &structs.MeshConfigEntry{
									HTTP: &structs.MeshHTTPConfig{
										SanitizeXForwardedClientCert: true,
									},
								},
							},
						},
					})
			},
			alsoRunTestForV2: true,
		},
		{
			name: "http-listener-with-timeouts",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["local_connect_timeout_ms"] = 1234
					ns.Proxy.Config["local_request_timeout_ms"] = 2345
					ns.Proxy.Config["local_idle_timeout_ms"] = 3456
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "http-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["protocol"] = "http"
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-upstream-defaults",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					for _, v := range ns.Proxy.Upstreams {
						// Prepared queries do not get centrally configured upstream defaults merged into them.
						if v.DestinationType == structs.UpstreamDestTypePreparedQuery {
							continue
						}
						// Represent upstream config as if it came from centrally configured upstream defaults.
						// The name/namespace must not make it onto the cluster name attached to the outbound listener.
						v.CentrallyConfigured = true
						v.DestinationNamespace = structs.WildcardSpecifier
						v.DestinationName = structs.WildcardSpecifier
					}
				}, nil)
			},
			alsoRunTestForV2: true,
		},
	}
	tests = append(tests, getConnectProxyDiscoChainTests(false)...)
	tests = append(tests, getConnectProxyTransparentProxyGoldenTestCases()...)
	tests = append(tests, getMeshGatewayGoldenTestCases()...)
	tests = append(tests, getMeshGatewayPeeringGoldenTestCases()...)
	tests = append(tests, getTrafficControlPeeringGoldenTestCases(false)...)
	tests = append(tests, getEnterpriseGoldenTestCases(t)...)
	tests = append(tests, getAPIGatewayGoldenTestCases(t)...)
	tests = append(tests, getExposePathGoldenTestCases()...)
	tests = append(tests, getCustomConfigurationGoldenTestCases(false)...)
	tests = append(tests, getConnectProxyJWTProviderGoldenTestCases()...)
	tests = append(tests, getTerminatingGatewayPeeringGoldenTestCases()...)
	tests = append(tests, getIngressGatewayGoldenTestCases()...)
	tests = append(tests, getAccessLogsGoldenTestCases()...)
	tests = append(tests, getTLSGoldenTestCases()...)
	tests = append(tests, getPeeredGoldenTestCases()...)
	tests = append(tests, getXDSFetchTimeoutTestCases()...)

	latestEnvoyVersion := xdscommon.EnvoyVersions[0]
	for _, envoyVersion := range xdscommon.EnvoyVersions {
		sf, err := xdscommon.DetermineSupportedProxyFeaturesFromString(envoyVersion)
		require.NoError(t, err)
		t.Run("envoy-"+envoyVersion, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					run(t, sf, envoyVersion, latestEnvoyVersion, tt)
				})
			}
		})
	}
}

func getConnectProxyTransparentProxyGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "transparent-proxy",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTransparentProxy(t)
			},
			alsoRunTestForV2: true,
		},
		{
			name:             "transparent-proxy-catalog-destinations-only",
			create:           proxycfg.TestConfigSnapshotTransparentProxyCatalogDestinationsOnly,
			alsoRunTestForV2: true,
		},
		{
			name:             "transparent-proxy-dial-instances-directly",
			create:           proxycfg.TestConfigSnapshotTransparentProxyDialDirectly,
			alsoRunTestForV2: true,
		},
		{
			name: "transparent-proxy-http-upstream",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTransparentProxyHTTPUpstream(t, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name:             "transparent-proxy-with-resolver-redirect-upstream",
			create:           proxycfg.TestConfigSnapshotTransparentProxyResolverRedirectUpstream,
			alsoRunTestForV2: true,
		},
		{
			name:             "transparent-proxy-terminating-gateway",
			create:           proxycfg.TestConfigSnapshotTransparentProxyTerminatingGatewayCatalogDestinationsOnly,
			alsoRunTestForV2: true,
		},
		{
			name:   "transparent-proxy-destination",
			create: proxycfg.TestConfigSnapshotTransparentProxyDestination,
			// TODO(proxystate): currently failing. should work.  possible issue in converter.
			alsoRunTestForV2: false,
		},
		{
			name: "transparent-proxy-destination-http",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTransparentProxyDestinationHTTP(t, nil)
			},
			// TODO(proxystate): currently failing. should work.  possible issue in converter.
			alsoRunTestForV2: false,
		},
		{
			name: "transparent-proxy-terminating-gateway-destinations-only",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGatewayDestinations(t, true, nil)
			},
			// TODO(proxystate): terminating gateways will come at a later date.
			alsoRunTestForV2: false,
		},
	}
}

func getConnectProxyDiscoChainTests(enterprise bool) []goldenTestCase {
	return []goldenTestCase{
		{
			name: "connect-proxy-with-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", false, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-external-sni",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "external-sni", false, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-and-failover",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover", false, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-exported-to-peers",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					// This test is only concerned about the SPIFFE cert validator config in the public listener
					// so we empty out the upstreams to avoid generating unnecessary upstream listeners.
					ns.Proxy.Upstreams = structs.Upstreams{}
				}, []proxycfg.UpdateEvent{
					{
						CorrelationID: "peering-trust-bundles",
						Result:        proxycfg.TestPeerTrustBundles(t),
					},
				})
			},
			alsoRunTestForV2: true,
		},
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
			// TODO(proxystate): requires custom cluster work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-chain-http2",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["protocol"] = "http2"
				}, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-and-overrides",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple-with-overrides", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-and-failover",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway-triggered", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway-triggered", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway-triggered", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway-triggered", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "splitter-with-resolver-redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "splitter-with-resolver-redirect-multidc", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-lb-in-resolver",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "lb-resolver", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-default-chain-and-custom-cluster",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", enterprise, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
						customAppClusterJSON(t, customClusterJSONOptions{
							Name: "myservice",
						})
				}, nil)
			},
			// TODO(proxystate): requires custom cluster work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-splitter-overweight",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "splitter-overweight", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-and-splitter",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "chain-and-splitter", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-grpc-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "grpc-router", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-and-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "chain-and-router", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-route-to-lb-resolver",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "redirect-to-lb-node", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-resolver-with-lb",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "resolver-with-lb", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "custom-upstream-ignored-with-disco-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover", enterprise, func(ns *structs.NodeService) {
					for i := range ns.Proxy.Upstreams {
						if ns.Proxy.Upstreams[i].DestinationName != "db" {
							continue // only tweak the db upstream
						}
						if ns.Proxy.Upstreams[i].Config == nil {
							ns.Proxy.Upstreams[i].Config = map[string]interface{}{}
						}

						uid := proxycfg.NewUpstreamID(&ns.Proxy.Upstreams[i])

						ns.Proxy.Upstreams[i].Config["envoy_listener_json"] =
							customListenerJSON(t, customListenerJSONOptions{
								Name: uid.EnvoyID() + ":custom-upstream",
							})
					}
				}, nil)
			},
		},
		{
			name: "connect-proxy-with-tcp-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-http-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil,
					&structs.ProxyConfigEntry{
						Kind:     structs.ProxyDefaults,
						Name:     structs.ProxyConfigGlobal,
						Protocol: "http",
						Config: map[string]interface{}{
							"protocol": "http",
						},
					},
				)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-http2-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil,
					&structs.ProxyConfigEntry{
						Kind:     structs.ProxyDefaults,
						Name:     structs.ProxyConfigGlobal,
						Protocol: "http2",
						Config: map[string]interface{}{
							"protocol": "http2",
						},
					},
				)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-grpc-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil,
					&structs.ProxyConfigEntry{
						Kind:     structs.ProxyDefaults,
						Name:     structs.ProxyConfigGlobal,
						Protocol: "grpc",
						Config: map[string]interface{}{
							"protocol": "grpc",
						},
					},
				)
			},
			alsoRunTestForV2: true,
		},
	}
}

func getMeshGatewayGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "mesh-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "default", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-using-federation-states",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "federation-states", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-using-federation-control-plane",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "mesh-gateway-federation", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-newer-information-in-federation-states",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "newer-info-in-federation-states", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-older-information-in-federation-states",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "older-info-in-federation-states", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-no-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "no-services", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-service-subsets",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "service-subsets", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-service-subsets2", // TODO: make this merge with 'service-subsets'
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "service-subsets2", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-default-service-subset",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "default-service-subsets2", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-ignore-extra-resolvers",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "ignore-extra-resolvers", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-service-timeouts",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "service-timeouts", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-non-hash-lb-injected",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "non-hash-lb-injected", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-hash-lb-ignored",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "hash-lb-ignored", nil, nil)
			},
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): mesh gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-tagged-addresses",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "default", func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"envoy_mesh_gateway_no_default_bind":       true,
						"envoy_mesh_gateway_bind_tagged_addresses": true,
					}
				}, nil)
			},
		},
		{
			name: "mesh-gateway-custom-addresses",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "default", func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"envoy_mesh_gateway_bind_addresses": map[string]structs.ServiceAddress{
							"foo": {
								Address: "198.17.2.3",
								Port:    8080,
							},
							"bar": {
								Address: "2001:db8::ff",
								Port:    9999,
							},
							"baz": {
								Address: "127.0.0.1",
								Port:    8765,
							},
						},
					}
				}, nil)
			},
		},
	}
}
func getMeshGatewayPeeringGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "mesh-gateway-with-exported-peered-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "default-services-tcp", nil, nil)
			},
			// TODO(proxystate): mesh gateways will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-with-exported-peered-services-http",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "default-services-http", nil, nil)
			},
			// TODO(proxystate): mesh gateways will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-with-exported-peered-services-http-with-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "chain-and-l7-stuff", nil, nil)
			},
			// TODO(proxystate): mesh gateways will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-peering-control-plane",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "control-plane", nil, nil)
			},
			// TODO(proxystate): mesh gateways will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-with-imported-peered-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "imported-services", func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"envoy_dns_discovery_type": "STRICT_DNS",
					}
				}, nil)
			},
			// TODO(proxystate): mesh gateways will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "mesh-gateway-with-peer-through-mesh-gateway-enabled",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "peer-through-mesh-gateway", nil, nil)
			},
			// TODO(proxystate): mesh gateways will come at a later date.
			alsoRunTestForV2: false,
		},
	}
}

func getTrafficControlPeeringGoldenTestCases(enterprise bool) []goldenTestCase {
	cases := []goldenTestCase{
		{
			name: "connect-proxy-with-chain-and-failover-to-cluster-peer",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-to-cluster-peer", enterprise, nil, nil)
			},
			// TODO(proxystate): peering will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-chain-and-redirect-to-cluster-peer",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "redirect-to-cluster-peer", enterprise, nil, nil)
			},
			// TODO(proxystate): peering will come at a later date.
			alsoRunTestForV2: false,
		},
	}

	if enterprise {
		for i := range cases {
			cases[i].name = "enterprise-" + cases[i].name
		}
	}

	return cases
}

const (
	gatewayTestPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAx95Opa6t4lGEpiTUogEBptqOdam2ch4BHQGhNhX/MrDwwuZQ
httBwMfngQ/wd9NmYEPAwj0dumUoAITIq6i2jQlhqTodElkbsd5vWY8R/bxJWQSo
NvVE12TlzECxGpJEiHt4W0r8pGffk+rvpljiUyCfnT1kGF3znOSjK1hRMTn6RKWC
yYaBvXQiB4SGilfLgJcEpOJKtISIxmZ+S409g9X5VU88/Bmmrz4cMyxce86Kc2ug
5/MOv0CjWDJwlrv8njneV2zvraQ61DDwQftrXOvuCbO5IBRHMOBHiHTZ4rtGuhMa
Ir21V4vb6n8c4YzXiFvhUYcyX7rltGZzVd+WmQIDAQABAoIBACYvceUzp2MK4gYA
GWPOP2uKbBdM0l+hHeNV0WAM+dHMfmMuL4pkT36ucqt0ySOLjw6rQyOZG5nmA6t9
sv0g4ae2eCMlyDIeNi1Yavu4Wt6YX4cTXbQKThm83C6W2X9THKbauBbxD621bsDK
7PhiGPN60yPue7YwFQAPqqD4YaK+s22HFIzk9gwM/rkvAUNwRv7SyHMiFe4Igc1C
Eev7iHWzvj5Heoz6XfF+XNF9DU+TieSUAdjd56VyUb8XL4+uBTOhHwLiXvAmfaMR
HvpcxeKnYZusS6NaOxcUHiJnsLNWrxmJj9WEGgQzuLxcLjTe4vVmELVZD8t3QUKj
PAxu8tUCgYEA7KIWVn9dfVpokReorFym+J8FzLwSktP9RZYEMonJo00i8aii3K9s
u/aSwRWQSCzmON1ZcxZzWhwQF9usz6kGCk//9+4hlVW90GtNK0RD+j7sp4aT2JI8
9eLEjTG+xSXa7XWe98QncjjL9lu/yrRncSTxHs13q/XP198nn2aYuQ8CgYEA2Dnt
sRBzv0fFEvzzFv7G/5f85mouN38TUYvxNRTjBLCXl9DeKjDkOVZ2b6qlfQnYXIru
H+W+v+AZEb6fySXc8FRab7lkgTMrwE+aeI4rkW7asVwtclv01QJ5wMnyT84AgDD/
Dgt/RThFaHgtU9TW5GOZveL+l9fVPn7vKFdTJdcCgYEArJ99zjHxwJ1whNAOk1av
09UmRPm6TvRo4heTDk8oEoIWCNatoHI0z1YMLuENNSnT9Q280FFDayvnrY/qnD7A
kktT/sjwJOG8q8trKzIMqQS4XWm2dxoPcIyyOBJfCbEY6XuRsUuePxwh5qF942EB
yS9a2s6nC4Ix0lgPrqAIr48CgYBgS/Q6riwOXSU8nqCYdiEkBYlhCJrKpnJxF9T1
ofa0yPzKZP/8ZEfP7VzTwHjxJehQ1qLUW9pG08P2biH1UEKEWdzo8vT6wVJT1F/k
HtTycR8+a+Hlk2SHVRHqNUYQGpuIe8mrdJ1as4Pd0d/F/P0zO9Rlh+mAsGPM8HUM
T0+9gwKBgHDoerX7NTskg0H0t8O+iSMevdxpEWp34ZYa9gHiftTQGyrRgERCa7Gj
nZPAxKb2JoWyfnu3v7G5gZ8fhDFsiOxLbZv6UZJBbUIh1MjJISpXrForDrC2QNLX
kHrHfwBFDB3KMudhQknsJzEJKCL/KmFH6o0MvsoaT9yzEl3K+ah/
-----END RSA PRIVATE KEY-----`
	gatewayTestCertificate = `-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQCQMDsYO8FrPjANBgkqhkiG9w0BAQsFADANMQswCQYDVQQGEwJV
UzAeFw0yMjEyMjAxNzUwMjVaFw0yNzEyMTkxNzUwMjVaMA0xCzAJBgNVBAYTAlVT
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAx95Opa6t4lGEpiTUogEB
ptqOdam2ch4BHQGhNhX/MrDwwuZQhttBwMfngQ/wd9NmYEPAwj0dumUoAITIq6i2
jQlhqTodElkbsd5vWY8R/bxJWQSoNvVE12TlzECxGpJEiHt4W0r8pGffk+rvplji
UyCfnT1kGF3znOSjK1hRMTn6RKWCyYaBvXQiB4SGilfLgJcEpOJKtISIxmZ+S409
g9X5VU88/Bmmrz4cMyxce86Kc2ug5/MOv0CjWDJwlrv8njneV2zvraQ61DDwQftr
XOvuCbO5IBRHMOBHiHTZ4rtGuhMaIr21V4vb6n8c4YzXiFvhUYcyX7rltGZzVd+W
mQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQBfCqoUIdPf/HGSbOorPyZWbyizNtHJ
GL7x9cAeIYxpI5Y/WcO1o5v94lvrgm3FNfJoGKbV66+JxOge731FrfMpHplhar1Z
RahYIzNLRBTLrwadLAZkApUpZvB8qDK4knsTWFYujNsylCww2A6ajzIMFNU4GkUK
NtyHRuD+KYRmjXtyX1yHNqfGN3vOQmwavHq2R8wHYuBSc6LAHHV9vG+j0VsgMELO
qwxn8SmLkSKbf2+MsQVzLCXXN5u+D8Yv+4py+oKP4EQ5aFZuDEx+r/G/31rTthww
AAJAMaoXmoYVdgXV+CPuBb2M4XCpuzLu3bcA2PXm5ipSyIgntMKwXV7r
-----END CERTIFICATE-----`
	// openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem -sha256 -days 3650 \
	// -nodes -subj "/C=XX/CN=secondcert.com" -addext "subjectAltName = DNS:secondcert.com"
	gatewayTestPrivateKeyTwo = `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQCiPr2HCbVzbZ1M
IW89rfLLrciPTWWl48DF9CmYHS0C2gSD1W6bxzO7zdA+ced0ajI+YsQ9aBAXRhKl
EHgnhBJ6sGsz1XBQ9+lNDHrg9AjugIiHoscYOeCcxMeXhp97ti+vpVsc2/AvEf2K
GIUuOjcufXuRXkWQ2aB4RGyodkgRF6n8YrLJb7pWIjoCNwDAWtZX4wIVFgGq1ew0
E/E9EyStMYTb5h1lvCpXYRN9AeSFKUQI/y0xsT3+nZ/gyzx3CrgzuSYRgptbuVwm
5F2Q16sLR/EtCBIhA8npKagx/4U7KOilF31I2locH4Aq5l9VJd/6pTA5F4KCAW/E
ybXz6DojAgMBAAECggEAPcOuuRqsFf4ztIjB5XQ0Cu/kexFW0flLKNDTiNIKkZxX
vaxhyDHkculeDnekSkAnUnKdDFdyULnfXTFQ3JI9yrEgjoIBmQFXsno+ySZ9w/Xw
g9om+wUFigirhva7/geUTcSgU/Myk2jA4XKGONv2p98jTGrcBtGickZyKwukUcTa
M18phLdjejg09d45QV5pEtU5m0HuydvtMNCxL2UeWMxyIVezAH2S48m7IAn7Xs4p
J9bwjboDWQYs+zLPfEZyosiJiKugpEKvApIKsJXf4JqRXHN+vvKKDeXkKrrGR+pg
3e5foPjFrLcDltZMkrfnlm8fa0yLnoxdiyd1pDcJaQKBgQDSnJbM6CDb0b3bUyiz
LpfJSBzEPqABM8mNeVHfEjHcBJ7YBOceBxDNasmAPvFbhoDrlHiEYW2QnDLRXKeF
XVdXjSsUV30SPMeg6yeSd8L+LKXLjrGMNGDcJfnjLavv7Glu1xDnYyFSmeVIhWoo
cOhfaFQ69vnHiU1idrOlz6zhPwKBgQDFNcY0S59f3tht7kgnItg8PSfJqJQrIdLt
x6MC2Nc7Eui7/LTuO2rMG6HRA/8zQa/TfnfG7CsUwLB1NiC2R1TtM9YBYPxhMl1o
JeGTfM+tD0lBwPhYpgnOCppuayRCfAsPYA6NcvXcGZbxOigxliOuvgVBH47EAApA
zJ+8b6nKHQKBgQCZ0GDV/4XX5KNq5Z3o1tNl3jOcIzyKBD9kAkGHz+r4C6vSiioc
pP5hd2b4MX/l3yKSapll3R2+qkT24Fs8LEJYn7Hhpk+inR8SaAs7jhmrtgHT2z/R
7IL85QNOJhHXJGqP16PxyVUR1XE9eKpiJKug2joB4lPjpWQN0DE9nKFe0wKBgEo3
qpgTva7+1sTIYC8aVfaVrVufLePtnswNzbNMl/OLcjsNJ6pgghi+bW+T6X8IwXr+
pWUfjDcLLV1vOXBf9/4s++UY8uJBahW/69zto9qlXhR44v25vwbjxqq3d7XtqNvo
cpGZKh3jI4M1N9sxfcxNhvyzO69XtIQefh8UhvmhAoGBAKzSA51l50ocOnWSNAXs
QQoU+dYQjLDMtzc5N68EUf1GSjtgkpa3PYjVo09OMeb7+W9LhwHQDNMqgeeEDCsm
B6NDnI4VyjVae7Hqz48WBERJBFMFWiLxEa1m2UwaV2jAubN8FKgH4KzDzOKtJEUy
Rz9IUct6HXsDSs+Q3/zdFmPo
-----END PRIVATE KEY-----`
	gatewayTestCertificateTwo = `-----BEGIN CERTIFICATE-----
MIIC7DCCAdSgAwIBAgIJAMHpuSA3ioNPMA0GCSqGSIb3DQEBCwUAMCYxCzAJBgNV
BAYTAlhYMRcwFQYDVQQDDA5zZWNvbmRjZXJ0LmNvbTAeFw0yMzA3MTExNTE1MjBa
Fw0zMzA3MDgxNTE1MjBaMCYxCzAJBgNVBAYTAlhYMRcwFQYDVQQDDA5zZWNvbmRj
ZXJ0LmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAKI+vYcJtXNt
nUwhbz2t8sutyI9NZaXjwMX0KZgdLQLaBIPVbpvHM7vN0D5x53RqMj5ixD1oEBdG
EqUQeCeEEnqwazPVcFD36U0MeuD0CO6AiIeixxg54JzEx5eGn3u2L6+lWxzb8C8R
/YoYhS46Ny59e5FeRZDZoHhEbKh2SBEXqfxisslvulYiOgI3AMBa1lfjAhUWAarV
7DQT8T0TJK0xhNvmHWW8KldhE30B5IUpRAj/LTGxPf6dn+DLPHcKuDO5JhGCm1u5
XCbkXZDXqwtH8S0IEiEDyekpqDH/hTso6KUXfUjaWhwfgCrmX1Ul3/qlMDkXgoIB
b8TJtfPoOiMCAwEAAaMdMBswGQYDVR0RBBIwEIIOc2Vjb25kY2VydC5jb20wDQYJ
KoZIhvcNAQELBQADggEBAJvP3deuEpJZktAny6/az09GLSUYddiNCE4sG/2ASj7C
mwRTh2HM4BDnkhW9PNjfHoaWa2TDIhOyHQ5hLYz2tnaeU1sOrADCuFSxGiQqgr8J
prahKh6AzNsXba4rumoO08QTTtJzoa8L6TV4PTQ6gi+OMdbyBe3CQ7DSRzLseHNH
KG5tqRRu+Jm7dUuOXDV4MDHoloyZlksOvIYSC+gaS+ke3XlR+GzOW7hpgn5SIDlv
aR/zlIKXUCvVux3/pNFgW6rduFE0f5Hbc1+J4ghTl8EQu1dwDTax7blXQwE+VDgJ
u4fZGRmoUvvO/bjVCbehBxfJn0rHsxpuD5b4Jg2OZNc=
-----END CERTIFICATE-----`
)

func getAPIGatewayGoldenTestCases(t *testing.T) []goldenTestCase {
	t.Helper()

	service := structs.NewServiceName("service", nil)
	serviceUID := proxycfg.NewUpstreamIDFromServiceName(service)
	serviceChain := discoverychain.TestCompileConfigEntries(t, "service", "default", "default", "dc1", connect.TestClusterID+".consul", nil, nil)

	return []goldenTestCase{
		{
			name: "api-gateway-with-tcp-route-and-inline-certificate",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolTCP,
							Port:     8080,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "certificate",
								}},
							},
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
							Routes: []structs.ResourceReference{{
								Kind: structs.TCPRoute,
								Name: "route",
							}},
						},
					}
				},
					[]structs.BoundRoute{
						&structs.TCPRouteConfigEntry{
							Kind: structs.TCPRoute,
							Name: "route",
							Services: []structs.TCPService{{
								Name: "service",
							}},
							Parents: []structs.ResourceReference{
								{
									Kind: structs.APIGateway,
									Name: "api-gateway",
								},
							},
						},
					}, []structs.InlineCertificateConfigEntry{{
						Kind:        structs.InlineCertificate,
						Name:        "certificate",
						PrivateKey:  gatewayTestPrivateKey,
						Certificate: gatewayTestCertificate,
					}}, nil)
			},
			// TODO(proxystate): api gateways will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "api-gateway-with-multiple-inline-certificates",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolTCP,
							Port:     8080,
							TLS: structs.APIGatewayTLSConfiguration{
								Certificates: []structs.ResourceReference{{
									Kind: structs.InlineCertificate,
									Name: "certificate",
								}},
								MinVersion: types.TLSv1_2,
								MaxVersion: types.TLSv1_3,
								CipherSuites: []types.TLSCipherSuite{
									types.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
									types.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
								},
							},
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
							Certificates: []structs.ResourceReference{
								{
									Kind: structs.InlineCertificate,
									Name: "certificate",
								},
								{
									Kind: structs.InlineCertificate,
									Name: "certificate-too",
								},
							},
							Routes: []structs.ResourceReference{{
								Kind: structs.TCPRoute,
								Name: "route",
							}},
						},
					}
				},
					[]structs.BoundRoute{
						&structs.TCPRouteConfigEntry{
							Kind: structs.TCPRoute,
							Name: "route",
							Services: []structs.TCPService{{
								Name: "service",
							}},
							Parents: []structs.ResourceReference{
								{
									Kind: structs.APIGateway,
									Name: "api-gateway",
								},
							},
						},
					}, []structs.InlineCertificateConfigEntry{
						{
							Kind:        structs.InlineCertificate,
							Name:        "certificate",
							PrivateKey:  gatewayTestPrivateKey,
							Certificate: gatewayTestCertificate,
						},
						{
							Kind:        structs.InlineCertificate,
							Name:        "certificate-too",
							PrivateKey:  gatewayTestPrivateKeyTwo,
							Certificate: gatewayTestCertificateTwo,
						},
					}, nil)
			},
			// TODO(proxystate): api gateways will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "api-gateway-with-http-route",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
							Routes: []structs.ResourceReference{{
								Kind: structs.HTTPRoute,
								Name: "route",
							}},
						},
					}
				}, []structs.BoundRoute{
					&structs.HTTPRouteConfigEntry{
						Kind: structs.HTTPRoute,
						Name: "route",
						Rules: []structs.HTTPRouteRule{{
							Filters: structs.HTTPFilters{
								Headers: []structs.HTTPHeaderFilter{
									{
										Add: map[string]string{
											"X-Header-Add": "added",
										},
										Set: map[string]string{
											"X-Header-Set": "set",
										},
										Remove: []string{"X-Header-Remove"},
									},
								},
								RetryFilter: &structs.RetryFilter{
									NumRetries:            3,
									RetryOn:               []string{"cancelled"},
									RetryOnStatusCodes:    []uint32{500},
									RetryOnConnectFailure: true,
								},
								TimeoutFilter: &structs.TimeoutFilter{
									IdleTimeout:    time.Second * 30,
									RequestTimeout: time.Second * 30,
								},
							},
							Services: []structs.HTTPService{{
								Name: "service",
							}},
						}},
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
					},
				}, []structs.InlineCertificateConfigEntry{{
					Kind:        structs.InlineCertificate,
					Name:        "certificate",
					PrivateKey:  gatewayTestPrivateKey,
					Certificate: gatewayTestCertificate,
				}}, []proxycfg.UpdateEvent{{
					CorrelationID: "discovery-chain:" + serviceUID.String(),
					Result: &structs.DiscoveryChainResponse{
						Chain: serviceChain,
					},
				}, {
					CorrelationID: "upstream-target:" + serviceChain.ID() + ":" + serviceUID.String(),
					Result: &structs.IndexedCheckServiceNodes{
						Nodes: proxycfg.TestUpstreamNodes(t, "service"),
					},
				}})
			},
			// TODO(proxystate): api gateways will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "api-gateway-with-http-route-timeoutfilter-one-set",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
							Certificates: []structs.ResourceReference{{
								Kind: structs.InlineCertificate,
								Name: "certificate",
							}},
							Routes: []structs.ResourceReference{{
								Kind: structs.HTTPRoute,
								Name: "route",
							}},
						},
					}
				}, []structs.BoundRoute{
					&structs.HTTPRouteConfigEntry{
						Kind: structs.HTTPRoute,
						Name: "route",
						Rules: []structs.HTTPRouteRule{{
							Filters: structs.HTTPFilters{
								Headers: []structs.HTTPHeaderFilter{
									{
										Add: map[string]string{
											"X-Header-Add": "added",
										},
										Set: map[string]string{
											"X-Header-Set": "set",
										},
										Remove: []string{"X-Header-Remove"},
									},
								},
								TimeoutFilter: &structs.TimeoutFilter{
									IdleTimeout: time.Second * 30,
								},
							},
							Services: []structs.HTTPService{{
								Name: "service",
							}},
						}},
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
					},
				}, []structs.InlineCertificateConfigEntry{{
					Kind:        structs.InlineCertificate,
					Name:        "certificate",
					PrivateKey:  gatewayTestPrivateKey,
					Certificate: gatewayTestCertificate,
				}}, []proxycfg.UpdateEvent{{
					CorrelationID: "discovery-chain:" + serviceUID.String(),
					Result: &structs.DiscoveryChainResponse{
						Chain: serviceChain,
					},
				}, {
					CorrelationID: "upstream-target:" + serviceChain.ID() + ":" + serviceUID.String(),
					Result: &structs.IndexedCheckServiceNodes{
						Nodes: proxycfg.TestUpstreamNodes(t, "service"),
					},
				}})
			},
			// TODO(proxystate): api gateways will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "api-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, nil, nil, nil, nil)
			},
		},
		{
			name: "api-gateway-nil-config-entry",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway_NilConfigEntry(t)
			},
		},
		{
			name: "api-gateway-tcp-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolTCP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
						},
					}
				}, nil, nil, nil)
			},
		},
		{
			name: "api-gateway-tcp-listener-with-tcp-route",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolTCP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
							Routes: []structs.ResourceReference{
								{
									Name: "tcp-route",
									Kind: structs.TCPRoute,
								},
							},
						},
					}

				}, []structs.BoundRoute{
					&structs.TCPRouteConfigEntry{
						Name: "tcp-route",
						Kind: structs.TCPRoute,
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
						Services: []structs.TCPService{
							{Name: "tcp-service"},
						},
					},
				}, nil, nil)
			},
		},
		{
			name: "api-gateway-http-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name:   "listener",
							Routes: []structs.ResourceReference{},
						},
					}
				}, nil, nil, nil)
			},
		},
		{
			name: "api-gateway-http-listener-with-http-route",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8080,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener",
							Routes: []structs.ResourceReference{
								{
									Name: "http-route",
									Kind: structs.HTTPRoute,
								},
							},
						},
					}
				}, []structs.BoundRoute{
					&structs.HTTPRouteConfigEntry{
						Name: "http-route",
						Kind: structs.HTTPRoute,
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
						Rules: []structs.HTTPRouteRule{
							{
								Services: []structs.HTTPService{
									{Name: "http-service"},
								},
							},
						},
					},
				}, nil, nil)
			},
		},
		{
			name: "api-gateway-tcp-listener-with-tcp-and-http-route",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "listener-tcp",
							Protocol: structs.ListenerProtocolTCP,
							Port:     8080,
						},
						{
							Name:     "listener-http",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8081,
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "listener-tcp",
							Routes: []structs.ResourceReference{
								{
									Name: "tcp-route",
									Kind: structs.TCPRoute,
								},
							},
						},
						{
							Name: "listener-http",
							Routes: []structs.ResourceReference{
								{
									Name: "http-route",
									Kind: structs.HTTPRoute,
								},
							},
						},
					}
				}, []structs.BoundRoute{
					&structs.TCPRouteConfigEntry{
						Name: "tcp-route",
						Kind: structs.TCPRoute,
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
						Services: []structs.TCPService{
							{Name: "tcp-service"},
						},
					},
					&structs.HTTPRouteConfigEntry{
						Name: "http-route",
						Kind: structs.HTTPRoute,
						Parents: []structs.ResourceReference{
							{
								Kind: structs.APIGateway,
								Name: "api-gateway",
							},
						},
						Rules: []structs.HTTPRouteRule{
							{
								Services: []structs.HTTPService{
									{Name: "http-service"},
								},
							},
						},
					},
				}, nil, nil)
			},
		},
		{
			name: "api-gateway-with-multiple-hostnames",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotAPIGateway(t, "default", nil, func(entry *structs.APIGatewayConfigEntry, bound *structs.BoundAPIGatewayConfigEntry) {
					entry.Listeners = []structs.APIGatewayListener{
						{
							Name:     "http",
							Protocol: structs.ListenerProtocolHTTP,
							Port:     8080,
							Hostname: "*.example.com",
						},
					}
					bound.Listeners = []structs.BoundAPIGatewayListener{
						{
							Name: "http",
							Routes: []structs.ResourceReference{
								{Kind: structs.HTTPRoute, Name: "backend-route"},
								{Kind: structs.HTTPRoute, Name: "frontend-route"},
								{Kind: structs.HTTPRoute, Name: "generic-route"},
							}},
					}
				},
					[]structs.BoundRoute{
						&structs.HTTPRouteConfigEntry{
							Kind:      structs.HTTPRoute,
							Name:      "backend-route",
							Hostnames: []string{"backend.example.com"},
							Parents:   []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gateway"}},
							Rules: []structs.HTTPRouteRule{
								{Services: []structs.HTTPService{{Name: "backend"}}},
							},
						},
						&structs.HTTPRouteConfigEntry{
							Kind:      structs.HTTPRoute,
							Name:      "frontend-route",
							Hostnames: []string{"frontend.example.com"},
							Parents:   []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gateway"}},
							Rules: []structs.HTTPRouteRule{
								{Services: []structs.HTTPService{{Name: "frontend"}}},
							},
						},
						&structs.HTTPRouteConfigEntry{
							Kind:    structs.HTTPRoute,
							Name:    "generic-route",
							Parents: []structs.ResourceReference{{Kind: structs.APIGateway, Name: "api-gateway"}},
							Rules: []structs.HTTPRouteRule{
								{
									Matches:  []structs.HTTPMatch{{Path: structs.HTTPPathMatch{Match: structs.HTTPPathMatchPrefix, Value: "/frontend"}}},
									Services: []structs.HTTPService{{Name: "frontend"}},
								},
								{
									Matches:  []structs.HTTPMatch{{Path: structs.HTTPPathMatch{Match: structs.HTTPPathMatchPrefix, Value: "/backend"}}},
									Services: []structs.HTTPService{{Name: "backend"}},
								},
							},
						},
					}, nil, nil)
			},
		},
	}
}

func getExposePathGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "expose-paths-local-app-paths",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotExposeConfig(t, nil)
			},
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
		},
		{
			name:   "expose-checks",
			create: proxycfg.TestConfigSnapshotExposeChecks,
			generatorSetup: func(s *ResourceGenerator) {
				s.CfgFetcher = configFetcherFunc(func() string {
					return "192.0.2.1"
				})
			},
			alsoRunTestForV2: true,
		},
		{
			name:             "expose-paths-grpc-new-cluster-http1",
			create:           proxycfg.TestConfigSnapshotGRPCExposeHTTP1,
			alsoRunTestForV2: true,
		},
		{
			// NOTE: if IPv6 is not supported in the kernel per
			// platform.SupportsIPv6() then this test will fail because the golden
			// files were generated assuming ipv6 support was present
			name:   "expose-checks-http",
			create: proxycfg.TestConfigSnapshotExposeChecks,
			generatorSetup: func(s *ResourceGenerator) {
				s.CfgFetcher = configFetcherFunc(func() string {
					return "192.0.2.1"
				})
			},
		},
		{
			// NOTE: if IPv6 is not supported in the kernel per
			// platform.SupportsIPv6() then this test will fail because the golden
			// files were generated assuming ipv6 support was present
			name:   "expose-checks-http-with-bind-override",
			create: proxycfg.TestConfigSnapshotExposeChecksWithBindOverride,
			generatorSetup: func(s *ResourceGenerator) {
				s.CfgFetcher = configFetcherFunc(func() string {
					return "192.0.2.1"
				})
			},
		},
		{
			// NOTE: if IPv6 is not supported in the kernel per
			// platform.SupportsIPv6() then this test will fail because the golden
			// files were generated assuming ipv6 support was present
			name:   "expose-checks-grpc",
			create: proxycfg.TestConfigSnapshotExposeChecksGRPC,
			generatorSetup: func(s *ResourceGenerator) {
				s.CfgFetcher = configFetcherFunc(func() string {
					return "192.0.2.1"
				})
			},
		},
	}
}

func getCustomConfigurationGoldenTestCases(enterprise bool) []goldenTestCase {
	return []goldenTestCase{
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
			// TODO(proxystate): requires custom cluster work
			alsoRunTestForV2: false,
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
			// TODO(proxystate): requires custom cluster work
			alsoRunTestForV2: false,
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
			// TODO(proxystate): requires custom cluster work
			alsoRunTestForV2: false,
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
			// TODO(proxystate): requires custom cluster work
			alsoRunTestForV2: false,
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
			// TODO(proxystate): requires custom cluster work
			alsoRunTestForV2: false,
		},
		{
			name: "custom-timeouts",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["local_connect_timeout_ms"] = 1234
					ns.Proxy.Upstreams[0].Config["connect_timeout_ms"] = 2345
				}, nil)
			},
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
		},
		{
			name: "custom-max-inbound-connections",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["max_inbound_connections"] = 3456
				}, nil)
			},
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-default-chain-and-custom-cluster",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", enterprise, func(ns *structs.NodeService) {
					ns.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
						customAppClusterJSON(t, customClusterJSONOptions{
							Name: "myservice",
						})
				}, nil)
			},
			// TODO(proxystate): requires custom cluster work
			alsoRunTestForV2: false,
		},
		{
			name: "custom-public-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["envoy_public_listener_json"] =
						customListenerJSON(t, customListenerJSONOptions{
							Name: "custom-public-listen",
						})
				}, nil)
			},
		},
		{
			name: "custom-public-listener-http",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["envoy_public_listener_json"] =
						customHTTPListenerJSON(t, customHTTPListenerJSONOptions{
							Name: "custom-public-listen",
						})
				}, nil)
			},
		},
		{
			name: "custom-public-listener-http-2",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["envoy_public_listener_json"] =
						customHTTPListenerJSON(t, customHTTPListenerJSONOptions{
							Name:                      "custom-public-listen",
							HTTPConnectionManagerName: httpConnectionManagerNewName,
						})
				}, nil)
			},
		},
		{
			name: "custom-public-listener-http-missing",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["envoy_public_listener_json"] =
						customListenerJSON(t, customListenerJSONOptions{
							Name: "custom-public-listen",
						})
				}, nil)
			},
		},
		{
			name:               "custom-public-listener-ignores-tls",
			overrideGoldenName: "custom-public-listener", // should be the same
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["envoy_public_listener_json"] =
						customListenerJSON(t, customListenerJSONOptions{
							Name: "custom-public-listen",
							// Attempt to override the TLS context should be ignored
							TLSContext: `"allowRenegotiation": false`,
						})
				}, nil)
			},
		},
		{
			name: "custom-trace-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["envoy_listener_tracing_json"] = customTraceJSON(t)
				}, nil)
			},
		},
	}
}

func getConnectProxyJWTProviderGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
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
			// TODO(proxystate): jwt work will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): jwt work will come at a later time
			alsoRunTestForV2: false,
		},
	}
}

func getTerminatingGatewayPeeringGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "terminating-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, nil)
			},
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "terminating-gateway-no-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, false, nil, nil)
			},
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "terminating-gateway-service-subsets",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGatewayServiceSubsetsWebAndCache(t, nil)
			},
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-hostname-service-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayHostnameSubsets,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-sni",
			create: proxycfg.TestConfigSnapshotTerminatingGatewaySNI,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-http2-upstream",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayHTTP2,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-http2-upstream-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGatewaySubsetsHTTP2,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-ignore-extra-resolvers",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayIgnoreExtraResolvers,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-lb-config",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayLBConfig,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-lb-config-no-hash-policies",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayLBConfigNoHashPolicies,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "terminating-gateway-custom-trace-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{}
					ns.Proxy.Config["protocol"] = "http"
					ns.Proxy.Config["envoy_listener_tracing_json"] = customTraceJSON(t)
				}, nil)
			},
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "terminating-gateway-with-tls-incoming-min-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										TLSMinVersion: types.TLSv1_3,
									},
								},
							},
						},
					},
				})
			},
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "terminating-gateway-with-tls-incoming-max-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										TLSMaxVersion: types.TLSv1_2,
									},
								},
							},
						},
					},
				})
			},
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "terminating-gateway-with-tls-incoming-cipher-suites",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
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
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "terminating-gateway-custom-and-tagged-addresses",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"envoy_gateway_no_default_bind":       true,
						"envoy_gateway_bind_tagged_addresses": true,
						"envoy_gateway_bind_addresses": map[string]structs.ServiceAddress{
							// This bind address should not get a listener due to deduplication and it sorts to the end
							"z-duplicate-of-tagged-wan-addr": {
								Address: "198.18.0.1",
								Port:    443,
							},
							"foo": {
								Address: "198.17.2.3",
								Port:    8080,
							},
						},
					}
				}, nil)
			},
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "terminating-gateway-no-api-cert",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				api := structs.NewServiceName("api", nil)
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "service-leaf:" + api.String(), // serviceLeafIDPrefix
						Result:        nil,                            // tombstone this
					},
				})
			},
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "terminating-gateway-with-peer-trust-bundle",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				roots, _ := proxycfg.TestCerts(t)
				return proxycfg.TestConfigSnapshotTerminatingGateway(t, true, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "peer-trust-bundle:web",
						Result: &pbpeering.TrustBundleListByServiceResponse{
							Bundles: []*pbpeering.PeeringTrustBundle{
								{
									TrustDomain: "foo.bar.gov",
									PeerName:    "dc2",
									Partition:   "default",
									RootPEMs: []string{
										roots.Roots[0].RootCert,
									},
									ExportedPartition: "default",
									CreateIndex:       0,
									ModifyIndex:       0,
								},
							},
						},
					},
					{
						CorrelationID: "service-intentions:web",
						Result: structs.SimplifiedIntentions{
							{
								SourceName:           "source",
								SourcePeer:           "dc2",
								DestinationName:      "web",
								DestinationPartition: "default",
								Action:               structs.IntentionActionAllow,
							},
						},
					},
				})
			},
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-default-service-subset",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayDefaultServiceSubset,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
	}
}

func getIngressGatewayGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "ingress-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"default", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-gateway-nil-config-entry",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway_NilConfigEntry(t)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-gateway-no-services",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, false, "tcp",
					"default", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"simple", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
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
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-external-sni",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"external-sni", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-and-failover",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-and-failover-to-cluster-peer",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-to-cluster-peer", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tcp-chain-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-remote-gateway", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-remote-gateway-triggered", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tcp-chain-double-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-double-remote-gateway", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-double-remote-gateway-triggered", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tcp-chain-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-local-gateway", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tcp-chain-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-local-gateway-triggered", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tcp-chain-double-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-double-local-gateway", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp",
					"failover-through-double-local-gateway-triggered", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-splitter-with-resolver-redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"splitter-with-resolver-redirect-multidc", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-lb-in-resolver",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"lb-resolver", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-multiple-listeners-duplicate-service",
			create: proxycfg.TestConfigSnapshotIngress_MultipleListenersDuplicateService,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-config-entry-nil",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway_NilConfigEntry(t)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-defaults-no-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, false, "tcp",
					"default", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-and-splitter",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"chain-and-splitter", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-grpc-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"grpc-router", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-and-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http",
					"chain-and-router", nil, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-http-multiple-services",
			create: proxycfg.TestConfigSnapshotIngress_HTTPMultipleServices,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-grpc-multiple-services",
			create: proxycfg.TestConfigSnapshotIngress_GRPCMultipleServices,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-chain-and-router-header-manip",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "router-header-manip", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-sds-listener-level",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "sds-listener-level", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-sds-listener-level-wildcard",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "sds-listener-level-wildcard", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tls-listener",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.TLS.Enabled = true
					}, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tls-listener-min-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.TLS.Enabled = true
						entry.TLS.TLSMinVersion = types.TLSv1_3
					}, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tls-listener-max-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.TLS.Enabled = true
						entry.TLS.TLSMaxVersion = types.TLSv1_2
					}, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-tls-listener-cipher-suites",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", nil,
					func(entry *structs.IngressGatewayConfigEntry) {
						entry.TLS.Enabled = true
						entry.TLS.CipherSuites = []types.TLSCipherSuite{
							types.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
							types.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
						}
					}, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-tls-mixed-listeners",
			create: proxycfg.TestConfigSnapshotIngressGateway_MixedListeners,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-tls-min-version-listeners-gateway-defaults",
			create: proxycfg.TestConfigSnapshotIngressGateway_TLSMinVersionListenersGatewayDefaults,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-single-tls-listener",
			create: proxycfg.TestConfigSnapshotIngressGateway_SingleTLSListener,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-tls-mixed-min-version-listeners",
			create: proxycfg.TestConfigSnapshotIngressGateway_TLSMixedMinVersionListeners,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-tls-mixed-max-version-listeners",
			create: proxycfg.TestConfigSnapshotIngressGateway_TLSMixedMaxVersionListeners,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-tls-mixed-cipher-suites-listeners",
			create: proxycfg.TestConfigSnapshotIngressGateway_TLSMixedCipherVersionListeners,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-sds-listener-gw-level",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_GatewayLevel,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-sds-listener-listener-level",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_GatewayAndListenerLevel,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-sds-listener-gw-level-http",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_GatewayAndListenerLevel_HTTP,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-sds-listener-gw-level-mixed-tls",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_GatewayLevel_MixedTLS,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		// TODO: cross reference with ingress-with-sds-service-level and figure out which should stay
		{
			name: "ingress-with-sds-service-level-2",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "sds-service-level", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-with-sds-service-level-mixed-tls",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGatewayWithChain(t, "sds-service-level-mixed-tls", nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-sds-service-level",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_ServiceLevel,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-sds-listener+service-level",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_ListenerAndServiceLevel,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-sds-service-level-mixed-no-tls",
			create: proxycfg.TestConfigSnapshotIngressGatewaySDS_MixedNoTLS,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-grpc-single-tls-listener",
			create: proxycfg.TestConfigSnapshotIngressGateway_SingleTLSListener_GRPC,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-http2-single-tls-listener",
			create: proxycfg.TestConfigSnapshotIngressGateway_SingleTLSListener_HTTP2,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-with-http2-and-grpc-multiple-tls-listener",
			create: proxycfg.TestConfigSnapshotIngressGateway_GWTLSListener_MixedHTTP2gRPC,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name: "ingress-gateway-bind-addrs",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "tcp", "default", func(ns *structs.NodeService) {
					//
					ns.TaggedAddresses = map[string]structs.ServiceAddress{
						"lan": {Address: "10.0.0.1"},
						"wan": {Address: "172.16.0.1"},
					}
					ns.Proxy.Config = map[string]interface{}{
						"envoy_gateway_no_default_bind":       true,
						"envoy_gateway_bind_tagged_addresses": true,
						"envoy_gateway_bind_addresses": map[string]structs.ServiceAddress{
							"foo": {Address: "8.8.8.8"},
						},
					}
				}, nil, nil)
			},
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
	}
}

func getAccessLogsGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "access-logs-defaults",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					// This should be passed into the snapshot through proxy-defaults
					ns.Proxy.AccessLogs = structs.AccessLogsConfig{
						Enabled: true,
					}
				},
					nil)
			},
		},
		{
			name: "access-logs-json-file",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					// This should be passed into the snapshot through proxy-defaults
					ns.Proxy.AccessLogs = structs.AccessLogsConfig{
						Enabled:    true,
						Type:       structs.FileLogSinkType,
						Path:       "/tmp/accesslog.txt",
						JSONFormat: "{ \"custom_start_time\": \"%START_TIME%\" }",
					}
				},
					nil)
			},
		},
		{
			name: "access-logs-text-stderr-disablelistenerlogs",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					// This should be passed into the snapshot through proxy-defaults
					ns.Proxy.AccessLogs = structs.AccessLogsConfig{
						Enabled:             true,
						DisableListenerLogs: true,
						Type:                structs.StdErrLogSinkType,
						TextFormat:          "CUSTOM FORMAT %START_TIME%",
					}
				},
					nil)
			},
		},
	}
}

func getTLSGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
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
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
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
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tls-incoming-min-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										TLSMinVersion: types.TLSv1_3,
									},
								},
							},
						},
					},
				})
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tls-incoming-max-version",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
										TLSMaxVersion: types.TLSv1_2,
									},
								},
							},
						},
					},
				})
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tls-incoming-cipher-suites",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, []proxycfg.UpdateEvent{
					{
						CorrelationID: "mesh",
						Result: &structs.ConfigEntryResponse{
							Entry: &structs.MeshConfigEntry{
								TLS: &structs.MeshTLSConfig{
									Incoming: &structs.MeshDirectionalTLSConfig{
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
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tproxy-and-permissive-mtls",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.MutualTLSMode = structs.MutualTLSModePermissive
					ns.Proxy.Mode = structs.ProxyModeTransparent
					ns.Proxy.TransparentProxy.OutboundListenerPort = 1234
				},
					nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-without-tproxy-and-permissive-mtls",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, func(ns *structs.NodeService) {
					ns.Proxy.MutualTLSMode = structs.MutualTLSModePermissive
				},
					nil)
			},
			alsoRunTestForV2: true,
		},
	}
}

func getPeeredGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name:   "connect-proxy-with-peered-upstreams",
			create: proxycfg.TestConfigSnapshotPeering,
			// TODO(proxystate): peering will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name:   "connect-proxy-with-peered-upstreams-escape-overrides",
			create: proxycfg.TestConfigSnapshotPeeringWithEscapeOverrides,
			// TODO(proxystate): peering will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-peered-upstreams-http2",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeringWithHTTP2(t, nil)
			},
			// TODO(proxystate): peering will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name:   "transparent-proxy-with-peered-upstreams",
			create: proxycfg.TestConfigSnapshotPeeringTProxy,
			// TODO(proxystate): peering will come at a later date.
			alsoRunTestForV2: false,
		},
		{
			name:   "local-mesh-gateway-with-peered-upstreams",
			create: proxycfg.TestConfigSnapshotPeeringLocalMeshGateway,
			// TODO(proxystate): mesh gateways and peering will come at a later date.
			alsoRunTestForV2: false,
		},
	}
}

func getXDSFetchTimeoutTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "xds-fetch-timeout-ms-sidecar",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				// Covers cases in:
				// - clusters.go:makeUpstreamClustersForDiscoveryChain
				// - clusters.go:makeUpstreamClusterForPreparedQuery
				// - listeners.go:listenersFromSnapshotConnectProxy (partially)
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "chain-and-router", false, func(ns *structs.NodeService) {
					ns.Proxy.Config["xds_fetch_timeout_ms"] = 9999
				}, nil)
			},
			alsoRunTestForV2: false,
		},
		{
			name: "xds-fetch-timeout-ms-tproxy-http-peering",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				// Covers cases in:
				// - clusters.go:makeUpstreamClusterForPeerService
				snap := proxycfg.TestConfigSnapshotPeeringWithHTTP2(t, func(ns *structs.NodeService) {
					ns.Proxy.Config["xds_fetch_timeout_ms"] = 9999
				})
				return snap
			},
			alsoRunTestForV2: false,
		},
		{
			name: "xds-fetch-timeout-ms-tproxy-passthrough",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				// Covers cases in:
				// - clusters.go:makePassthrough
				// - listeners.go:listenersFromSnapshotConnectProxy (partially)
				return proxycfg.TestConfigSnapshotTransparentProxyDestinationHTTP(t, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"xds_fetch_timeout_ms": 9999,
					}
				})
			},
			alsoRunTestForV2: false,
		},
		{
			name: "xds-fetch-timeout-ms-term-gw",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				// Covers cases in:
				// - listeners.go:makeFilterChainTerminatingGateway
				// - clusters.go:makeGatewayCluster
				return proxycfg.TestConfigSnapshotTerminatingGatewayServiceSubsetsWebAndCache(t, func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"xds_fetch_timeout_ms": 9999,
					}
				})
			},
			alsoRunTestForV2: false,
		},
		{
			name: "xds-fetch-timeout-ms-mgw-peering",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				// Covers cases in:
				// - listeners.go:makeMeshGatewayPeerFilterChain
				// - clusters.go:makeGatewayCluster
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "default-services-http", func(ns *structs.NodeService) {
					ns.Proxy.Config["xds_fetch_timeout_ms"] = 9999
				}, nil)
			},
			alsoRunTestForV2: false,
		},
		{
			name: "xds-fetch-timeout-ms-ingress-with-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				// Covers cases in:
				// - listeners.go:makeIngressGatewayListeners (partially)
				return proxycfg.TestConfigSnapshotIngressGateway(t, true, "http", "chain-and-router", func(ns *structs.NodeService) {
					ns.Proxy.Config = map[string]interface{}{
						"xds_fetch_timeout_ms": 9999,
					}
				}, nil, nil)
			},
			alsoRunTestForV2: false,
		},
	}
}
