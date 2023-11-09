// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	// "fmt"
	// "net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"

	"github.com/hashicorp/consul/test-integ/topoutil"
)

// Test_Snapshot_Restore_Agentless verifies consul agent can continue
// to push envoy confgi after restoring from a snapshot.
//
//   - This test is to detect server agent frozen after restoring from a snapshot
//     (https://github.com/hashicorp/consul/pull/18636)
//
//   - This bug only appeared in agentless mode
//
// Steps:
//  1. The test spins up a one-server cluster with static-server and static-client.
//  2. A snapshot is taken and the cluster is restored from the snapshot
//  3. A new static-server replaces the old one
//  4. At the end, we assert the static-client's upstream is updated with the
//     new static-server
func Test_Snapshot_Restore_Agentless(t *testing.T) {
	t.Parallel()

	staticServerSID := topology.NewServiceID("static-server", "default", "default")
	staticClientSID := topology.NewServiceID("static-client", "default", "default")

	clu := &topology.Config{
		Images: utils.TargetImages(),
		Networks: []*topology.Network{
			{Name: "dc1"},
		},
		Clusters: []*topology.Cluster{
			{
				Name: "dc1",
				Nodes: []*topology.Node{
					{
						Kind: topology.NodeKindServer,
						// NOTE: uncomment the following lines to trigger the agent frozen bug
						// Images: topology.Images{
						// 	ConsulEnterprise: "hashicorp/consul-enterprise:1.16.1-ent",
						// },
						Name: "dc1-server1",
						Addresses: []*topology.Address{
							{Network: "dc1"},
						},
					},
					{
						Kind: topology.NodeKindDataplane,
						Name: "dc1-client1",
						Services: []*topology.Service{
							{
								ID:             staticServerSID,
								Image:          "docker.mirror.hashicorp.services/fortio/fortio",
								Port:           8080,
								EnvoyAdminPort: 19000,
								CheckTCP:       "127.0.0.1:8080",
								Command: []string{
									"server",
									"-http-port", "8080",
									"-redirect-port", "-disabled",
								},
							},
						},
					},
					{
						Kind: topology.NodeKindDataplane,
						Name: "dc1-client2",
						Services: []*topology.Service{
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
								Upstreams: []*topology.Upstream{
									{
										ID:        staticServerSID,
										LocalPort: 5000,
									},
								},
							},
						},
					},
					// Client3 for second static-server
					{
						Kind:     topology.NodeKindDataplane,
						Name:     "dc1-client3",
						Disabled: true,
						Services: []*topology.Service{
							{
								ID:             staticServerSID,
								Image:          "docker.mirror.hashicorp.services/fortio/fortio",
								Port:           8080,
								EnvoyAdminPort: 19000,
								CheckTCP:       "127.0.0.1:8080",
								Command: []string{
									"server",
									"-http-port", "8080",
									"-redirect-port", "-disabled",
								},
							},
						},
					},
				},
				Enterprise: utils.IsEnterprise(),
				InitialConfigEntries: []api.ConfigEntry{
					&api.ProxyConfigEntry{
						Kind: api.ProxyDefaults,
						Name: "global",
						Config: map[string]any{
							"protocol": "http",
						},
					},
					&api.ServiceConfigEntry{
						Kind: api.ServiceDefaults,
						Name: "static-server",
					},
					&api.ServiceIntentionsConfigEntry{
						Kind: api.ServiceIntentions,
						Name: "static-server",
						Sources: []*api.SourceIntention{
							{
								Name:   "static-client",
								Action: api.IntentionActionAllow,
							},
						},
					},
				},
			},
		},
	}
	sp := sprawltest.Launch(t, clu)
	asserter := topoutil.NewAsserter(sp)

	staticClient := sp.Topology().Clusters["dc1"].ServiceByID(
		topology.NewNodeID("dc1-client2", "default"),
		staticClientSID,
	)
	asserter.FortioFetch2HeaderEcho(t, staticClient, &topology.Upstream{
		ID:        staticServerSID,
		LocalPort: 5000,
	})
	staticServer := sp.Topology().Clusters["dc1"].ServiceByID(
		topology.NewNodeID("dc1-client1", "default"),
		staticServerSID,
	)
	asserter.HTTPStatus(t, staticServer, staticServer.Port, 200)

	t.Log("Take a snapshot of the cluster and restore ...")
	err := sp.SnapshotSave("dc1")
	require.NoError(t, err)

	// Shutdown existing static-server
	cfg := sp.Config()
	cluster := cfg.Cluster("dc1")
	cluster.Nodes[1].Disabled = true //  client 1 -- static-server
	require.NoError(t, sp.Relaunch(cfg))
	// verify static-server is down
	asserter.HTTPStatus(t, staticServer, staticServer.Port, 504)

	// Add a new static-server
	cfg = sp.Config()
	cluster = cfg.Cluster("dc1")
	cluster.Nodes[3].Disabled = false //  client 3 -- new static-server
	require.NoError(t, sp.Relaunch(cfg))

	// Ensure the static-client connected to the new static-server
	asserter.HTTPServiceEchoes(t, staticClient, staticClient.Port, "")
}
