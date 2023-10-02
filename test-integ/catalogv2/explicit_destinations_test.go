// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogv2

import (
	"fmt"
	"strings"
	"testing"

	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"

	"github.com/hashicorp/consul/test-integ/topoutil"
)

// TestBasicL4ExplicitDestination sets up the following:
//
// - 1 cluster (no peering / no wanfed)
// - 3 servers in that cluster
// - v2 arch is activated
// - for each tenancy, only using v2 constructs:
//   - a client with one explicit destination to a single port service
//   - a client with multiple explicit destinations to multiple ports of the
//     same multiport service
//
// When this test is executed in CE it will only use the default/default
// tenancy.
//
// When this test is executed in Enterprise it will additionally test the same
// things within these tenancies:
//
// - part1/default
// - default/nsa
// - part1/nsa
func TestBasicL4ExplicitDestination(t *testing.T) {
	cfg := testBasicL4ExplicitDestinationCreator{}.NewConfig(t)

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
	for _, name := range []string{
		"single-server",
		"single-client",
		"multi-server",
		"multi-client",
	} {
		libassert.CatalogV2ServiceHasEndpointCount(t, clientV2, name, nil, 1)
	}

	// Check relationships
	for _, ship := range ships {
		t.Run("relationship: "+ship.String(), func(t *testing.T) {
			var (
				svc           = ship.Caller
				u             = ship.Upstream
				clusterPrefix string
			)

			if u.Peer == "" {
				if u.ID.PartitionOrDefault() == "default" {
					clusterPrefix = strings.Join([]string{u.PortName, u.ID.Name, u.ID.Namespace, u.Cluster, "internal"}, ".")
				} else {
					clusterPrefix = strings.Join([]string{u.PortName, u.ID.Name, u.ID.Namespace, u.ID.Partition, u.Cluster, "internal-v1"}, ".")
				}
			} else {
				clusterPrefix = strings.Join([]string{u.ID.Name, u.ID.Namespace, u.Peer, "external"}, ".")
			}

			asserter.UpstreamEndpointStatus(t, svc, clusterPrefix+".", "HEALTHY", 1)
			asserter.HTTPServiceEchoes(t, svc, u.LocalPort, "")
			asserter.FortioFetch2FortioName(t, svc, u, cluster.Name, u.ID)
		})
	}
}

type testBasicL4ExplicitDestinationCreator struct{}

func (c testBasicL4ExplicitDestinationCreator) NewConfig(t *testing.T) *topology.Config {
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
		Images: getImages(),
		Networks: []*topology.Network{
			{Name: clusterName},
			{Name: "wan", Type: "wan"},
		},
		Clusters: []*topology.Cluster{
			cluster,
		},
	}
}

func (c testBasicL4ExplicitDestinationCreator) topologyConfigAddNodes(
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

	singleportServerNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: partition,
		Name:      nodeName(),
		Services: []*topology.Service{
			topoutil.NewFortioServiceWithDefaults(
				clusterName,
				newServiceID("single-server"),
				topology.NodeVersionV2,
				nil,
			),
		},
	}
	singleportClientNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: partition,
		Name:      nodeName(),
		Services: []*topology.Service{
			topoutil.NewFortioServiceWithDefaults(
				clusterName,
				newServiceID("single-client"),
				topology.NodeVersionV2,
				func(svc *topology.Service) {
					delete(svc.Ports, "grpc")     // v2 mode turns this on, so turn it off
					delete(svc.Ports, "http-alt") // v2 mode turns this on, so turn it off
					svc.Upstreams = []*topology.Upstream{{
						ID:           newServiceID("single-server"),
						PortName:     "http",
						LocalAddress: "0.0.0.0", // needed for an assertion
						LocalPort:    5000,
					}}
				},
			),
		},
	}
	singleportTrafficPerms := sprawltest.MustSetResourceData(t, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbauth.TrafficPermissionsType,
			Name:    "single-server-perms",
			Tenancy: tenancy,
		},
	}, &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "single-server",
		},
		Action: pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{{
			Sources: []*pbauth.Source{{
				IdentityName: "single-client",
				Namespace:    namespace,
			}},
		}},
	})

	multiportServerNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: partition,
		Name:      nodeName(),
		Services: []*topology.Service{
			topoutil.NewFortioServiceWithDefaults(
				clusterName,
				newServiceID("multi-server"),
				topology.NodeVersionV2,
				nil,
			),
		},
	}
	multiportClientNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: partition,
		Name:      nodeName(),
		Services: []*topology.Service{
			topoutil.NewFortioServiceWithDefaults(
				clusterName,
				newServiceID("multi-client"),
				topology.NodeVersionV2,
				func(svc *topology.Service) {
					svc.Upstreams = []*topology.Upstream{
						{
							ID:           newServiceID("multi-server"),
							PortName:     "http",
							LocalAddress: "0.0.0.0", // needed for an assertion
							LocalPort:    5000,
						},
						{
							ID:           newServiceID("multi-server"),
							PortName:     "http-alt",
							LocalAddress: "0.0.0.0", // needed for an assertion
							LocalPort:    5001,
						},
					}
				},
			),
		},
	}
	multiportTrafficPerms := sprawltest.MustSetResourceData(t, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbauth.TrafficPermissionsType,
			Name:    "multi-server-perms",
			Tenancy: tenancy,
		},
	}, &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "multi-server",
		},
		Action: pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{{
			Sources: []*pbauth.Source{{
				IdentityName: "multi-client",
				Namespace:    namespace,
			}},
		}},
	})

	cluster.Nodes = append(cluster.Nodes,
		singleportClientNode,
		singleportServerNode,
		multiportClientNode,
		multiportServerNode,
	)

	cluster.InitialResources = append(cluster.InitialResources,
		singleportTrafficPerms,
		multiportTrafficPerms,
	)
}
