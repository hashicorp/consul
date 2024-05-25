// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/sdk/testutil"
)

func TestCompile_CE(t *testing.T) {
	type testcase struct {
		in        *Config
		expect    *Topology
		expectErr string
	}

	logger := hclog.NewNullLogger()

	const clusterID = "87c82bd03dc89d4d"

	run := func(t *testing.T, tc testcase) {
		got, err := compile(logger, tc.in, nil, clusterID)
		if tc.expectErr == "" {
			require.NotNil(t, tc.expect, "field must be set")
			require.NoError(t, err)

			// Set recursive pointers on expectations.
			for _, c := range tc.expect.Clusters {
				for _, n := range c.Nodes {
					for _, w := range n.Workloads {
						w.Node = n
					}
				}
			}

			assertDeepEqual(t, tc.expect, got)
		} else {
			require.Nil(t, tc.expect, "field cannot be set")
			require.Nil(t, got)
			testutil.RequireErrorContains(t, err, tc.expectErr)
		}
	}

	cases := map[string]testcase{
		"nil": {
			in:        nil,
			expectErr: `config is required`,
		},
		"consul image cannot be set at the top level": {
			in: &Config{
				Images: DefaultImages().ChooseConsul(true),
			},
			expectErr: `topology.images.consul cannot be set at this level`,
		},
		"no networks": {
			in:        &Config{},
			expectErr: `topology.networks is empty`,
		},
		"bad network/no name": {
			in: &Config{
				Networks: []*Network{{
					//
				}},
			},
			expectErr: `network name is not valid`,
		},
		"bad network/invalid name": {
			in: &Config{
				Networks: []*Network{{
					Name: "-123",
				}},
			},
			expectErr: `network name is not valid`,
		},
		"bad network/should not use DockerName": {
			in: &Config{
				Networks: []*Network{{
					Name:       "foo",
					DockerName: "blah",
				}},
			},
			expectErr: `network "foo" should not specify DockerName`,
		},
		"bad network/duplicate names": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
					{Name: "foo"},
				},
			},
			expectErr: `cannot have two networks with the same name "foo"`,
		},
		"bad network/unknown type": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo", Type: "ext"},
				},
			},
			expectErr: `network "foo" has unknown type "ext"`,
		},
		"good networks one server node": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo1"},
					{Name: "foo2", Type: "lan"},
					{Name: "foo3", Type: "wan"},
				},
				Clusters: []*Cluster{{
					Name: "foo2",
					Nodes: []*Node{{
						Kind: NodeKindServer,
						Name: "node1",
						Addresses: []*Address{
							{Network: "foo2"},
							{Network: "foo3"},
						},
					}},
				}},
			},
			expect: &Topology{
				ID:     clusterID,
				Images: DefaultImages(),
				Networks: map[string]*Network{
					"foo1": {Name: "foo1", Type: "lan", DockerName: "cslc-foo1-" + clusterID},
					"foo2": {Name: "foo2", Type: "lan", DockerName: "cslc-foo2-" + clusterID},
					"foo3": {Name: "foo3", Type: "wan", DockerName: "cslc-foo3-" + clusterID},
				},
				Clusters: map[string]*Cluster{
					"foo2": {
						Name:        "foo2",
						NetworkName: "foo2",
						Datacenter:  "foo2",
						Images:      DefaultImages().ChooseConsul(false),
						Nodes: []*Node{{
							Kind:      NodeKindServer,
							Partition: "default",
							Name:      "node1",
							Images:    DefaultImages().ChooseConsul(false).ChooseNode(NodeKindServer),
							Addresses: []*Address{
								{Network: "foo2", Type: "lan", DockerNetworkName: "cslc-foo2-" + clusterID},
								{Network: "foo3", Type: "wan", DockerNetworkName: "cslc-foo3-" + clusterID},
							},
							Cluster:    "foo2",
							Datacenter: "foo2",
						}},
						Partitions: []*Partition{{
							Name:       "default",
							Namespaces: []string{"default"},
						}},
					},
				},
			},
		},
		"no clusters": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo1"},
				},
			},
			expectErr: `topology.clusters is empty`,
		},
		"bad cluster/no name": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo1"},
				},
				Clusters: []*Cluster{{}},
			},
			expectErr: `error building cluster "": cluster has no name`,
		},
		"bad cluster/invalid name": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo1"},
				},
				Clusters: []*Cluster{{
					Name: "-123",
				}},
			},
			expectErr: `error building cluster "-123": cluster name is not valid: -123`,
		},
		"bad cluster/invalid dc": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo1"},
				},
				Clusters: []*Cluster{{
					Name:       "foo",
					Datacenter: "-123",
				}},
			},
			expectErr: `error building cluster "foo": datacenter name is not valid: -123`,
		},
		"bad cluster/missing network": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo1"},
				},
				Clusters: []*Cluster{{
					Name:        "foo",
					NetworkName: "bar",
				}},
			},
			expectErr: `error building cluster "foo": cluster "foo" uses network name "bar" that does not exist`,
		},
		"bad cluster/no nodes": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
				}},
			},
			expectErr: `error building cluster "foo": cluster "foo" has no nodes`,
		},
		"colliding clusters": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{
					{
						Name: "foo",
						Nodes: []*Node{{
							Kind: NodeKindServer,
							Name: "node1",
						}},
					},
					{
						Name: "foo",
						Nodes: []*Node{{
							Kind: NodeKindServer,
							Name: "node1",
						}},
					},
				},
			},
			expectErr: `cannot have two clusters with the same name "foo"; use unique names and override the Datacenter field if that's what you want`,
		},
		// TODO: ERR: partitions in CE
		// TODO: ERR: namespaces in CE
		// TODO: ERR: servers  in a non-default partition
		"tenancy collection": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name:       "foo",
					Enterprise: true,
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
						},
						{
							Kind:      NodeKindClient,
							Name:      "mesh1",
							Partition: "ap1",
							Addresses: []*Address{
								{Network: "foo"},
							},
							Workloads: []*Workload{
								{
									ID:             NewID("zim", "ns1", ""),
									Image:          "busybox",
									Port:           8888,
									EnvoyAdminPort: 19000,
								},
								{
									ID:             NewID("gir", "ns2", ""),
									Image:          "busybox",
									Port:           8877,
									EnvoyAdminPort: 19001,
									Upstreams: []*Upstream{{
										ID:           NewID("gaz", "ns3", "ap3"),
										LocalAddress: "127.0.0.1",
										LocalPort:    5000,
									}},
								},
							},
						},
					},
				}},
			},
			expect: &Topology{
				ID:     clusterID,
				Images: DefaultImages(),
				Networks: map[string]*Network{
					"foo": {Name: "foo", Type: "lan", DockerName: "cslc-foo-" + clusterID},
				},
				Clusters: map[string]*Cluster{
					"foo": {
						Name:        "foo",
						NetworkName: "foo",
						Datacenter:  "foo",
						Enterprise:  true,
						Images:      DefaultImages().ChooseConsul(true),
						Nodes: []*Node{
							{
								Kind:      NodeKindServer,
								Name:      "server1",
								Partition: "default",
								Images:    DefaultImages().ChooseConsul(true).ChooseNode(NodeKindServer),
								Addresses: []*Address{
									{Network: "foo", Type: "lan", DockerNetworkName: "cslc-foo-" + clusterID},
								},
								Cluster:    "foo",
								Datacenter: "foo",
							},
							{
								Kind:      NodeKindClient,
								Name:      "mesh1",
								Partition: "ap1",
								Images:    DefaultImages().ChooseConsul(true).ChooseNode(NodeKindClient),
								Addresses: []*Address{
									{Network: "foo", Type: "lan", DockerNetworkName: "cslc-foo-" + clusterID},
								},
								Cluster:    "foo",
								Datacenter: "foo",
								Index:      1,
								Workloads: []*Workload{
									{
										ID:             NewID("zim", "ns1", "ap1"),
										Image:          "busybox",
										Port:           8888,
										EnvoyAdminPort: 19000,
									},
									{
										ID:             NewID("gir", "ns2", "ap1"),
										Image:          "busybox",
										Port:           8877,
										EnvoyAdminPort: 19001,
										Upstreams: []*Upstream{{
											ID:           NewID("gaz", "ns3", "ap3"),
											Cluster:      "foo",
											LocalAddress: "127.0.0.1",
											LocalPort:    5000,
										}},
									},
								},
							},
						},
						Partitions: []*Partition{
							{
								Name:       "ap1",
								Namespaces: []string{"default", "ns1", "ns2"},
							},
							{
								Name:       "ap3",
								Namespaces: []string{"default", "ns3"},
							},
							{
								Name:       "default",
								Namespaces: []string{"default"},
							},
						},
					},
				},
			},
		},
		"tls volume errantly set": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind: NodeKindServer,
						Name: "node1",
					}},
					TLSVolumeName: "foo",
				}},
			},
			expectErr: `error building cluster "foo": user cannot specify the TLSVolumeName field`,
		},
		"node/no name": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind: NodeKindServer,
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "": cluster "foo" node has no name`,
		},
		"node/invalid name": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind: NodeKindServer,
						Name: "-123",
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "-123": node name is not valid: -123`,
		},
		"node/bad kind": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Name: "zim",
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": cluster "foo" node "zim" has invalid kind`,
		},
		"node/invalid partition": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind:      NodeKindServer,
						Name:      "zim",
						Partition: "-123",
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": node partition is not valid: -123`,
		},
		"node/invalid usedPorts": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind:      NodeKindServer,
						Name:      "zim",
						usedPorts: map[int]int{5: 6},
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": user cannot specify the usedPorts field`,
		},
		"node/invalid index": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind:  NodeKindServer,
						Name:  "zim",
						Index: 99,
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": user cannot specify the node index`,
		},
		"node/missing address network": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind:      NodeKindServer,
						Name:      "zim",
						Addresses: []*Address{{
							//
						}},
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": cluster "foo" node "zim" has invalid address`,
		},
		"node/invalid address type": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind: NodeKindServer,
						Name: "zim",
						Addresses: []*Address{{
							Network: "foo",
							Type:    "lan",
						}},
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": user cannot specify the address type directly`,
		},
		"node/address network does not exist": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind: NodeKindServer,
						Name: "zim",
						Addresses: []*Address{{
							Network: "bar",
						}},
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": cluster "foo" node "zim" uses network name "bar" that does not exist`,
		},
		"node/no local addresses": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
					{Name: "bar", Type: "wan"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind: NodeKindServer,
						Name: "zim",
						Addresses: []*Address{{
							Network: "bar",
						}},
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": cluster "foo" node "zim" has no local addresses`,
		},
		"node/too many public addresses": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
					{Name: "bar", Type: "wan"},
					{Name: "baz", Type: "wan"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind: NodeKindServer,
						Name: "zim",
						Addresses: []*Address{
							{Network: "foo"},
							{Network: "bar"},
							{Network: "baz"},
						},
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": cluster "foo" node "zim" has more than one public address`,
		},
		"node/dataplane with more than one workload": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind: NodeKindDataplane,
							Name: "mesh1",
							Addresses: []*Address{
								{Network: "foo"},
							},
							Workloads: []*Workload{
								{ID: NewID("zim", "default", "default")},
								{ID: NewID("gir", "default", "default")},
							},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" uses dataplane, but has more than one service`,
		},
		"workload/invalid partition": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID: ID{
									Name:      "zim",
									Namespace: "default",
									Partition: "-123",
								},
								Image: "busybox",
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": service partition is not valid: -123`,
		},
		"workload/invalid namespace": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID: ID{
									Name:      "zim",
									Namespace: "-123",
									Partition: "default",
								},
								Image: "busybox",
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": service namespace is not valid: -123`,
		},
		"workload/invalid name": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID: ID{
									Name:      "-123",
									Namespace: "default",
									Partition: "default",
								},
								Image: "busybox",
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": service name is not valid: -123`,
		},
		"workload/mismatched partitions": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID: ID{
									Name:      "zim",
									Namespace: "default",
									Partition: "ap1",
								},
								Image: "busybox",
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": service zim on node mesh1 has mismatched partitions: ap1 != default`,
		},
		"workload/node collision": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindClient,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{
								{
									ID:             NewID("zim", "", ""),
									Image:          "busybox",
									Port:           8080,
									EnvoyAdminPort: 19000,
								},
								{
									ID:             NewID("zim", "", ""),
									Image:          "busybox",
									Port:           9090,
									EnvoyAdminPort: 19001,
								},
							},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cannot have two services on the same node "default/mesh1" in the same cluster "foo" with the same name "default/default/zim"`,
		},
		"workload/default-upstream/expl dest local address defaulting": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								Port:           8080,
								EnvoyAdminPort: 19000,
								Upstreams: []*Upstream{{
									ID:        NewID("gir", "", ""),
									LocalPort: 5000,
								}},
							}},
						},
					},
				}},
			},
			expect: &Topology{
				ID:     clusterID,
				Images: DefaultImages(),
				Networks: map[string]*Network{
					"foo": {Name: "foo", Type: "lan", DockerName: "cslc-foo-" + clusterID},
				},
				Clusters: map[string]*Cluster{
					"foo": {
						Name:        "foo",
						NetworkName: "foo",
						Datacenter:  "foo",
						Images:      DefaultImages().ChooseConsul(false),
						Nodes: []*Node{
							{
								Kind:      NodeKindServer,
								Partition: "default",
								Name:      "server1",
								Images:    DefaultImages().ChooseConsul(false).ChooseNode(NodeKindServer),
								Addresses: []*Address{
									{Network: "foo", Type: "lan", DockerNetworkName: "cslc-foo-" + clusterID},
								},
								Cluster:    "foo",
								Datacenter: "foo",
							},
							{
								Kind:      NodeKindDataplane,
								Partition: "default",
								Name:      "mesh1",
								Images:    DefaultImages().ChooseConsul(false).ChooseNode(NodeKindDataplane),
								Addresses: []*Address{
									{Network: "foo", Type: "lan", DockerNetworkName: "cslc-foo-" + clusterID},
								},
								Cluster:    "foo",
								Datacenter: "foo",
								Index:      1,
								Workloads: []*Workload{{
									ID:                      NewID("zim", "", ""),
									Image:                   "busybox",
									Port:                    8080,
									EnvoyAdminPort:          19000,
									EnvoyPublicListenerPort: 20000,
									Upstreams: []*Upstream{{
										ID:           NewID("gir", "", ""),
										LocalAddress: "127.0.0.1", // <--- this
										LocalPort:    5000,
										Cluster:      "foo",
									}},
								}},
							},
						},
						Partitions: []*Partition{{
							Name:       "default",
							Namespaces: []string{"default"},
						}},
					},
				},
			},
		},
		"workload/validate/no name": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindClient,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("", "", ""),
								Image:          "busybox",
								Port:           8080,
								EnvoyAdminPort: 19000,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": service name is not valid`,
		},
		"workload/validate/no image": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindClient,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Port:           8080,
								EnvoyAdminPort: 19000,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: service image is required`,
		},
		"workload/validate/no image mesh gateway": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindClient,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Port:           8080,
								EnvoyAdminPort: 19000,
								IsMeshGateway:  true,
							}},
						},
					},
				}},
			},
			expect: &Topology{
				ID:     clusterID,
				Images: DefaultImages(),
				Networks: map[string]*Network{
					"foo": {Name: "foo", Type: "lan", DockerName: "cslc-foo-" + clusterID},
				},
				Clusters: map[string]*Cluster{
					"foo": {
						Name:        "foo",
						NetworkName: "foo",
						Datacenter:  "foo",
						Images:      DefaultImages().ChooseConsul(false),
						Nodes: []*Node{
							{
								Kind:      NodeKindServer,
								Partition: "default",
								Name:      "server1",
								Images:    DefaultImages().ChooseConsul(false).ChooseNode(NodeKindServer),
								Addresses: []*Address{
									{Network: "foo", Type: "lan", DockerNetworkName: "cslc-foo-" + clusterID},
								},
								Cluster:    "foo",
								Datacenter: "foo",
							},
							{
								Kind:      NodeKindClient,
								Partition: "default",
								Name:      "mesh1",
								Images:    DefaultImages().ChooseConsul(false).ChooseNode(NodeKindClient),
								Addresses: []*Address{
									{Network: "foo", Type: "lan", DockerNetworkName: "cslc-foo-" + clusterID},
								},
								Cluster:    "foo",
								Datacenter: "foo",
								Index:      1,
								Workloads: []*Workload{{
									ID:             NewID("zim", "", ""),
									Port:           8080,
									EnvoyAdminPort: 19000,
									IsMeshGateway:  true,
								}},
							},
						},
						Partitions: []*Partition{{
							Name:       "default",
							Namespaces: []string{"default"},
						}},
					},
				},
			},
		},
		"workload/validate/singleport invalid port": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								EnvoyAdminPort: 19000,
								Port:           -999,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: service has invalid port`,
		},
		"workload/validate/mesh with no admin port": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:    NewID("zim", "", ""),
								Image: "busybox",
								Port:  999,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: envoy admin port is required`,
		},
		"workload/validate/no mesh with admin port": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:                 NewID("zim", "", ""),
								EnvoyAdminPort:     19000,
								DisableServiceMesh: true,
								Image:              "busybox",
								Port:               999,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot use envoy admin port without a service mesh`,
		},
		"workload/validate/expl dest with no name": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								EnvoyAdminPort: 19000,
								Image:          "busybox",
								Port:           999,
								Upstreams: []*Upstream{{
									ID: NewID("", "", ""),
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: upstream service name is required`,
		},
		"workload/validate/expl dest with no local port": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								EnvoyAdminPort: 19000,
								Image:          "busybox",
								Port:           999,
								Upstreams: []*Upstream{{
									ID: NewID("dib", "", ""),
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: upstream local port is required`,
		},
		"workload/validate/expl dest bad local address": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								EnvoyAdminPort: 19000,
								Image:          "busybox",
								Port:           999,
								Upstreams: []*Upstream{{
									ID:           NewID("dib", "", ""),
									LocalPort:    5000,
									LocalAddress: "clown@address",
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: upstream local address is invalid: clown@address`,
		},
		"workload/validate/disable-mesh/mgw": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:                 NewID("zim", "", ""),
								Image:              "busybox",
								EnvoyAdminPort:     19000,
								IsMeshGateway:      true,
								Port:               8443,
								DisableServiceMesh: true,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot disable service mesh and still run a mesh gateway`,
		},
		"workload/validate/disable-mesh/expl dest": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:                 NewID("zim", "", ""),
								Image:              "busybox",
								EnvoyAdminPort:     19000,
								Port:               8443,
								DisableServiceMesh: true,
								Upstreams: []*Upstream{{
									ID: NewID("gir", "", ""),
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot disable service mesh and configure upstreams`,
		},
		"workload/validate/disable-mesh/set admin port": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:                 NewID("zim", "", ""),
								Image:              "busybox",
								EnvoyAdminPort:     19000,
								Port:               8443,
								DisableServiceMesh: true,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot use envoy admin port without a service mesh`,
		},
		"workload/validate/enable-mesh/unset admin port": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{
						{
							Kind: NodeKindServer,
							Name: "server1",
							Addresses: []*Address{
								{Network: "foo"},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:    NewID("zim", "", ""),
								Image: "busybox",
								Port:  8443,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: envoy admin port is required`,
		},

		// TODO: OK: v2 mesh gateway port defaulting

		// TODO: collect tenancies from all places (cluster, initialconfigs, initialres, nodes, services, ...)
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

type testingT interface {
	Helper()
	Fatalf(format string, args ...interface{})
	require.TestingT
}

var ignoreUnexportedTypes = []any{
	Cluster{},
	Images{},
	Node{},
	Workload{},
}

func assertDeepEqual[V any](t testingT, exp, got V, msgAndArgs ...any) {
	t.Helper()

	if diff := cmp.Diff(exp, got, protocmp.Transform(), cmpopts.IgnoreUnexported(ignoreUnexportedTypes...)); diff != "" {
		format := "assertion failed: values are not equal\n--- expected\n+++ actual\n%v"
		args := []any{diff}

		if len(msgAndArgs) > 0 {
			suffix, ok := msgAndArgs[0].(string)
			require.True(t, ok)
			require.NotEmpty(t, suffix)
			format += "\n\nMessage: " + suffix
			args = append(args, msgAndArgs[1:]...)
		}
		t.Fatalf(format, args...)
	}
}
