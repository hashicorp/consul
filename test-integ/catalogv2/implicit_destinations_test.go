// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package catalogv2

import (
	"fmt"
	"testing"

	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"

	"github.com/hashicorp/consul/test-integ/topoutil"
)

// TestBasicL4ImplicitDestinations sets up the following:
//
// - 1 cluster (no peering / no wanfed)
// - 3 servers in that cluster
// - v2 arch is activated
// - for each tenancy, only using v2 constructs:
//   - a server exposing 2 tcp ports
//   - a client with transparent proxy enabled and no explicit upstreams
//   - a traffic permission granting the client access to the service on all ports
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
func TestBasicL4ImplicitDestinations(t *testing.T) {
	cfg := testBasicL4ImplicitDestinationsCreator{}.NewConfig(t)

	sp := sprawltest.Launch(t, cfg)

	var (
		asserter = topoutil.NewAsserter(sp)

		topo    = sp.Topology()
		cluster = topo.Clusters["dc1"]

		ships = topo.ComputeRelationships()
	)

	clientV2 := sp.ResourceServiceClientForCluster(cluster.Name)

	t.Log(topology.RenderRelationships(ships))

	// Make sure things are truly in v2 not v1.
	for _, name := range []string{
		"static-server",
		"static-client",
	} {
		libassert.CatalogV2ServiceHasEndpointCount(t, clientV2, name, nil, 1)
	}

	// Check relationships
	for _, ship := range ships {
		t.Run("relationship: "+ship.String(), func(t *testing.T) {
			var (
				svc = ship.Caller
				u   = ship.Upstream
			)

			clusterPrefix := clusterPrefixForUpstream(u)

			asserter.UpstreamEndpointStatus(t, svc, clusterPrefix+".", "HEALTHY", 1)
			if u.LocalPort > 0 {
				asserter.HTTPServiceEchoes(t, svc, u.LocalPort, "")
			}
			asserter.FortioFetch2FortioName(t, svc, u, cluster.Name, u.ID)
		})
	}
}

type testBasicL4ImplicitDestinationsCreator struct{}

func (c testBasicL4ImplicitDestinationsCreator) NewConfig(t *testing.T) *topology.Config {
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
		Images: topoutil.TargetImages(),
		Networks: []*topology.Network{
			{Name: clusterName},
			{Name: "wan", Type: "wan"},
		},
		Clusters: []*topology.Cluster{
			cluster,
		},
	}
}

func (c testBasicL4ImplicitDestinationsCreator) topologyConfigAddNodes(
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

	serverNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: partition,
		Name:      nodeName(),
		Services: []*topology.Service{
			topoutil.NewFortioServiceWithDefaults(
				clusterName,
				newServiceID("static-server"),
				topology.NodeVersionV2,
				func(svc *topology.Service) {
					svc.EnableTransparentProxy = true
				},
			),
		},
	}
	clientNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: partition,
		Name:      nodeName(),
		Services: []*topology.Service{
			topoutil.NewFortioServiceWithDefaults(
				clusterName,
				newServiceID("static-client"),
				topology.NodeVersionV2,
				func(svc *topology.Service) {
					svc.EnableTransparentProxy = true
					svc.ImpliedUpstreams = []*topology.Upstream{
						{
							ID:       newServiceID("static-server"),
							PortName: "http",
						},
						{
							ID:       newServiceID("static-server"),
							PortName: "http-alt",
						},
					}
				},
			),
		},
	}
	trafficPerms := sprawltest.MustSetResourceData(t, &pbresource.Resource{
		Id: &pbresource.ID{
			Type:    pbauth.TrafficPermissionsType,
			Name:    "static-server-perms",
			Tenancy: tenancy,
		},
	}, &pbauth.TrafficPermissions{
		Destination: &pbauth.Destination{
			IdentityName: "static-server",
		},
		Action: pbauth.Action_ACTION_ALLOW,
		Permissions: []*pbauth.Permission{{
			Sources: []*pbauth.Source{{
				IdentityName: "static-client",
				Namespace:    namespace,
			}},
		}},
	})

	cluster.Nodes = append(cluster.Nodes,
		clientNode,
		serverNode,
	)

	cluster.InitialResources = append(cluster.InitialResources,
		trafficPerms,
	)
}
