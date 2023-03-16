package sprawltest_test

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul-topology/sprawl/sprawltest"
	"github.com/hashicorp/consul-topology/topology"
)

func TestSprawl(t *testing.T) {
	cfg := &topology.Config{
		Networks: []*topology.Network{
			{Name: "dc1"},
			{Name: "dc2"},
			{Name: "wan", Type: "wan"},
		},
		Clusters: []*topology.Cluster{
			{
				Name: "dc1",
				Nodes: []*topology.Node{
					{
						Kind: topology.NodeKindServer,
						Name: "dc1-server1",
						Addresses: []*topology.Address{
							{Network: "dc1"},
							{Network: "wan"},
						},
					},
					{
						Kind: topology.NodeKindClient,
						Name: "dc1-client1",
						Services: []*topology.Service{
							{
								ID:             topology.ServiceID{Name: "mesh-gateway"},
								Port:           8443,
								EnvoyAdminPort: 19000,
								IsMeshGateway:  true,
							},
						},
					},
					{
						Kind: topology.NodeKindClient,
						Name: "dc1-client2",
						Services: []*topology.Service{
							{
								ID:             topology.ServiceID{Name: "ping"},
								Image:          "rboyer/pingpong:latest",
								Port:           8080,
								EnvoyAdminPort: 19000,
								Command: []string{
									"-bind", "0.0.0.0:8080",
									"-dial", "127.0.0.1:9090",
									"-pong-chaos",
									"-dialfreq", "250ms",
									"-name", "ping",
								},
								Upstreams: []*topology.Upstream{{
									ID:        topology.ServiceID{Name: "pong"},
									LocalPort: 9090,
									Peer:      "peer-dc2-default",
								}},
							},
						},
					},
				},
				InitialConfigEntries: []api.ConfigEntry{
					&api.ExportedServicesConfigEntry{
						Name: "default",
						Services: []api.ExportedService{{
							Name: "ping",
							Consumers: []api.ServiceConsumer{{
								Peer: "peer-dc2-default",
							}},
						}},
					},
				},
			},
			{
				Name: "dc2",
				Nodes: []*topology.Node{
					{
						Kind: topology.NodeKindServer,
						Name: "dc2-server1",
						Addresses: []*topology.Address{
							{Network: "dc2"},
							{Network: "wan"},
						},
					},
					{
						Kind: topology.NodeKindClient,
						Name: "dc2-client1",
						Services: []*topology.Service{
							{
								ID:             topology.ServiceID{Name: "mesh-gateway"},
								Port:           8443,
								EnvoyAdminPort: 19000,
								IsMeshGateway:  true,
							},
						},
					},
					{
						Kind: topology.NodeKindDataplane,
						Name: "dc2-client2",
						Services: []*topology.Service{
							{
								ID:             topology.ServiceID{Name: "pong"},
								Image:          "rboyer/pingpong:latest",
								Port:           8080,
								EnvoyAdminPort: 19000,
								Command: []string{
									"-bind", "0.0.0.0:8080",
									"-dial", "127.0.0.1:9090",
									"-pong-chaos",
									"-dialfreq", "250ms",
									"-name", "pong",
								},
								Upstreams: []*topology.Upstream{{
									ID:        topology.ServiceID{Name: "ping"},
									LocalPort: 9090,
									Peer:      "peer-dc1-default",
								}},
							},
						},
					},
				},
				InitialConfigEntries: []api.ConfigEntry{
					&api.ExportedServicesConfigEntry{
						Name: "default",
						Services: []api.ExportedService{{
							Name: "ping",
							Consumers: []api.ServiceConsumer{{
								Peer: "peer-dc2-default",
							}},
						}},
					},
				},
			},
		},
		Peerings: []*topology.Peering{{
			Dialing: topology.PeerCluster{
				Name: "dc1",
			},
			Accepting: topology.PeerCluster{
				Name: "dc2",
			},
		}},
	}

	sp := sprawltest.Launch(t, cfg)

	for _, cluster := range sp.Topology().Clusters {
		leader, err := sp.Leader(cluster.Name)
		require.NoError(t, err)
		t.Logf("%s: leader = %s", cluster.Name, leader.ID())

		followers, err := sp.Followers(cluster.Name)
		require.NoError(t, err)
		for _, f := range followers {
			t.Logf("%s: follower = %s", cluster.Name, f.ID())
		}
	}
}
