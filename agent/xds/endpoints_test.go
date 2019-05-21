package xds

import (
	"testing"

	"github.com/mitchellh/copystructure"

	"github.com/stretchr/testify/require"

	envoy "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoyendpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/hashicorp/consul/agent/structs"
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

	tests := []struct {
		name        string
		clusterName string
		endpoints   structs.CheckServiceNodes
		want        *envoy.ClusterLoadAssignment
	}{
		{
			name:        "no instances",
			clusterName: "service:test",
			endpoints:   structs.CheckServiceNodes{},
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
			endpoints:   testCheckServiceNodes,
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
			endpoints:   testWeightedCheckServiceNodes,
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
			endpoints:   testWarningCheckServiceNodes,
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
			got := makeLoadAssignment(tt.clusterName, tt.endpoints)
			require.Equal(t, tt.want, got)
		})
	}
}
