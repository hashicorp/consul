package xds

import (
	"path/filepath"
	"sort"
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"

	"github.com/mitchellh/copystructure"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/proxysupport"
	"github.com/hashicorp/consul/sdk/testutil"
)

func Test_makeLoadAssignment(t *testing.T) {

	testCheckServiceNodes := structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "node1-id",
				Node:       "node1",
				Address:    "10.10.10.10",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Service: "web",
				Port:    1234,
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "node1",
					CheckID: "serfHealth",
					Status:  "passing",
				},
				&structs.HealthCheck{
					Node:      "node1",
					ServiceID: "web",
					CheckID:   "web:check",
					Status:    "passing",
				},
			},
		},
		structs.CheckServiceNode{
			Node: &structs.Node{
				ID:         "node2-id",
				Node:       "node2",
				Address:    "10.10.10.20",
				Datacenter: "dc1",
			},
			Service: &structs.NodeService{
				Service: "web",
				Port:    1234,
			},
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Node:    "node2",
					CheckID: "serfHealth",
					Status:  "passing",
				},
				&structs.HealthCheck{
					Node:      "node2",
					ServiceID: "web",
					CheckID:   "web:check",
					Status:    "passing",
				},
			},
		},
	}

	testWeightedCheckServiceNodesRaw, err := copystructure.Copy(testCheckServiceNodes)
	require.NoError(t, err)
	testWeightedCheckServiceNodes := testWeightedCheckServiceNodesRaw.(structs.CheckServiceNodes)

	testWeightedCheckServiceNodes[0].Service.Weights = &structs.Weights{
		Passing: 10,
		Warning: 1,
	}
	testWeightedCheckServiceNodes[1].Service.Weights = &structs.Weights{
		Passing: 5,
		Warning: 0,
	}

	testWarningCheckServiceNodesRaw, err := copystructure.Copy(testWeightedCheckServiceNodes)
	require.NoError(t, err)
	testWarningCheckServiceNodes := testWarningCheckServiceNodesRaw.(structs.CheckServiceNodes)

	testWarningCheckServiceNodes[0].Checks[0].Status = "warning"
	testWarningCheckServiceNodes[1].Checks[0].Status = "warning"

	// TODO(rb): test onlypassing
	tests := []struct {
		name        string
		clusterName string
		endpoints   []loadAssignmentEndpointGroup
		want        *envoy_endpoint_v3.ClusterLoadAssignment
	}{
		{
			name:        "no instances",
			clusterName: "service:test",
			endpoints: []loadAssignmentEndpointGroup{
				{Endpoints: nil},
			},
			want: &envoy_endpoint_v3.ClusterLoadAssignment{
				ClusterName: "service:test",
				Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{},
				}},
			},
		},
		{
			name:        "instances, no weights",
			clusterName: "service:test",
			endpoints: []loadAssignmentEndpointGroup{
				{Endpoints: testCheckServiceNodes},
			},
			want: &envoy_endpoint_v3.ClusterLoadAssignment{
				ClusterName: "service:test",
				Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: makeAddress("10.10.10.10", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: makeUint32Value(1),
						},
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: makeAddress("10.10.10.20", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: makeUint32Value(1),
						},
					},
				}},
			},
		},
		{
			name:        "instances, healthy weights",
			clusterName: "service:test",
			endpoints: []loadAssignmentEndpointGroup{
				{Endpoints: testWeightedCheckServiceNodes},
			},
			want: &envoy_endpoint_v3.ClusterLoadAssignment{
				ClusterName: "service:test",
				Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: makeAddress("10.10.10.10", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: makeUint32Value(10),
						},
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: makeAddress("10.10.10.20", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: makeUint32Value(5),
						},
					},
				}},
			},
		},
		{
			name:        "instances, warning weights",
			clusterName: "service:test",
			endpoints: []loadAssignmentEndpointGroup{
				{Endpoints: testWarningCheckServiceNodes},
			},
			want: &envoy_endpoint_v3.ClusterLoadAssignment{
				ClusterName: "service:test",
				Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: makeAddress("10.10.10.10", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: makeUint32Value(1),
						},
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: makeAddress("10.10.10.20", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_UNHEALTHY,
							LoadBalancingWeight: makeUint32Value(1),
						},
					},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeLoadAssignment(
				tt.clusterName,
				tt.endpoints,
				proxycfg.GatewayKey{Datacenter: "dc1"},
			)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEndpointsFromSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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
			name:   "mesh-gateway",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-using-federation-states",
			create: proxycfg.TestConfigSnapshotMeshGatewayUsingFederationStates,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-newer-information-in-federation-states",
			create: proxycfg.TestConfigSnapshotMeshGatewayNewerInformationInFederationStates,
		},
		{
			name:   "mesh-gateway-older-information-in-federation-states",
			create: proxycfg.TestConfigSnapshotMeshGatewayOlderInformationInFederationStates,
		},
		{
			name:   "mesh-gateway-no-services",
			create: proxycfg.TestConfigSnapshotMeshGatewayNoServices,
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
			name:   "connect-proxy-with-default-chain-and-custom-cluster",
			create: proxycfg.TestConfigSnapshotDiscoveryChainDefault,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.Proxy.Upstreams[0].Config["envoy_cluster_json"] =
					customAppClusterJSON(t, customClusterJSONOptions{
						Name: "myservice",
					})
				snap.ConnectProxy.UpstreamConfig = map[string]*structs.Upstream{
					"db": {
						Config: map[string]interface{}{
							"envoy_cluster_json": customAppClusterJSON(t, customClusterJSONOptions{
								Name: "myservice",
							}),
						},
					},
				}
			},
		},
		{
			name:   "splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-service-subsets",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("foo", nil): {
						Kind: structs.ServiceResolver,
						Name: "foo",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "mesh-gateway-default-service-subset",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("bar", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "bar",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("foo", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "foo",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "ingress-gateway",
			create: proxycfg.TestConfigSnapshotIngressGateway,
			setup:  nil,
		},
		{
			name:   "ingress-gateway-no-services",
			create: proxycfg.TestConfigSnapshotIngressGatewayNoServices,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain",
			create: proxycfg.TestConfigSnapshotIngress,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-external-sni",
			create: proxycfg.TestConfigSnapshotIngressExternalSNI,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-overrides",
			create: proxycfg.TestConfigSnapshotIngressWithOverrides,
			setup:  nil,
		},
		{
			name:   "ingress-with-chain-and-failover",
			create: proxycfg.TestConfigSnapshotIngressWithFailover,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-remote-gateway",
			create: proxycfg.TestConfigSnapshotIngressWithFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: proxycfg.TestConfigSnapshotIngressWithFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-remote-gateway",
			create: proxycfg.TestConfigSnapshotIngressWithDoubleFailoverThroughRemoteGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: proxycfg.TestConfigSnapshotIngressWithDoubleFailoverThroughRemoteGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-local-gateway",
			create: proxycfg.TestConfigSnapshotIngressWithFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-failover-through-local-gateway-triggered",
			create: proxycfg.TestConfigSnapshotIngressWithFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-local-gateway",
			create: proxycfg.TestConfigSnapshotIngressWithDoubleFailoverThroughLocalGateway,
			setup:  nil,
		},
		{
			name:   "ingress-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: proxycfg.TestConfigSnapshotIngressWithDoubleFailoverThroughLocalGatewayTriggered,
			setup:  nil,
		},
		{
			name:   "ingress-splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotIngress_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "terminating-gateway",
			create: proxycfg.TestConfigSnapshotTerminatingGateway,
			setup:  nil,
		},
		{
			name:   "terminating-gateway-no-services",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayNoServices,
			setup:  nil,
		},
		{
			name:   "terminating-gateway-service-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("web", nil): {
						Kind: structs.ServiceResolver,
						Name: "web",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("web", nil): {
						Kind: structs.ServiceResolver,
						Name: "web",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "terminating-gateway-default-service-subset",
			create: proxycfg.TestConfigSnapshotTerminatingGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.TerminatingGateway.ServiceResolvers = map[structs.ServiceName]*structs.ServiceResolverConfigEntry{
					structs.NewServiceName("web", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "web",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceName("web", nil): {
						Kind:          structs.ServiceResolver,
						Name:          "web",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": {
								Filter: "Service.Meta.version == 1",
							},
							"v2": {
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
				}
			},
		},
		{
			name:   "ingress-multiple-listeners-duplicate-service",
			create: proxycfg.TestConfigSnapshotIngress_MultipleListenersDuplicateService,
			setup:  nil,
		},
	}

	latestEnvoyVersion := proxysupport.EnvoyVersions[0]
	for _, envoyVersion := range proxysupport.EnvoyVersions {
		sf, err := determineSupportedProxyFeaturesFromString(envoyVersion)
		require.NoError(t, err)
		t.Run("envoy-"+envoyVersion, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Sanity check default with no overrides first
					snap := tt.create(t)

					// We need to replace the TLS certs with deterministic ones to make golden
					// files workable. Note we don't update these otherwise they'd change
					// golden files for every test case and so not be any use!
					setupTLSRootsAndLeaf(t, snap)

					if tt.setup != nil {
						tt.setup(snap)
					}

					// Need server just for logger dependency
					g := newResourceGenerator(testutil.Logger(t), nil, nil, false)
					g.ProxyFeatures = sf

					endpoints, err := g.endpointsFromSnapshot(snap)
					require.NoError(t, err)

					sort.Slice(endpoints, func(i, j int) bool {
						return endpoints[i].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName < endpoints[j].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName
					})
					r, err := createResponse(EndpointType, "00000001", "00000001", endpoints)
					require.NoError(t, err)

					t.Run("current", func(t *testing.T) {
						gotJSON := protoToJSON(t, r)

						gName := tt.name
						if tt.overrideGoldenName != "" {
							gName = tt.overrideGoldenName
						}

						require.JSONEq(t, goldenEnvoy(t, filepath.Join("endpoints", gName), envoyVersion, latestEnvoyVersion, gotJSON), gotJSON)
					})
				})
			}
		})
	}
}
