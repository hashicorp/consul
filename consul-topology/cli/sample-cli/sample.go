package main

import (
	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul-topology/topology"
)

func SampleTopology1() *topology.Config {
	return &topology.Config{
		Networks: []*topology.Network{
			{Name: "dc1"},
			{Name: "dc2"},
			{Name: "wan", Type: "wan"},
		},
		Clusters: []*topology.Cluster{
			{
				Enterprise: true,
				Name:       "dc1",
				Nodes:      sampleTopology1_cluster_dc1_nodes(),
				InitialConfigEntries: []api.ConfigEntry{
					&api.ExportedServicesConfigEntry{
						Name: "default",
						Services: []api.ExportedService{
							{
								Name: "ping",
								Consumers: []api.ServiceConsumer{{
									Peer: "peer-dc2-default",
								}},
							},
							{
								Name: "pong",
								Consumers: []api.ServiceConsumer{{
									Peer: "peer-dc2-default",
								}},
							},
						},
					},
				},
			},
			{
				Enterprise: true,
				Name:       "dc2",
				Nodes:      sampleTopology1_cluster_dc2_nodes(),
				InitialConfigEntries: []api.ConfigEntry{
					&api.ExportedServicesConfigEntry{
						Name: "default",
						Services: []api.ExportedService{
							{
								Name: "ping",
								Consumers: []api.ServiceConsumer{{
									Peer: "peer-dc1-default",
								}},
							},
							{
								Name: "pong",
								Consumers: []api.ServiceConsumer{{
									Peer: "peer-dc1-default",
								}},
							},
						},
					},
				},
			},
		},
		Peerings: []*topology.Peering{
			{
				Dialing: topology.PeerCluster{
					Name:      "dc1",
					Partition: "default",
				},
				Accepting: topology.PeerCluster{
					Name:      "dc2",
					Partition: "default",
				},
			},
		},
	}
}

func sampleTopology1_cluster_dc1_nodes() []*topology.Node {
	var out []*topology.Node
	addNode := func(n *topology.Node) {
		out = append(out, n)
	}

	// NOTE: the order of nodes in this list dictates assigned
	// IP addresses. Don't permute mid-test run unless you want
	// to scramble the IPs.

	// Servers
	out = append(out, makeServers(
		[]string{
			"dc1-server1",
			"dc1-server2",
			"dc1-server3",
		},
		[]*topology.Address{
			{Network: "dc1"},
			{Network: "wan"},
		},
	)...)

	addNode(&topology.Node{
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
	})
	addNode(&topology.Node{
		Kind:      topology.NodeKindClient,
		Name:      "dc1-client2",
		Partition: "ap1",
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
					ID:        topology.ServiceID{Name: "pong", Namespace: "ns1"},
					LocalPort: 9090,
				}},
			},
		},
	})
	addNode(&topology.Node{
		Kind:      topology.NodeKindClient,
		Name:      "dc1-client3",
		Partition: "ap1",
		Services: []*topology.Service{
			{
				ID:             topology.ServiceID{Name: "pong", Namespace: "ns1"},
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
				}},
			},
		},
	})
	// TODO: dataplane1 is a MGW
	addNode(&topology.Node{
		Kind: topology.NodeKindDataplane,
		Name: "dc1-dataplane2",
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
				}},
			},
		},
	})
	addNode(&topology.Node{
		Kind: topology.NodeKindDataplane,
		Name: "dc1-dataplane3",
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
				}},
			},
		},
	})
	return out
}

func sampleTopology1_cluster_dc2_nodes() []*topology.Node {
	var out []*topology.Node
	addNode := func(n *topology.Node) {
		out = append(out, n)
	}

	// NOTE: the order of nodes in this list dictates assigned
	// IP addresses. Don't permute mid-test run unless you want
	// to scramble the IPs.

	// Servers
	out = append(out, makeServers(
		[]string{
			"dc2-server1",
			"dc2-server2",
			"dc2-server3",
		},
		[]*topology.Address{
			{Network: "dc2"},
			{Network: "wan"},
		},
	)...)

	addNode(&topology.Node{
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
	})

	return out
}
