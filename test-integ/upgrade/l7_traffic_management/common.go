// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package l7_traffic_management

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"

	"github.com/hashicorp/consul/test-integ/topoutil"
)

type commonTopo struct {
	Cfg *topology.Config

	Sprawl *sprawl.Sprawl
	Assert *topoutil.Asserter

	StaticServerSID topology.ID
	StaticClientSID topology.ID

	StaticServerWorkload *topology.Workload
	StaticClientWorkload *topology.Workload
}

const (
	defaultNamespace = "default"
	defaultPartition = "default"
	dc1              = "dc1"
)

var (
	staticServerSID = topology.NewID("static-server", defaultNamespace, defaultPartition)
	staticClientSID = topology.NewID("static-client", defaultNamespace, defaultPartition)
)

func NewCommonTopo(t *testing.T) *commonTopo {
	t.Helper()
	return newCommonTopo(t)
}

// create below topology
// consul server
//   - consul server on node dc1-server1
//
// dataplane
//   - workload(fortio) static-server on node dc1-client1
//   - workload(fortio) static-client on node dc1-client2 with destination to static-server
//   - static-client, static-server are registered at 2 agentless nodes.
//
// Intentions
//   - static-client has destination to static-server
func newCommonTopo(t *testing.T) *commonTopo {
	t.Helper()

	ct := &commonTopo{}

	cfg := &topology.Config{
		Images: topology.Images{
			// ConsulEnterprise: "hashicorp/consul-enterprise:local",
		},
		Networks: []*topology.Network{
			{Name: dc1},
		},
		Clusters: []*topology.Cluster{
			{
				Name: dc1,
				Nodes: []*topology.Node{
					// consul server on dc1-server1
					{
						Kind:   topology.NodeKindServer,
						Images: utils.LatestImages(),
						Name:   "dc1-server1",
						Addresses: []*topology.Address{
							{Network: dc1},
						},
						Meta: map[string]string{
							"build": "0.0.1",
						},
					},
					// static-server-v1 on dc1-client1
					{
						Kind: topology.NodeKindDataplane,
						Name: "dc1-client1",
						Workloads: []*topology.Workload{
							{
								ID:             staticServerSID,
								Image:          "docker.mirror.hashicorp.services/fortio/fortio",
								Port:           8080,
								EnvoyAdminPort: 19000,
								CheckTCP:       "127.0.0.1:8080",
								Meta:           map[string]string{"version": "v2"},
								Env: []string{
									"FORTIO_NAME=" + dc1 + "::" + staticServerSID.String(),
								},
								Command: []string{
									"server",
									"-http-port", "8080",
									"-redirect-port", "-disabled",
								},
							},
						},
					},
					// static-client on dc1-client2 with destination to static-server
					{
						Kind: topology.NodeKindDataplane,
						Name: "dc1-client2",
						Workloads: []*topology.Workload{
							{
								ID:             staticClientSID,
								Image:          "docker.mirror.hashicorp.services/fortio/fortio",
								Port:           8080,
								EnvoyAdminPort: 19000,
								CheckTCP:       "127.0.0.1:8080",
								Command: []string{
									"server",
									"-http-port", "8080",
									"-redirect-port", "-disabled",
								},
								Destinations: []*topology.Destination{
									{
										ID:        staticServerSID, // static-server
										LocalPort: 5000,
									},
								},
							},
						},
					},
				},
				Enterprise: utils.IsEnterprise(),
				InitialConfigEntries: []api.ConfigEntry{
					&api.ProxyConfigEntry{
						Kind:      api.ProxyDefaults,
						Name:      "global",
						Partition: topoutil.ConfigEntryPartition("default"),
						Config: map[string]any{
							"protocol": "http",
						},
					},
					&api.ServiceConfigEntry{
						Kind:      api.ServiceDefaults,
						Name:      staticServerSID.Name,
						Partition: topoutil.ConfigEntryPartition("default"),
					},
					&api.ServiceIntentionsConfigEntry{
						Kind:      api.ServiceIntentions,
						Name:      staticServerSID.Name,
						Partition: topoutil.ConfigEntryPartition("default"),
						Sources: []*api.SourceIntention{
							{
								Name:   staticClientSID.Name,
								Action: api.IntentionActionAllow},
						},
					},
				},
			},
		},
	}

	ct.Cfg = cfg
	ct.StaticClientSID = staticClientSID
	ct.StaticServerSID = staticServerSID

	return ct
}

func (ct *commonTopo) Launch(t *testing.T) {
	t.Helper()
	if ct.Sprawl != nil {
		t.Fatalf("Launch must only be called once")
	}
	ct.Sprawl = sprawltest.Launch(t, ct.Cfg)
	ct.ValidateWorkloads(t)
}

// ValidateWorkloads validates below
// - static server, static client workloads are reachable and, static server, static client services are healthy
// - static client and its sidecar exists in catalog
// - envoy is running for static server, static client workloads
// - envoy cert uri is present in for static server, static client workloads
func (ct *commonTopo) ValidateWorkloads(t *testing.T) {

	t.Helper()
	ct.Assert = topoutil.NewAsserter(ct.Sprawl)
	cluster := ct.Sprawl.Topology().Clusters[dc1]
	node := cluster.Nodes[0]

	staticServerWorkload := cluster.WorkloadByID(
		topology.NewNodeID("dc1-client1", defaultPartition),
		ct.StaticServerSID,
	)
	ct.Assert.HTTPStatus(t, staticServerWorkload, staticServerWorkload.Port, 200)
	ct.Assert.HealthServiceEntries(t, cluster.Name, node, ct.StaticServerSID.Name, true, &api.QueryOptions{}, 1, 0)

	staticClientWorkload := cluster.WorkloadByID(
		topology.NewNodeID("dc1-client2", defaultPartition),
		ct.StaticClientSID,
	)

	ct.Assert.HealthServiceEntries(t, cluster.Name, node, ct.StaticClientSID.Name, true, &api.QueryOptions{}, 1, 0)

	// check the service exists in catalog
	svcs := cluster.WorkloadsByID(ct.StaticClientSID)
	client := svcs[0]
	upstream := client.Destinations[0]
	ct.Assert.CatalogServiceExists(t, cluster.Name, upstream.ID.Name, utils.CompatQueryOpts(&api.QueryOptions{
		Partition: upstream.ID.Partition,
		Namespace: upstream.ID.Namespace,
	}))
	ct.Assert.CatalogServiceExists(t, cluster.Name, fmt.Sprintf("%s-sidecar-proxy", upstream.ID.Name), utils.CompatQueryOpts(&api.QueryOptions{
		Partition: upstream.ID.Partition,
		Namespace: upstream.ID.Namespace,
	}))

	ct.StaticServerWorkload = staticServerWorkload
	ct.StaticClientWorkload = staticClientWorkload

	ct.Assert.AssertEnvoyRunningWithClient(t, ct.StaticServerWorkload)
	ct.Assert.AssertEnvoyRunningWithClient(t, ct.StaticClientWorkload)

	ct.Assert.AssertEnvoyPresentsCertURIWithClient(t, ct.StaticServerWorkload)
	ct.Assert.AssertEnvoyPresentsCertURIWithClient(t, ct.StaticClientWorkload)
}
