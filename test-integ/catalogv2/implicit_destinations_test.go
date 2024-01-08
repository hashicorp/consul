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
	tenancies := []*pbresource.Tenancy{{
		Namespace: "default",
		Partition: "default",
	}}
	if utils.IsEnterprise() {
		tenancies = append(tenancies, &pbresource.Tenancy{
			Namespace: "default",
			Partition: "nsa",
		})
		tenancies = append(tenancies, &pbresource.Tenancy{
			Namespace: "part1",
			Partition: "default",
		})
		tenancies = append(tenancies, &pbresource.Tenancy{
			Namespace: "part1",
			Partition: "nsa",
		})
	}

	cfg := testBasicL4ImplicitDestinationsCreator{
		tenancies: tenancies,
	}.NewConfig(t)

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
	for _, tenancy := range tenancies {
		for _, name := range []string{
			"static-server",
			"static-client",
		} {
			libassert.CatalogV2ServiceHasEndpointCount(t, clientV2, name, tenancy, 1)
		}
	}

	// Check relationships
	for _, ship := range ships {
		t.Run("relationship: "+ship.String(), func(t *testing.T) {
			var (
				wrk  = ship.Caller
				dest = ship.Destination
			)

			clusterPrefix := clusterPrefixForDestination(dest)

			asserter.DestinationEndpointStatus(t, wrk, clusterPrefix+".", "HEALTHY", 1)
			if dest.LocalPort > 0 {
				asserter.HTTPServiceEchoes(t, wrk, dest.LocalPort, "")
			}
			asserter.FortioFetch2FortioName(t, wrk, dest, cluster.Name, dest.ID)
		})
	}
}

type testBasicL4ImplicitDestinationsCreator struct {
	tenancies []*pbresource.Tenancy
}

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

	for i := range c.tenancies {
		c.topologyConfigAddNodes(t, cluster, nodeName, c.tenancies[i])
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

func (c testBasicL4ImplicitDestinationsCreator) topologyConfigAddNodes(
	t *testing.T,
	cluster *topology.Cluster,
	nodeName func() string,
	tenancy *pbresource.Tenancy,
) {
	clusterName := cluster.Name

	newID := func(name string, tenancy *pbresource.Tenancy) topology.ID {
		return topology.ID{
			Partition: tenancy.Partition,
			Namespace: tenancy.Namespace,
			Name:      name,
		}
	}

	serverNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: tenancy.Partition,
		Name:      nodeName(),
		Workloads: []*topology.Workload{
			topoutil.NewFortioWorkloadWithDefaults(
				clusterName,
				newID("static-server", tenancy),
				topology.NodeVersionV2,
				func(wrk *topology.Workload) {
					wrk.EnableTransparentProxy = true
				},
			),
		},
	}

	var impliedDestinations []*topology.Destination
	for _, ten := range c.tenancies {
		// For now we include all services in the same partition as implicit upstreams.
		if tenancy.Partition != ten.Partition {
			continue
		}
		impliedDestinations = append(impliedDestinations, &topology.Destination{
			ID:       newID("static-server", ten),
			PortName: "http",
		})
		impliedDestinations = append(impliedDestinations, &topology.Destination{
			ID:       newID("static-server", ten),
			PortName: "http2",
		})
	}

	clientNode := &topology.Node{
		Kind:      topology.NodeKindDataplane,
		Version:   topology.NodeVersionV2,
		Partition: tenancy.Partition,
		Name:      nodeName(),
		Workloads: []*topology.Workload{
			topoutil.NewFortioWorkloadWithDefaults(
				clusterName,
				newID("static-client", tenancy),
				topology.NodeVersionV2,
				func(wrk *topology.Workload) {
					wrk.EnableTransparentProxy = true
					wrk.ImpliedDestinations = impliedDestinations
				},
			),
		},
	}

	var sources []*pbauth.Source
	for _, ten := range c.tenancies {
		sources = append(sources, &pbauth.Source{
			IdentityName: "static-client",
			Namespace:    ten.Namespace,
			Partition:    ten.Partition,
		})
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
			Sources: sources,
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
