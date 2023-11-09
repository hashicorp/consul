// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogv2

import (
	"fmt"
	"testing"

	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"

	"github.com/hashicorp/consul/test-integ/topoutil"
)

func TestSplitterFeaturesL7ExplicitDestinations(t *testing.T) {
	cfg := testSplitterFeaturesL7ExplicitDestinationsCreator{}.NewConfig(t)

	sp := sprawltest.Launch(t, cfg)

	var (
		asserter = topoutil.NewAsserter(sp)

		topo    = sp.Topology()
		cluster = topo.Clusters["dc1"]

		ships = topo.ComputeRelationships()
	)

	clientV2 := sp.ResourceServiceClientForCluster(cluster.Name)

	t.Log(topology.RenderRelationships(ships))

	// Make sure things are in v2.
	libassert.CatalogV2ServiceHasEndpointCount(t, clientV2, "static-client", nil, 1)
	libassert.CatalogV2ServiceHasEndpointCount(t, clientV2, "static-server-v1", nil, 1)
	libassert.CatalogV2ServiceHasEndpointCount(t, clientV2, "static-server-v2", nil, 1)
	libassert.CatalogV2ServiceHasEndpointCount(t, clientV2, "static-server", nil, 0)

	// Check relationships
	for _, ship := range ships {
		t.Run("relationship: "+ship.String(), func(t *testing.T) {
			var (
				svc = ship.Caller
				u   = ship.Upstream
			)

			v1ID := u.ID
			v1ID.Name = "static-server-v1"
			v1ClusterPrefix := clusterPrefix(u.PortName, v1ID, u.Cluster)

			v2ID := u.ID
			v2ID.Name = "static-server-v2"
			v2ClusterPrefix := clusterPrefix(u.PortName, v2ID, u.Cluster)

			// we expect 2 clusters, one for each leg of the split
			asserter.UpstreamEndpointStatus(t, svc, v1ClusterPrefix+".", "HEALTHY", 1)
			asserter.UpstreamEndpointStatus(t, svc, v2ClusterPrefix+".", "HEALTHY", 1)

			// Both should be possible.
			v1Expect := fmt.Sprintf("%s::%s", cluster.Name, v1ID.String())
			v2Expect := fmt.Sprintf("%s::%s", cluster.Name, v2ID.String())

			switch u.PortName {
			case "tcp":
				asserter.CheckBlankspaceNameTrafficSplitViaTCP(t, svc, u,
					map[string]int{v1Expect: 10, v2Expect: 90})
			case "grpc":
				asserter.CheckBlankspaceNameTrafficSplitViaGRPC(t, svc, u,
					map[string]int{v1Expect: 10, v2Expect: 90})
			case "http":
				asserter.CheckBlankspaceNameTrafficSplitViaHTTP(t, svc, u, false, "/",
					map[string]int{v1Expect: 10, v2Expect: 90})
			case "http2":
				asserter.CheckBlankspaceNameTrafficSplitViaHTTP(t, svc, u, true, "/",
					map[string]int{v1Expect: 10, v2Expect: 90})
			default:
				t.Fatalf("unexpected port name: %s", u.PortName)
			}
		})
	}
}

type testSplitterFeaturesL7ExplicitDestinationsCreator struct{}

func (c testSplitterFeaturesL7ExplicitDestinationsCreator) NewConfig(t *testing.T) *topology.Config {
	const clusterName = "dc1"

	servers := topoutil.NewTopologyServerSet(clusterName+"-server", 3, []string{clusterName, "wan"}, nil)

	cluster := &topology.Cluster{
		Enterprise: utils.IsEnterprise(),
		Name:       clusterName,
		Nodes:      servers,
	}

	lastNode := 0
	nodeName := func() string {
		lastNode++
		return fmt.Sprintf("%s-box%d", clusterName, lastNode)
	}

	c.topologyConfigAddNodes(t, cluster, nodeName, "default", "default")
	if cluster.Enterprise {
		c.topologyConfigAddNodes(t, cluster, nodeName, "part1", "default")
		c.topologyConfigAddNodes(t, cluster, nodeName, "part1", "nsa")
		c.topologyConfigAddNodes(t, cluster, nodeName, "default", "nsa")
	}

	return &topology.Config{
		Images: utils.TargetImages(),
		Networks: []*topology.Network{
			{Name: clusterName},
			{Name: "wan", Type: "wan"},
		},
		Clusters: []*topology.Cluster{
			cluster,
		},
	}
}

