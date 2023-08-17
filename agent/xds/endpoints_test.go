// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"path/filepath"
	"sort"
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/proxystateconverter"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/agent/xds/testcommon"
	"github.com/hashicorp/consul/agent/xdsv2"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/copystructure"
	testinf "github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"
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
		locality    *structs.Locality
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
									Address: response.MakeAddress("10.10.10.10", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: response.MakeUint32Value(1),
						},
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: response.MakeAddress("10.10.10.20", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: response.MakeUint32Value(1),
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
									Address: response.MakeAddress("10.10.10.10", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: response.MakeUint32Value(10),
						},
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: response.MakeAddress("10.10.10.20", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: response.MakeUint32Value(5),
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
									Address: response.MakeAddress("10.10.10.10", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_HEALTHY,
							LoadBalancingWeight: response.MakeUint32Value(1),
						},
						{
							HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
								Endpoint: &envoy_endpoint_v3.Endpoint{
									Address: response.MakeAddress("10.10.10.20", 1234),
								}},
							HealthStatus:        envoy_core_v3.HealthStatus_UNHEALTHY,
							LoadBalancingWeight: response.MakeUint32Value(1),
						},
					},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeLoadAssignment(
				hclog.NewNullLogger(),
				&proxycfg.ConfigSnapshot{ServiceLocality: tt.locality},
				tt.clusterName,
				nil,
				tt.endpoints,
				proxycfg.GatewayKey{Datacenter: "dc1"},
			)
			require.Equal(t, tt.want, got)

			if tt.locality == nil {
				got := makeLoadAssignment(
					hclog.NewNullLogger(),
					&proxycfg.ConfigSnapshot{ServiceLocality: &structs.Locality{Region: "us-west-1", Zone: "us-west-1a"}},
					tt.clusterName,
					nil,
					tt.endpoints,
					proxycfg.GatewayKey{Datacenter: "dc1"},
				)
				require.Equal(t, tt.want, got)
			}
		})
	}
}

type endpointTestCase struct {
	name               string
	create             func(t testinf.T) *proxycfg.ConfigSnapshot
	overrideGoldenName string
	alsoRunTestForV2   bool
}

func makeEndpointDiscoChainTests(enterprise bool) []endpointTestCase {
	return []endpointTestCase{
		{
			name: "connect-proxy-with-chain",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-external-sni",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "external-sni", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-chain-and-overrides",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "simple-with-overrides", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
		{
			name: "connect-proxy-with-chain-and-failover",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-remote-gateway-triggered", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-remote-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-remote-gateway-triggered", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tcp-chain-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-local-gateway-triggered", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-local-gateway",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway", enterprise, nil, nil)
			},
			alsoRunTestForV2: true,
		},
		{
			name: "connect-proxy-with-tcp-chain-double-failover-through-local-gateway-triggered",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "failover-through-double-local-gateway-triggered", enterprise, nil, nil)
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
			name: "splitter-with-resolver-redirect",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotDiscoveryChain(t, "splitter-with-resolver-redirect-multidc", enterprise, nil, nil)
			},
			// TODO(proxystate): requires routes work
			alsoRunTestForV2: false,
		},
	}
}

func TestEndpointsFromSnapshot(t *testing.T) {
	// TODO: we should move all of these to TestAllResourcesFromSnapshot
	// eventually to test all of the xDS types at once with the same input,
	// just as it would be triggered by our xDS server.
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	tests := []endpointTestCase{
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
			name: "mesh-gateway-newer-information-in-federation-states",
			create: func(t testinf.T) *proxycfg.ConfigSnapshot {
				return proxycfg.TestConfigSnapshotMeshGateway(t, "newer-info-in-federation-states", nil, nil)
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
			name:   "terminating-gateway-service-subsets",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayServiceSubsets,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "terminating-gateway-default-service-subset",
			create: proxycfg.TestConfigSnapshotTerminatingGatewayDefaultServiceSubset,
			// TODO(proxystate): terminating gateway will come at a later time
			alsoRunTestForV2: false,
		},
		{
			name:   "ingress-multiple-listeners-duplicate-service",
			create: proxycfg.TestConfigSnapshotIngress_MultipleListenersDuplicateService,
			// TODO(proxystate): ingress gateway will come at a later time
			alsoRunTestForV2: false,
		},
	}

	tests = append(tests, makeEndpointDiscoChainTests(false)...)

	latestEnvoyVersion := xdscommon.EnvoyVersions[0]
	for _, envoyVersion := range xdscommon.EnvoyVersions {
		sf, err := xdscommon.DetermineSupportedProxyFeaturesFromString(envoyVersion)
		require.NoError(t, err)
		t.Run("envoy-"+envoyVersion, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Sanity check default with no overrides first
					snap := tt.create(t)

					// We need to replace the TLS certs with deterministic ones to make golden
					// files workable. Note we don't update these otherwise they'd change
					// golden files for every test case and so not be any use!
					testcommon.SetupTLSRootsAndLeaf(t, snap)

					// Need server just for logger dependency
					g := NewResourceGenerator(testutil.Logger(t), nil, false)
					g.ProxyFeatures = sf

					endpoints, err := g.endpointsFromSnapshot(snap)
					require.NoError(t, err)

					sort.Slice(endpoints, func(i, j int) bool {
						return endpoints[i].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName < endpoints[j].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName
					})
					r, err := response.CreateResponse(xdscommon.EndpointType, "00000001", "00000001", endpoints)
					require.NoError(t, err)

					t.Run("current-xdsv1", func(t *testing.T) {
						gotJSON := protoToJSON(t, r)

						gName := tt.name
						if tt.overrideGoldenName != "" {
							gName = tt.overrideGoldenName
						}

						require.JSONEq(t, goldenEnvoy(t, filepath.Join("endpoints", gName), envoyVersion, latestEnvoyVersion, gotJSON), gotJSON)
					})

					if tt.alsoRunTestForV2 {
						generator := xdsv2.NewResourceGenerator(testutil.Logger(t))

						converter := proxystateconverter.NewConverter(testutil.Logger(t), &mockCfgFetcher{addressLan: "10.10.10.10"})
						proxyState, err := converter.ProxyStateFromSnapshot(snap)
						require.NoError(t, err)

						res, err := generator.AllResourcesFromIR(proxyState)
						require.NoError(t, err)

						endpoints = res[xdscommon.EndpointType]
						// The order of listeners returned via LDS isn't relevant, so it's safe
						// to sort these for the purposes of test comparisons.
						sort.Slice(endpoints, func(i, j int) bool {
							return endpoints[i].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName < endpoints[j].(*envoy_endpoint_v3.ClusterLoadAssignment).ClusterName
						})

						r, err := response.CreateResponse(xdscommon.EndpointType, "00000001", "00000001", endpoints)
						require.NoError(t, err)

						t.Run("current-xdsv2", func(t *testing.T) {
							gotJSON := protoToJSON(t, r)

							gName := tt.name
							if tt.overrideGoldenName != "" {
								gName = tt.overrideGoldenName
							}

							expectedJSON := goldenEnvoy(t, filepath.Join("endpoints", gName), envoyVersion, latestEnvoyVersion, gotJSON)
							require.JSONEq(t, expectedJSON, gotJSON)
						})
					}
				})
			}
		})
	}
}
