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

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
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
							Version:   NodeVersionV1,
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
						Services: map[ID]*pbcatalog.Service{},
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
									Destinations: []*Destination{{
										ID:           NewID("gaz", "ns3", "ap3"),
										LocalAddress: "127.0.0.1",
										LocalPort:    5000,
									}},
								},
							},
						},
						{
							Kind:      NodeKindDataplane,
							Version:   NodeVersionV2,
							Name:      "mesh2",
							Partition: "ap2",
							Addresses: []*Address{
								{Network: "foo"},
							},
							Workloads: []*Workload{
								{
									ID:             NewID("gir", "ns4", "ap2"),
									Image:          "busybox",
									Port:           8877,
									EnvoyAdminPort: 19001,
									ImpliedDestinations: []*Destination{{
										ID:       NewID("gaz", "", "ap4"),
										PortName: "www",
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
						EnableV2:    true,
						Images:      DefaultImages().ChooseConsul(true),
						Services: map[ID]*pbcatalog.Service{
							NewID("gir", "ns4", "ap2"): {
								Workloads: &pbcatalog.WorkloadSelector{
									Names: []string{"gir-mesh2"},
								},
								Ports: []*pbcatalog.ServicePort{
									{TargetPort: "legacy", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
									{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
								},
								VirtualIps: []string{"10.244.0.2"},
							},
						},
						Nodes: []*Node{
							{
								Kind:      NodeKindServer,
								Version:   NodeVersionV1,
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
								Version:   NodeVersionV1,
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
										NodeVersion:    NodeVersionV1,
									},
									{
										ID:             NewID("gir", "ns2", "ap1"),
										Image:          "busybox",
										Port:           8877,
										EnvoyAdminPort: 19001,
										NodeVersion:    NodeVersionV1,
										Destinations: []*Destination{{
											ID:           NewID("gaz", "ns3", "ap3"),
											Cluster:      "foo",
											LocalAddress: "127.0.0.1",
											LocalPort:    5000,
										}},
									},
								},
							},
							{
								Kind:      NodeKindDataplane,
								Version:   NodeVersionV2,
								Name:      "mesh2",
								Partition: "ap2",
								Images:    DefaultImages().ChooseConsul(true).ChooseNode(NodeKindDataplane),
								Addresses: []*Address{
									{Network: "foo", Type: "lan", DockerNetworkName: "cslc-foo-" + clusterID},
								},
								Cluster:    "foo",
								Datacenter: "foo",
								Index:      2,
								Workloads: []*Workload{
									{
										ID:    NewID("gir", "ns4", "ap2"),
										Image: "busybox",
										Ports: map[string]*Port{
											"legacy": {Number: 8877, Protocol: "tcp", ActualProtocol: pbcatalog.Protocol_PROTOCOL_TCP},
											"mesh":   {Number: 20000, Protocol: "mesh", ActualProtocol: pbcatalog.Protocol_PROTOCOL_MESH},
										},
										EnvoyAdminPort:          19001,
										EnvoyPublicListenerPort: 20000,
										NodeVersion:             NodeVersionV2,
										V2Services:              []string{"gir"},
										WorkloadIdentity:        "gir",
										Workload:                "gir-mesh2",
										ImpliedDestinations: []*Destination{{
											ID:       NewID("gaz", "default", "ap4"),
											Cluster:  "foo", // TODO: why is this only sometimes populated?
											PortName: "www",
											Implied:  true,
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
								Name:       "ap2",
								Namespaces: []string{"default", "ns4"},
							},
							{
								Name:       "ap3",
								Namespaces: []string{"default", "ns3"},
							},
							{
								Name:       "ap4",
								Namespaces: []string{"default"},
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
		"explicit v2 services": {
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
					Services: map[ID]*pbcatalog.Service{
						NewID("zim", "default", "default"): {
							Ports: []*pbcatalog.ServicePort{
								{TargetPort: "http"},
								{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
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
						Images:      DefaultImages().ChooseConsul(false),
						Nodes: []*Node{{
							Kind:      NodeKindServer,
							Version:   NodeVersionV1,
							Partition: "default",
							Name:      "node1",
							Images:    DefaultImages().ChooseConsul(false).ChooseNode(NodeKindServer),
							Addresses: []*Address{
								{Network: "foo", Type: "lan", DockerNetworkName: "cslc-foo-" + clusterID},
							},
							Cluster:    "foo",
							Datacenter: "foo",
						}},
						EnableV2: true,
						Services: map[ID]*pbcatalog.Service{
							NewID("zim", "default", "default"): {
								Ports: []*pbcatalog.ServicePort{
									{TargetPort: "http", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
									{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
								},
								VirtualIps: []string{"10.244.0.2"},
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
		"explicit v2 services/bad workload selector": {
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
					Services: map[ID]*pbcatalog.Service{
						NewID("zim", "default", "default"): {
							Workloads: &pbcatalog.WorkloadSelector{Names: []string{"zzz"}},
							Ports: []*pbcatalog.ServicePort{
								{TargetPort: "http"},
								{TargetPort: "mesh"},
							},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": the workloads field for v2 service "default/default/zim" is not user settable`,
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
		"node/bad version": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind:    NodeKindServer,
						Version: "v3",
						Name:    "zim",
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": cluster "foo" node "zim" has invalid version: v3`,
		},
		"node/bad version for client": {
			in: &Config{
				Networks: []*Network{
					{Name: "foo"},
				},
				Clusters: []*Cluster{{
					Name: "foo",
					Nodes: []*Node{{
						Kind:    NodeKindClient,
						Version: NodeVersionV2,
						Name:    "zim",
					}},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "zim": v2 does not support client agents at this time`,
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
		"workload/v1 and implied dest": {
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
								Image:          "busybox",
								Port:           8080,
								EnvoyAdminPort: 19000,
								ImpliedDestinations: []*Destination{{
									ID: NewID("gir", "", ""),
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": v1 does not support implied destinations yet`,
		},
		"workload/default-destination/impl dest need port names in v2": {
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
							Version:   NodeVersionV2,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								Port:           8080,
								EnvoyAdminPort: 19000,
								ImpliedDestinations: []*Destination{{
									ID: NewID("gir", "", ""),
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": implicit destinations must use port names in v2`,
		},
		"workload/default-destination/expl dest port name legacy defaulting": {
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
							Version:   NodeVersionV2,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								Port:           8888,
								EnvoyAdminPort: 19000,
								Destinations: []*Destination{{
									ID:           NewID("gir", "", ""),
									LocalAddress: "127.0.0.1",
									LocalPort:    5000,
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
						EnableV2:    true,
						Services: map[ID]*pbcatalog.Service{
							NewID("zim", "default", "default"): {
								Workloads: &pbcatalog.WorkloadSelector{
									Names: []string{"zim-mesh1"},
								},
								Ports: []*pbcatalog.ServicePort{
									{TargetPort: "legacy", VirtualPort: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
									{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
								},
								VirtualIps: []string{"10.244.0.2"},
							},
						},
						Nodes: []*Node{
							{
								Kind:      NodeKindServer,
								Version:   NodeVersionV1,
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
								Version:   NodeVersionV2,
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
									ID:    NewID("zim", "", ""),
									Image: "busybox",
									Ports: map[string]*Port{
										"legacy": {Number: 8888, Protocol: "tcp", ActualProtocol: pbcatalog.Protocol_PROTOCOL_TCP},
										"mesh":   {Number: 20000, Protocol: "mesh", ActualProtocol: pbcatalog.Protocol_PROTOCOL_MESH},
									},
									EnvoyAdminPort:          19000,
									EnvoyPublicListenerPort: 20000,
									NodeVersion:             NodeVersionV2,
									V2Services:              []string{"zim"},
									WorkloadIdentity:        "zim",
									Workload:                "zim-mesh1",
									Destinations: []*Destination{{
										ID:           NewID("gir", "", ""),
										LocalAddress: "127.0.0.1",
										LocalPort:    5000,
										Cluster:      "foo",
										PortName:     "legacy", // <--- this
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
		"workload/default-destination/expl dest local address defaulting": {
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
							Version:   NodeVersionV1,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								Port:           8080,
								EnvoyAdminPort: 19000,
								Destinations: []*Destination{{
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
						Services:    map[ID]*pbcatalog.Service{
							//
						},
						Nodes: []*Node{
							{
								Kind:      NodeKindServer,
								Version:   NodeVersionV1,
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
								Version:   NodeVersionV1,
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
									NodeVersion:             NodeVersionV1,
									Destinations: []*Destination{{
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
		"workload/default-destination/expl dest cannot use port names in v1": {
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
							Version:   NodeVersionV1,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								Port:           8080,
								EnvoyAdminPort: 19000,
								Destinations: []*Destination{{
									ID:       NewID("gir", "", ""),
									PortName: "http",
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": explicit destinations cannot use port names in v1`,
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
						Services:    map[ID]*pbcatalog.Service{
							//
						},
						Nodes: []*Node{
							{
								Kind:      NodeKindServer,
								Version:   NodeVersionV1,
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
								Version:   NodeVersionV1,
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
									NodeVersion:    NodeVersionV1,
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
		"workload/validate/single and multiport v2": {
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
							Version:   NodeVersionV2,
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								Port:           8080,
								EnvoyAdminPort: 19000,
								Ports: map[string]*Port{
									"blah": {
										Number:   8181,
										Protocol: "tcp",
									},
								},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot specify both singleport and multiport on service in v2`,
		},
		"workload/validate/multiport nil port": {
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
							Version:   NodeVersionV2,
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								EnvoyAdminPort: 19000,
								Ports: map[string]*Port{
									"blah": nil,
								},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot be nil`,
		},
		"workload/validate/multiport negative port": {
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
							Version:   NodeVersionV2,
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								EnvoyAdminPort: 19000,
								Ports: map[string]*Port{
									"blah": {
										Number: -5,
									},
								},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: service has invalid port number`,
		},
		"workload/validate/multiport set actualprotocol": {
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
							Version:   NodeVersionV2,
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								EnvoyAdminPort: 19000,
								Ports: map[string]*Port{
									"blah": {
										Number:         8888,
										ActualProtocol: pbcatalog.Protocol_PROTOCOL_GRPC,
									},
								},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: user cannot specify ActualProtocol field`,
		},
		"workload/validate/multiport invalid port protocol": {
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
							Version:   NodeVersionV2,
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								Image:          "busybox",
								EnvoyAdminPort: 19000,
								Ports: map[string]*Port{
									"blah": {
										Number:   8888,
										Protocol: "zzzz",
									},
								},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: service has invalid port protocol "zzzz"`,
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
		"workload/validate/singleport tproxy": {
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
								ID:                     NewID("zim", "", ""),
								Image:                  "busybox",
								EnvoyAdminPort:         19000,
								Port:                   999,
								EnableTransparentProxy: true,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: tproxy does not work with v1 yet`,
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
								Destinations: []*Destination{{
									ID: NewID("", "", ""),
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: destination service name is required`,
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
								Destinations: []*Destination{{
									ID: NewID("dib", "", ""),
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: destination local port is required`,
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
								Destinations: []*Destination{{
									ID:           NewID("dib", "", ""),
									LocalPort:    5000,
									LocalAddress: "clown@address",
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: destination local address is invalid: clown@address`,
		},
		"workload/validate/expl dest with implied": {
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
							Version:   NodeVersionV2,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								EnvoyAdminPort: 19000,
								Image:          "busybox",
								Port:           999,
								Destinations: []*Destination{{
									ID:        NewID("dib", "", ""),
									LocalPort: 5000,
									PortName:  "http",
									Implied:   true,
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: implied field cannot be set`,
		},
		"workload/validate/impl dest with no name": {
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
							Version:   NodeVersionV2,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								EnvoyAdminPort: 19000,
								Image:          "busybox",
								Port:           999,
								ImpliedDestinations: []*Destination{{
									ID:       NewID("", "", ""),
									PortName: "http",
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: implied destination service name is required`,
		},
		"workload/validate/impl dest with local port": {
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
							Version:   NodeVersionV2,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								EnvoyAdminPort: 19000,
								Image:          "busybox",
								Port:           999,
								ImpliedDestinations: []*Destination{{
									ID:        NewID("dib", "", ""),
									PortName:  "http",
									LocalPort: 5000,
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: implied destination local port cannot be set`,
		},
		"workload/validate/impl dest with local address": {
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
							Version:   NodeVersionV2,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:             NewID("zim", "", ""),
								EnvoyAdminPort: 19000,
								Image:          "busybox",
								Port:           999,
								ImpliedDestinations: []*Destination{{
									ID:           NewID("dib", "", ""),
									PortName:     "http",
									LocalAddress: "127.0.0.1",
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: implied destination local address cannot be set`,
		},
		"workload/validate/v1 cannot use multiport": {
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
								Ports:          map[string]*Port{"web": {Number: 8080}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot specify multiport on service in v1`,
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
							Version:   NodeVersionV2,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:                 NewID("zim", "", ""),
								Image:              "busybox",
								EnvoyAdminPort:     19000,
								Port:               8443,
								DisableServiceMesh: true,
								Destinations: []*Destination{{
									ID: NewID("gir", "", ""),
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot disable service mesh and configure destinations`,
		},
		"workload/validate/disable-mesh/impl dest": {
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
							Version:   NodeVersionV2,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:                 NewID("zim", "", ""),
								Image:              "busybox",
								EnvoyAdminPort:     19000,
								Port:               8443,
								DisableServiceMesh: true,
								ImpliedDestinations: []*Destination{{
									ID:       NewID("gir", "", ""),
									PortName: "web",
								}},
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot disable service mesh and configure implied destinations`,
		},
		"workload/validate/disable-mesh/tproxy": {
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
							Version:   NodeVersionV2,
							Name:      "mesh1",
							Addresses: []*Address{{Network: "foo"}},
							Workloads: []*Workload{{
								ID:                     NewID("zim", "", ""),
								Image:                  "busybox",
								EnvoyAdminPort:         19000,
								Port:                   8443,
								DisableServiceMesh:     true,
								EnableTransparentProxy: true,
							}},
						},
					},
				}},
			},
			expectErr: `error building cluster "foo": error compiling node "mesh1": cluster "foo" node "mesh1" service "default/default/zim" is not valid: cannot disable service mesh and activate tproxy`,
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
							Version:   NodeVersionV2,
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
							Version:   NodeVersionV2,
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