func (c testSplitterFeaturesL7ExplicitDestinationsCreator) topologyConfigAddNodes(
	t *testing.T,
	cluster *topology.Cluster,
	nodeName func() string,
	partition,
	namespace string,
) {
	clusterName := cluster.Name

	newServiceID := func(name string) topology.ServiceID {
		return topology.ServiceID{
			Partition: partition,
			Namespace: namespace,
			Name:      name,
		}
	}

	tenancy := &pbresource.Tenancy{
		Partition: partition,
		Namespace: namespace,
		PeerName:  "local",
	}

	v1ServerNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: partition,
		Name:      nodeName(),
		Workloads: []*topology.Service{
			topoutil.NewBlankspaceServiceWithDefaults(
				clusterName,
				newServiceID("static-server-v1"),
				topology.NodeVersionV2,
				func(svc *topology.Service) {
					svc.Meta = map[string]string{
						"version": "v1",
					}
					svc.WorkloadIdentity = "static-server-v1"
				},
			),
		},
	}
	v2ServerNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: partition,
		Name:      nodeName(),
		Workloads: []*topology.Service{
			topoutil.NewBlankspaceServiceWithDefaults(
				clusterName,
				newServiceID("static-server-v2"),
				topology.NodeVersionV2,
				func(svc *topology.Service) {
					svc.Meta = map[string]string{
						"version": "v2",
					}
					svc.WorkloadIdentity = "static-server-v2"
				},
			),
		},
	}
	clientNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: partition,
		Name:      nodeName(),
		Workloads: []*topology.Service{
			topoutil.NewBlankspaceServiceWithDefaults(
				clusterName,
				newServiceID("static-client"),
				topology.NodeVersionV2,
				func(svc *topology.Service) {
					svc.Upstreams = []*topology.Upstream{
						{
							ID:           newServiceID("static-server"),
							PortName:     "http",
							LocalAddress: "0.0.0.0", // needed for an assertion
							LocalPort:    5000,
						},
						{
							ID:           newServiceID("static-server"),
							PortName:     "http2",
							LocalAddress: "0.0.0.0", // needed for an assertion
							LocalPort:    5001,
						},
						{
							ID:           newServiceID("static-server"),
							PortName:     "grpc",
							LocalAddress: "0.0.0.0", // needed for an assertion
							LocalPort:    5002,
						},
						{
							ID:           newServiceID("static-server"),
							PortName:     "tcp",
							LocalAddress: "0.0.0.0", // needed for an assertion
							LocalPort:    5003,
						},
					}
				},
			),
		},
	}

	v1TrafficPerms := sprawltest.MustSetResourceData(t, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbauth.TrafficPermissionsType,
			Name:    "static-server-v1-perms",
			Tenancy: tenancy,
		},
	}, &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "static-server-v1",
		},
		Action: pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{{
			Sources: []*pbauth.Source{{
				IdentityName: "static-client",
				Namespace:    namespace,
			}},
		}},
	})
	v2TrafficPerms := sprawltest.MustSetResourceData(t, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbauth.TrafficPermissionsType,
			Name:    "static-server-v2-perms",
			Tenancy: tenancy,
		},
	}, &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "static-server-v2",
		},
		Action: pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{{
			Sources: []*pbauth.Source{{
				IdentityName: "static-client",
				Namespace:    namespace,
			}},
		}},
	})

	staticServerService := sprawltest.MustSetResourceData(t, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbcatalog.ServiceType,
			Name:    "static-server",
			Tenancy: tenancy,
		},
	}, &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			// This will result in a 50/50 uncontrolled split.
			Prefixes: []string{"static-server-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "http",
				Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
			},
			{
				TargetPort: "http2",
				Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP2,
			},
			{
				TargetPort: "grpc",
				Protocol:   pbcatalog.Protocol_PROTOCOL_GRPC,
			},
			{
				TargetPort: "tcp",
				Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
			},
			{
				TargetPort: "mesh",
				Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
			},
		},
	})

	httpServerRoute := sprawltest.MustSetResourceData(t, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbmesh.HTTPRouteType,
			Name:    "static-server-http-route",
			Tenancy: tenancy,
		},
	}, &pbmesh.HTTPRoute{
		ParentRefs: []*pbmesh.ParentReference{
			{
				Ref: &pbresource.Reference{
					Type:    pbcatalog.ServiceType,
					Name:    "static-server",
					Tenancy: tenancy,
				},
				Port: "http",
			},
			{
				Ref: &pbresource.Reference{
					Type:    pbcatalog.ServiceType,
					Name:    "static-server",
					Tenancy: tenancy,
				},
				Port: "http2",
			},
		},
		Rules: []*pbmesh.HTTPRouteRule{{
			BackendRefs: []*pbmesh.HTTPBackendRef{
				{
					BackendRef: &pbmesh.BackendReference{
						Ref: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Name:    "static-server-v1",
							Tenancy: tenancy,
						},
					},
					Weight: 10,
				},
				{
					BackendRef: &pbmesh.BackendReference{
						Ref: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Name:    "static-server-v2",
							Tenancy: tenancy,
						},
					},
					Weight: 90,
				},
			},
		}},
	})
	grpcServerRoute := sprawltest.MustSetResourceData(t, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbmesh.GRPCRouteType,
			Name:    "static-server-grpc-route",
			Tenancy: tenancy,
		},
	}, &pbmesh.GRPCRoute{
		ParentRefs: []*pbmesh.ParentReference{{
			Ref: &pbresource.Reference{
				Type:    pbcatalog.ServiceType,
				Name:    "static-server",
				Tenancy: tenancy,
			},
			Port: "grpc",
		}},
		Rules: []*pbmesh.GRPCRouteRule{{
			BackendRefs: []*pbmesh.GRPCBackendRef{
				{
					BackendRef: &pbmesh.BackendReference{
						Ref: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Name:    "static-server-v1",
							Tenancy: tenancy,
						},
					},
					Weight: 10,
				},
				{
					BackendRef: &pbmesh.BackendReference{
						Ref: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Name:    "static-server-v2",
							Tenancy: tenancy,
						},
					},
					Weight: 90,
				},
			},
		}},
	})
	tcpServerRoute := sprawltest.MustSetResourceData(t, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbmesh.TCPRouteType,
			Name:    "static-server-tcp-route",
			Tenancy: tenancy,
		},
	}, &pbmesh.TCPRoute{
		ParentRefs: []*pbmesh.ParentReference{{
			Ref: &pbresource.Reference{
				Type:    pbcatalog.ServiceType,
				Name:    "static-server",
				Tenancy: tenancy,
			},
			Port: "tcp",
		}},
		Rules: []*pbmesh.TCPRouteRule{{
			BackendRefs: []*pbmesh.TCPBackendRef{
				{
					BackendRef: &pbmesh.BackendReference{
						Ref: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Name:    "static-server-v1",
							Tenancy: tenancy,
						},
					},
					Weight: 10,
				},
				{
					BackendRef: &pbmesh.BackendReference{
						Ref: &pbresource.Reference{
							Type:    pbcatalog.ServiceType,
							Name:    "static-server-v2",
							Tenancy: tenancy,
						},
					},
					Weight: 90,
				},
			},
		}},
	})

	cluster.Nodes = append(cluster.Nodes,
		clientNode,
		v1ServerNode,
		v2ServerNode,
	)

	cluster.InitialResources = append(cluster.InitialResources,
		staticServerService,
		v1TrafficPerms,
		v2TrafficPerms,
		httpServerRoute,
		tcpServerRoute,
		grpcServerRoute,
	)
}
