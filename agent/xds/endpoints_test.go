package xds

import (
	"path"
	"sort"
	"testing"

	"github.com/mitchellh/copystructure"

	"github.com/stretchr/testify/require"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoyendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	testinf "github.com/mitchellh/go-testing-interface"
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
		want        *envoy.ClusterLoadAssignment
	}{
		{
			name:        "no instances",
			clusterName: "service:test",
			endpoints: []loadAssignmentEndpointGroup{
				{Endpoints: nil},
			},
			want: &envoy.ClusterLoadAssignment{
				ClusterName: "service:test",
				Endpoints: []envoyendpoint.LocalityLbEndpoints{{
					LbEndpoints: []envoyendpoint.LbEndpoint{},
				}},
			},
		},
		{
			name:        "instances, no weights",
			clusterName: "service:test",
			endpoints: []loadAssignmentEndpointGroup{
				{Endpoints: testCheckServiceNodes},
			},
			want: &envoy.ClusterLoadAssignment{
				ClusterName: "service:test",
				Endpoints: []envoyendpoint.LocalityLbEndpoints{{
					LbEndpoints: []envoyendpoint.LbEndpoint{
						envoyendpoint.LbEndpoint{
							HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
								Endpoint: &envoyendpoint.Endpoint{
									Address: makeAddressPtr("10.10.10.10", 1234),
								}},
							HealthStatus:        core.HealthStatus_HEALTHY,
							LoadBalancingWeight: makeUint32Value(1),
						},
						envoyendpoint.LbEndpoint{
							HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
								Endpoint: &envoyendpoint.Endpoint{
									Address: makeAddressPtr("10.10.10.20", 1234),
								}},
							HealthStatus:        core.HealthStatus_HEALTHY,
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
			want: &envoy.ClusterLoadAssignment{
				ClusterName: "service:test",
				Endpoints: []envoyendpoint.LocalityLbEndpoints{{
					LbEndpoints: []envoyendpoint.LbEndpoint{
						envoyendpoint.LbEndpoint{
							HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
								Endpoint: &envoyendpoint.Endpoint{
									Address: makeAddressPtr("10.10.10.10", 1234),
								}},
							HealthStatus:        core.HealthStatus_HEALTHY,
							LoadBalancingWeight: makeUint32Value(10),
						},
						envoyendpoint.LbEndpoint{
							HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
								Endpoint: &envoyendpoint.Endpoint{
									Address: makeAddressPtr("10.10.10.20", 1234),
								}},
							HealthStatus:        core.HealthStatus_HEALTHY,
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
			want: &envoy.ClusterLoadAssignment{
				ClusterName: "service:test",
				Endpoints: []envoyendpoint.LocalityLbEndpoints{{
					LbEndpoints: []envoyendpoint.LbEndpoint{
						envoyendpoint.LbEndpoint{
							HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
								Endpoint: &envoyendpoint.Endpoint{
									Address: makeAddressPtr("10.10.10.10", 1234),
								}},
							HealthStatus:        core.HealthStatus_HEALTHY,
							LoadBalancingWeight: makeUint32Value(1),
						},
						envoyendpoint.LbEndpoint{
							HostIdentifier: &envoyendpoint.LbEndpoint_Endpoint{
								Endpoint: &envoyendpoint.Endpoint{
									Address: makeAddressPtr("10.10.10.20", 1234),
								}},
							HealthStatus:        core.HealthStatus_UNHEALTHY,
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
				"dc1",
			)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_endpointsFromSnapshot(t *testing.T) {

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
			name:   "splitter-with-resolver-redirect",
			create: proxycfg.TestConfigSnapshotDiscoveryChain_SplitterWithResolverRedirectMultiDC,
			setup:  nil,
		},
		{
			name:   "mesh-gateway-service-subsets",
			create: proxycfg.TestConfigSnapshotMeshGateway,
			setup: func(snap *proxycfg.ConfigSnapshot) {
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceID]*structs.ServiceResolverConfigEntry{
					structs.NewServiceID("bar", nil): &structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "bar",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": structs.ServiceResolverSubset{
								Filter: "Service.Meta.version == 1",
							},
							"v2": structs.ServiceResolverSubset{
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceID("foo", nil): &structs.ServiceResolverConfigEntry{
						Kind: structs.ServiceResolver,
						Name: "foo",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": structs.ServiceResolverSubset{
								Filter: "Service.Meta.version == 1",
							},
							"v2": structs.ServiceResolverSubset{
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
				snap.MeshGateway.ServiceResolvers = map[structs.ServiceID]*structs.ServiceResolverConfigEntry{
					structs.NewServiceID("bar", nil): &structs.ServiceResolverConfigEntry{
						Kind:          structs.ServiceResolver,
						Name:          "bar",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": structs.ServiceResolverSubset{
								Filter: "Service.Meta.version == 1",
							},
							"v2": structs.ServiceResolverSubset{
								Filter:      "Service.Meta.version == 2",
								OnlyPassing: true,
							},
						},
					},
					structs.NewServiceID("foo", nil): &structs.ServiceResolverConfigEntry{
						Kind:          structs.ServiceResolver,
						Name:          "foo",
						DefaultSubset: "v2",
						Subsets: map[string]structs.ServiceResolverSubset{
							"v1": structs.ServiceResolverSubset{
								Filter: "Service.Meta.version == 1",
							},
							"v2": structs.ServiceResolverSubset{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

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
			logger := testutil.Logger(t)
			s := Server{
				Logger: logger,
			}

			endpoints, err := s.endpointsFromSnapshot(snap, "my-token")
			sort.Slice(endpoints, func(i, j int) bool {
				return endpoints[i].(*envoy.ClusterLoadAssignment).ClusterName < endpoints[j].(*envoy.ClusterLoadAssignment).ClusterName
			})
			require.NoError(err)
			r, err := createResponse(EndpointType, "00000001", "00000001", endpoints)
			require.NoError(err)

			gotJSON := responseToJSON(t, r)

			gName := tt.name
			if tt.overrideGoldenName != "" {
				gName = tt.overrideGoldenName
			}

			require.JSONEq(golden(t, path.Join("endpoints", gName), gotJSON), gotJSON)
		})
	}
}
