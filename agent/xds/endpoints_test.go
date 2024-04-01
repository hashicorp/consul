// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"testing"

	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/copystructure"
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
