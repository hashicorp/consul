package xds

import (
	"path/filepath"
	"sort"
	"testing"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"

	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/sdk/testutil"
)

var testTypeUrlToPrettyName = map[string]string{
	xdscommon.ListenerType: "listeners",
	xdscommon.RouteType:    "routes",
	xdscommon.ClusterType:  "clusters",
	xdscommon.EndpointType: "endpoints",
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
}

func TestAllResourcesFromSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	type testcase = goldenTestCase

	run := func(
		t *testing.T,
		sf supportedProxyFeatures,
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
		// golder files for every test case and so not be any use!
		setupTLSRootsAndLeaf(t, snap)

		if tt.setup != nil {
			tt.setup(snap)
		}

		// Need server just for logger dependency
		g := newResourceGenerator(testutil.Logger(t), nil, false)
		g.ProxyFeatures = sf
		if tt.generatorSetup != nil {
			tt.generatorSetup(g)
		}

		resources, err := g.allResourcesFromSnapshot(snap)
		require.NoError(t, err)

		typeUrls := []string{
			xdscommon.ListenerType,
			xdscommon.RouteType,
			xdscommon.ClusterType,
			xdscommon.EndpointType,
		}
		require.Len(t, resources, len(typeUrls))

		for _, typeUrl := range typeUrls {
			prettyName := testTypeUrlToPrettyName[typeUrl]
			t.Run(prettyName, func(t *testing.T) {
				items, ok := resources[typeUrl]
				require.True(t, ok)

				sort.Slice(items, func(i, j int) bool {
					switch typeUrl {
					case xdscommon.ListenerType:
						return items[i].(*envoy_listener_v3.Listener).Name < items[j].(*envoy_listener_v3.Listener).Name
					case xdscommon.RouteType:
						return items[i].(*envoy_route_v3.RouteConfiguration).Name < items[j].(*envoy_route_v3.RouteConfiguration).Name
					case xdscommon.ClusterType:
						return items[i].(*envoy_cluster_v3.Cluster).Name < items[j].(*envoy_cluster_v3.Cluster).Name
					case xdscommon.EndpointType:
						return items[i].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName < items[j].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName
					default:
						panic("not possible")
					}
				})

				r, err := createResponse(typeUrl, "00000001", "00000001", items)
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

	tests := []testcase{
		{
			name: "defaults",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshot(t, nil, nil, false)
			},
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
				}, true)
			},
		},
		{
			name:   "transparent-proxy",
			create: proxycfg.TestConfigSnapshotTransparentProxy,
		},
		{
			name:   "connect-proxy-with-peered-upstreams",
			create: proxycfg.TestConfigSnapshotPeering,
		},
		{
			name:   "transparent-proxy-with-peered-upstreams",
			create: proxycfg.TestConfigSnapshotPeeringTProxy,
		},
	}
	tests = append(tests, getConnectProxyTransparentProxyGoldenTestCases()...)
	tests = append(tests, getMeshGatewayPeeringGoldenTestCases()...)
	tests = append(tests, getEnterpriseGoldenTestCases()...)

	latestEnvoyVersion := proxysupport.EnvoyVersions[0]
	for _, envoyVersion := range proxysupport.EnvoyVersions {
		sf, err := determineSupportedProxyFeaturesFromString(envoyVersion)
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
			name:   "transparent-proxy-destination",
			create: proxycfg.TestConfigSnapshotTransparentProxyDestination,
		},
		{
			name:   "transparent-proxy-destination-http",
			create: proxycfg.TestConfigSnapshotTransparentProxyDestinationHTTP,
		},
		{
			name: "transparent-proxy-terminating-gateway-destinations-only",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotTerminatingGatewayDestinations(t, true, nil)
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
		},
		{
			name: "mesh-gateway-with-exported-peered-services-http",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "default-services-http", nil, nil)
			},
		},
		{
			name: "mesh-gateway-with-exported-peered-services-http-with-router",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotPeeredMeshGateway(t, "chain-and-l7-stuff", nil, nil)
			},
		},
	}
}
