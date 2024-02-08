// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sprawltest_test

import (
	"strconv"
	"testing"

	"github.com/hashicorp/consul/api"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	pbresource "github.com/hashicorp/consul/proto-public/pbresource/v1"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

func TestSprawl_CatalogV2(t *testing.T) {
	serversDC1 := newTopologyServerSet("dc1-server", 3, []string{"dc1", "wan"}, nil)

	cfg := &topology.Config{
		Images: topology.Images{
			ConsulCE:         "hashicorppreview/consul:1.17-dev",
			ConsulEnterprise: "hashicorppreview/consul-enterprise:1.17-dev",
			Dataplane:        "hashicorppreview/consul-dataplane:1.3-dev",
		},
		Networks: []*topology.Network{
			{Name: "dc1"},
			{Name: "wan", Type: "wan"},
		},
		Clusters: []*topology.Cluster{
			{
				Enterprise: true,
				Name:       "dc1",
				Nodes: topology.MergeSlices(serversDC1, []*topology.Node{
					{
						Kind:    topology.NodeKindDataplane,
						Version: topology.NodeVersionV2,
						Name:    "dc1-client1",
						Workloads: []*topology.Workload{
							{
								ID:             topology.ID{Name: "ping"},
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
								Destinations: []*topology.Destination{{
									ID:        topology.ID{Name: "pong"},
									LocalPort: 9090,
								}},
							},
						},
					},
					{
						Kind:    topology.NodeKindDataplane,
						Version: topology.NodeVersionV2,
						Name:    "dc1-client2",
						Workloads: []*topology.Workload{
							{
								ID:             topology.ID{Name: "pong"},
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
								Destinations: []*topology.Destination{{
									ID:        topology.ID{Name: "ping"},
									LocalPort: 9090,
								}},
							},
						},
					},
				}),
				InitialResources: []*pbresource.Resource{
					sprawltest.MustSetResourceData(t, &pbresource.Resource{
						Id: &pbresource.ID{
							Type: pbmesh.HTTPRouteType,
							Name: "test-http-route",
						},
					}, &pbmesh.HTTPRoute{
						ParentRefs: []*pbmesh.ParentReference{{
							Ref: &pbresource.Reference{
								Type: pbcatalog.ServiceType,
								Name: "test",
							},
						}},
					}),
					sprawltest.MustSetResourceData(t, &pbresource.Resource{
						Id: &pbresource.ID{
							Type: pbauth.TrafficPermissionsType,
							Name: "ping-perms",
						},
					}, &pbauth.TrafficPermissions{
						Destination: &pbauth.Destination{
							IdentityName: "ping",
						},
						Action: pbauth.Action_ACTION_ALLOW,
						Permissions: []*pbauth.Permission{{
							Sources: []*pbauth.Source{{
								IdentityName: "pong",
							}},
						}},
					}),
					sprawltest.MustSetResourceData(t, &pbresource.Resource{
						Id: &pbresource.ID{
							Type: pbauth.TrafficPermissionsType,
							Name: "pong-perms",
						},
					}, &pbauth.TrafficPermissions{
						Destination: &pbauth.Destination{
							IdentityName: "pong",
						},
						Action: pbauth.Action_ACTION_ALLOW,
						Permissions: []*pbauth.Permission{{
							Sources: []*pbauth.Source{{
								IdentityName: "ping",
							}},
						}},
					}),
				},
			},
		},
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

func TestSprawl(t *testing.T) {
	serversDC1 := newTopologyServerSet("dc1-server", 3, []string{"dc1", "wan"}, nil)
	serversDC2 := newTopologyServerSet("dc2-server", 3, []string{"dc2", "wan"}, nil)

	cfg := &topology.Config{
		Images: topology.Images{
			// ConsulEnterprise: "consul-dev:latest",
			ConsulCE:         "hashicorppreview/consul:1.17-dev",
			ConsulEnterprise: "hashicorppreview/consul-enterprise:1.17-dev",
			Dataplane:        "hashicorppreview/consul-dataplane:1.3-dev",
		},
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
						Workloads: []*topology.Workload{
							{
								ID:             topology.ID{Name: "mesh-gateway"},
								Port:           8443,
								EnvoyAdminPort: 19000,
								IsMeshGateway:  true,
							},
						},
					},
					{
						Kind: topology.NodeKindClient,
						Name: "dc1-client2",
						Workloads: []*topology.Workload{
							{
								ID:             topology.ID{Name: "ping"},
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
								Destinations: []*topology.Destination{{
									ID:        topology.ID{Name: "pong"},
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
						Workloads: []*topology.Workload{
							{
								ID:             topology.ID{Name: "mesh-gateway"},
								Port:           8443,
								EnvoyAdminPort: 19000,
								IsMeshGateway:  true,
							},
						},
					},
					{
						Kind: topology.NodeKindDataplane,
						Name: "dc2-client2",
						Workloads: []*topology.Workload{
							{
								ID:             topology.ID{Name: "pong"},
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
								Destinations: []*topology.Destination{{
									ID:        topology.ID{Name: "ping"},
									LocalPort: 9090,
									Peer:      "peer-dc1-default",
								}},
							},
						},
					},
					{
						Kind:    topology.NodeKindDataplane,
						Version: topology.NodeVersionV2,
						Name:    "dc2-client3",
						Workloads: []*topology.Workload{
							{
								ID:             topology.ID{Name: "pong"},
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
								Destinations: []*topology.Destination{{
									ID:        topology.ID{Name: "ping"},
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
