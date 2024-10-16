// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package upgrade

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test-integ/topoutil"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

// The commonTopo comprises 3 agent servers and 3 nodes to run workload
// - workload node 1: static-server
// - workload node 2: static-client
// - workload node 3 (disabled initially): static-server
//
// The post upgrade validation enables workload node 3 to test upgraded
// cluster
type commonTopo struct {
	Cfg *topology.Config

	Sprawl *sprawl.Sprawl
	Assert *topoutil.Asserter

	StaticServerSID topology.ID
	StaticClientSID topology.ID

	StaticServerWorkload *topology.Workload
	StaticClientWorkload *topology.Workload

	// node index of static-server one
	StaticServerInstOne int
	// node index of static-server two
	StaticServerInstTwo int
}

func NewCommonTopo(t *testing.T) *commonTopo {
	t.Helper()
	return newCommonTopo(t)
}

func newCommonTopo(t *testing.T) *commonTopo {
	t.Helper()

	ct := &commonTopo{}
	staticServerSID := topology.NewID("static-server", "default", "default")
	staticClientSID := topology.NewID("static-client", "default", "default")

	cfg := &topology.Config{
		Images: topology.Images{
			// ConsulEnterprise: "hashicorp/consul-enterprise:local",
		},
		Networks: []*topology.Network{
			{Name: "dc1"},
		},
		Clusters: []*topology.Cluster{
			{
				Name: "dc1",
				Nodes: []*topology.Node{
					{
						Kind:   topology.NodeKindServer,
						Images: utils.LatestImages(),
						Name:   "dc1-server1",
						Addresses: []*topology.Address{
							{Network: "dc1"},
						},
						Meta: map[string]string{
							"build": "0.0.1",
						},
					},
					{
						Kind:   topology.NodeKindServer,
						Images: utils.LatestImages(),
						Name:   "dc1-server2",
						Addresses: []*topology.Address{
							{Network: "dc1"},
						},
						Meta: map[string]string{
							"build": "0.0.1",
						},
					},
					{
						Kind:   topology.NodeKindServer,
						Images: utils.LatestImages(),
						Name:   "dc1-server3",
						Addresses: []*topology.Address{
							{Network: "dc1"},
						},
						Meta: map[string]string{
							"build": "0.0.1",
						},
					},
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
						Workloads: []*topology.Workload{
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
						Kind:      api.ProxyDefaults,
						Name:      "global",
						Partition: topoutil.ConfigEntryPartition("default"),
						Config: map[string]any{
							"protocol": "http",
						},
					},
					&api.ServiceConfigEntry{
						Kind:      api.ServiceDefaults,
						Name:      "static-server",
						Partition: topoutil.ConfigEntryPartition("default"),
					},
					&api.ServiceIntentionsConfigEntry{
						Kind:      api.ServiceIntentions,
						Name:      "static-server",
						Partition: topoutil.ConfigEntryPartition("default"),
						Sources: []*api.SourceIntention{
							{
								Name:   "static-client",
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

	ct.StaticServerInstOne = 3
	ct.StaticServerInstTwo = 5
	return ct
}

// PostUpgradeValidation - replace the existing static-server with a new
// instance; verify the connection between static-client and the new instance
func (ct *commonTopo) PostUpgradeValidation(t *testing.T) {
	t.Helper()
	t.Log("Take down old static-server")
	cfg := ct.Sprawl.Config()
	cluster := cfg.Cluster("dc1")
	cluster.Nodes[ct.StaticServerInstOne].Disabled = true //  client 1 -- static-server
	require.NoError(t, ct.Sprawl.RelaunchWithPhase(cfg, sprawl.LaunchPhaseRegular))
	// verify static-server is down
	ct.Assert.HTTPStatus(t, ct.StaticServerWorkload, ct.StaticServerWorkload.Port, 504)

	// Add a new static-server
	t.Log("Add a new static server")
	cfg = ct.Sprawl.Config()
	cluster = cfg.Cluster("dc1")
	cluster.Nodes[ct.StaticServerInstTwo].Disabled = false //  client 3 -- new static-server
	require.NoError(t, ct.Sprawl.RelaunchWithPhase(cfg, sprawl.LaunchPhaseRegular))
	// Ensure the static-client connected to the new static-server
	ct.Assert.FortioFetch2HeaderEcho(t, ct.StaticClientWorkload, &topology.Upstream{
		ID:        ct.StaticServerSID,
		LocalPort: 5000,
	})
}

// calls sprawltest.Launch followed by validating the connection between
// static-client and static-server
func (ct *commonTopo) Launch(t *testing.T) {
	t.Helper()
	if ct.Sprawl != nil {
		t.Fatalf("Launch must only be called once")
	}
	ct.Sprawl = sprawltest.Launch(t, ct.Cfg)
	ct.Assert = topoutil.NewAsserter(ct.Sprawl)

	staticServerWorkload := ct.Sprawl.Topology().Clusters["dc1"].WorkloadByID(
		topology.NewNodeID("dc1-client1", "default"),
		ct.StaticServerSID,
	)
	ct.Assert.HTTPStatus(t, staticServerWorkload, staticServerWorkload.Port, 200)

	staticClientWorkload := ct.Sprawl.Topology().Clusters["dc1"].WorkloadByID(
		topology.NewNodeID("dc1-client2", "default"),
		ct.StaticClientSID,
	)
	ct.Assert.FortioFetch2HeaderEcho(t, staticClientWorkload, &topology.Upstream{
		ID:        ct.StaticServerSID,
		LocalPort: 5000,
	})

	ct.StaticServerWorkload = staticServerWorkload
	ct.StaticClientWorkload = staticClientWorkload
}
