package sprawltest_test

import (
	"strconv"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul-topology/sprawl/sprawltest"
	"github.com/hashicorp/consul-topology/topology"
)

func TestSprawl(t *testing.T) {
	serversDC1 := newTopologyServerSet("dc1-server", 3, []string{"dc1", "wan"}, nil)
	serversDC2 := newTopologyServerSet("dc2-server", 3, []string{"dc2", "wan"}, nil)

	cfg := &topology.Config{
		Networks: []*topology.Network{
			{Name: "dc1"},
			{Name: "dc2"},
			{Name: "wan", Type: "wan"},
		},
		Clusters: []*topology.Cluster{
			{
				Name: "dc1",
				Nodes: topology.MergeSlices(serversDC1, []*topology.Node{
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
				}),
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
				Nodes: topology.MergeSlices(serversDC2, []*topology.Node{
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
				}),
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

func newTopologyServerSet(
	namePrefix string,
	num int,
	networks []string,
	mutateFn func(i int, node *topology.Node),
) []*topology.Node {
	var out []*topology.Node
	for i := 1; i <= num; i++ {
		name := namePrefix + strconv.Itoa(i)

		node := &topology.Node{
			Kind: topology.NodeKindServer,
			Name: name,
		}
		for _, net := range networks {
			node.Addresses = append(node.Addresses, &topology.Address{Network: net})
		}

		if mutateFn != nil {
			mutateFn(i, node)
		}

		out = append(out, node)
	}
	return out
}
