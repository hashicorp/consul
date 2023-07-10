package peering

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/testingconsul/topology"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// NOTE: because AC6 needs to mutate the topo, we actually *DO NOT* share a topo

func TestAC6Failovers(t *testing.T) {
	if allowParallelCommonTopo {
		t.Parallel()
	}
	ct := NewCommonTopo(t)
	s := ac6FailoversSuite{}
	s.setup(t, ct)
	ct.Launch(t)
	s.test(t, ct)
}

type nodeKey struct {
	dc        string
	partition string
}

type ac6FailoversContext struct {
	clientSID topology.ServiceID
	serverSID topology.ServiceID

	// used to remove the node and trigger failover
	serverNode topology.NodeID
}

// note: unlike other *Suite structs that are per-peering direction,
// this one is special and does all directions itself, because the
// setup is not exactly symmetrical
type ac6FailoversSuite struct {
	ac6 map[nodeKey]ac6FailoversContext
}

var _ commonTopoSuite = (*ac6FailoversSuite)(nil)

func (s *ac6FailoversSuite) testName() string {
	return "ac6 failovers"
}

func (s *ac6FailoversSuite) setup(t *testing.T, ct *commonTopo) {
	// TODO: update setups to loop through a cluster's partitions+namespaces internally
	s.setupAC6Failovers(ct, ct.DC1, ct.DC2)
	s.setupAC6Failovers(ct, ct.DC2, ct.DC1)
	s.setupAC6FailoversDC3(ct, ct.DC3, ct.DC1, ct.DC2)
}

// dc1 is peered with dc2 and dc3.
// dc1 has an ac6-client in "default" and "part1" partitions (only default in OSS).
// ac6-client has a single upstream ac6-failover-svc in its respective partition^.
//
// ac6-failover-svc has the following failovers:
//   - peer-dc2-default
//   - peer-dc2-part1 (not in OSS)
//   - peer-dc3-default
//
// This setup is mirrored from dc2->dc1 as well
// (both dcs have dc3 as the last failover target)
//
// ^NOTE: There are no cross-partition upstreams because MeshGatewayMode = local
// and failover information gets stripped out by the mesh gateways so we
// can't test failovers.
func (s *ac6FailoversSuite) setupAC6Failovers(ct *commonTopo, clu, peerClu *topology.Cluster) {
	for _, part := range clu.Partitions {
		partition := part.Name

		// There is a peering per partition in the peered cluster
		var peers []string
		for _, peerPart := range peerClu.Partitions {
			peers = append(peers, LocalPeerName(peerClu, peerPart.Name))
		}

		// Make an HTTP server with various failover targets
		serverSID := topology.ServiceID{
			Name:      "ac6-failover-svc",
			Partition: partition,
		}
		server := NewFortioServiceWithDefaults(
			clu.Datacenter,
			serverSID,
			nil,
		)
		// Export to all known peers
		ct.ExportService(clu, partition,
			api.ExportedService{
				Name: server.ID.Name,
				Consumers: func() []api.ServiceConsumer {
					var consumers []api.ServiceConsumer
					for _, peer := range peers {
						consumers = append(consumers, api.ServiceConsumer{
							Peer: peer,
						})
					}
					return consumers
				}(),
			},
		)
		serverNode := ct.AddServiceNode(clu, serviceExt{Service: server})

		clu.InitialConfigEntries = append(clu.InitialConfigEntries,
			&api.ServiceConfigEntry{
				Kind:      api.ServiceDefaults,
				Name:      server.ID.Name,
				Partition: ConfigEntryPartition(partition),
				Protocol:  "http",
			},
			&api.ServiceResolverConfigEntry{
				Kind:      api.ServiceResolver,
				Name:      server.ID.Name,
				Partition: ConfigEntryPartition(partition),
				Failover: map[string]api.ServiceResolverFailover{
					"*": {
						Targets: func() []api.ServiceResolverFailoverTarget {
							// Make a failover target for every partition in the peer cluster
							var targets []api.ServiceResolverFailoverTarget
							for _, peer := range peers {
								targets = append(targets, api.ServiceResolverFailoverTarget{
									Peer: peer,
								})
							}
							// Just hard code default partition for dc3, since the exhaustive
							// testing will be done against dc2.
							targets = append(targets, api.ServiceResolverFailoverTarget{
								Peer: "peer-dc3-default",
							})
							return targets
						}(),
					},
				},
			},
		)

		// Make client which will dial server
		clientSID := topology.ServiceID{
			Name:      "ac6-client",
			Partition: partition,
		}
		client := NewFortioServiceWithDefaults(
			clu.Datacenter,
			clientSID,
			func(s *topology.Service) {
				// Upstream per partition
				s.Upstreams = []*topology.Upstream{
					{
						ID: topology.ServiceID{
							Name:      server.ID.Name,
							Partition: part.Name,
						},
						LocalPort: 5000,
						// exposed so we can hit it directly
						// TODO: we shouldn't do this; it's not realistic
						LocalAddress: "0.0.0.0",
					},
				}
			},
		)
		ct.ExportService(clu, partition,
			api.ExportedService{
				Name: client.ID.Name,
				Consumers: func() []api.ServiceConsumer {
					var consumers []api.ServiceConsumer
					// Export to each peer
					for _, peer := range peers {
						consumers = append(consumers, api.ServiceConsumer{
							Peer: peer,
						})
					}
					return consumers
				}(),
			},
		)
		ct.AddServiceNode(clu, serviceExt{Service: client})

		clu.InitialConfigEntries = append(clu.InitialConfigEntries,
			&api.ServiceConfigEntry{
				Kind:      api.ServiceDefaults,
				Name:      client.ID.Name,
				Partition: ConfigEntryPartition(partition),
				Protocol:  "http",
			},
		)

		// Add intention allowing local and peered clients to call server
		clu.InitialConfigEntries = append(clu.InitialConfigEntries,
			&api.ServiceIntentionsConfigEntry{
				Kind:      api.ServiceIntentions,
				Name:      server.ID.Name,
				Partition: ConfigEntryPartition(partition),
				// SourceIntention for local client and peered clients
				Sources: func() []*api.SourceIntention {
					ixns := []*api.SourceIntention{
						{
							Name:      client.ID.Name,
							Partition: ConfigEntryPartition(part.Name),
							Action:    api.IntentionActionAllow,
						},
					}
					for _, peer := range peers {
						ixns = append(ixns, &api.SourceIntention{
							Name:   client.ID.Name,
							Peer:   peer,
							Action: api.IntentionActionAllow,
						})
					}
					return ixns
				}(),
			},
		)
		if s.ac6 == nil {
			s.ac6 = map[nodeKey]ac6FailoversContext{}
		}
		s.ac6[nodeKey{clu.Datacenter, partition}] = struct {
			clientSID  topology.ServiceID
			serverSID  topology.ServiceID
			serverNode topology.NodeID
		}{
			clientSID:  clientSID,
			serverSID:  serverSID,
			serverNode: serverNode.ID(),
		}
	}
}

func (s *ac6FailoversSuite) setupAC6FailoversDC3(ct *commonTopo, clu, peer1, peer2 *topology.Cluster) {
	var peers []string
	for _, part := range peer1.Partitions {
		peers = append(peers, LocalPeerName(peer1, part.Name))
	}
	for _, part := range peer2.Partitions {
		peers = append(peers, LocalPeerName(peer2, part.Name))
	}

	partition := "default"

	// Make an HTTP server
	server := NewFortioServiceWithDefaults(
		clu.Datacenter,
		topology.ServiceID{
			Name:      "ac6-failover-svc",
			Partition: partition,
		},
		nil,
	)

	ct.AddServiceNode(clu, serviceExt{
		Service: server,
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      server.ID.Name,
			Partition: ConfigEntryPartition(partition),
			Protocol:  "http",
		},
		Intentions: &api.ServiceIntentionsConfigEntry{
			Kind:      api.ServiceIntentions,
			Name:      server.ID.Name,
			Partition: ConfigEntryPartition(partition),
			Sources: func() []*api.SourceIntention {
				var ixns []*api.SourceIntention
				for _, peer := range peers {
					ixns = append(ixns, &api.SourceIntention{
						Name:   "ac6-client",
						Peer:   peer,
						Action: api.IntentionActionAllow,
					})
				}
				return ixns
			}(),
		},
		Exports: func() []api.ServiceConsumer {
			var consumers []api.ServiceConsumer
			for _, peer := range peers {
				consumers = append(consumers, api.ServiceConsumer{
					Peer: peer,
				})
			}
			return consumers
		}(),
	})
}

func (s *ac6FailoversSuite) test(t *testing.T, ct *commonTopo) {
	dc1 := ct.Sprawl.Topology().Clusters["dc1"]
	dc2 := ct.Sprawl.Topology().Clusters["dc2"]

	type testcase struct {
		name      string
		cluster   *topology.Cluster
		peer      *topology.Cluster
		partition string
	}
	tcs := []testcase{
		{
			name:      "dc1 default partition failovers",
			cluster:   dc1,
			peer:      dc2, // dc3 is hardcoded
			partition: "default",
		},
		{
			name:      "dc1 part1 partition failovers",
			cluster:   dc1,
			peer:      dc2, // dc3 is hardcoded
			partition: "part1",
		},
		{
			name:      "dc2 default partition failovers",
			cluster:   dc2,
			peer:      dc1, // dc3 is hardcoded
			partition: "default",
		},
		{
			name:      "dc2 part1 partition failovers",
			cluster:   dc2,
			peer:      dc1, // dc3 is hardcoded
			partition: "part1",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// NOTE: *not parallel* because we mutate resources that are shared
			// between test cases (disable/enable nodes)
			if !utils.IsEnterprise() && tc.partition != "default" {
				t.Skip("skipping enterprise test")
			}
			partition := tc.partition
			clu := tc.cluster
			peerClu := tc.peer

			svcs := clu.ServicesByID(s.ac6[nodeKey{clu.Datacenter, partition}].clientSID)
			require.Len(t, svcs, 1, "expected exactly one client in datacenter")

			serverSID := s.ac6[nodeKey{clu.Datacenter, partition}].serverSID
			serverSID.Normalize()

			client := svcs[0]
			require.Len(t, client.Upstreams, 1, "expected one upstream for client")

			u := client.Upstreams[0]
			ct.Assert.CatalogServiceExists(t, clu.Name, u.ID.Name, utils.CompatQueryOpts(&api.QueryOptions{
				Partition: u.ID.Partition,
			}))

			t.Cleanup(func() {
				cfg := ct.Sprawl.Config()
				for _, part := range clu.Partitions {
					EnableNode(t, cfg, clu.Name, s.ac6[nodeKey{clu.Datacenter, part.Name}].serverNode)
				}
				for _, part := range peerClu.Partitions {
					EnableNode(t, cfg, peerClu.Name, s.ac6[nodeKey{peerClu.Datacenter, part.Name}].serverNode)
				}
				require.NoError(t, ct.Sprawl.Relaunch(cfg))
			})

			fmt.Println("### preconditions")
			// TODO: deduce this number, instead of hard-coding
			nFailoverTargets := 4
			// in OSS, we don't have failover targets for non-default partitions
			if !utils.IsEnterprise() {
				nFailoverTargets = 3
			}
			for i := 0; i < nFailoverTargets; i++ {
				ct.Assert.UpstreamEndpointStatus(t, client, fmt.Sprintf("failover-target~%d~%s", i, clusterPrefix(u, clu.Datacenter)), "HEALTHY", 1)
			}

			ct.Assert.FortioFetch2FortioName(t, client, u, clu.Name, serverSID)

			if t.Failed() {
				t.Fatalf("failed preconditions")
			}

			fmt.Println("### Failover to peer target")
			cfg := ct.Sprawl.Config()
			DisableNode(t, cfg, clu.Name, s.ac6[nodeKey{clu.Datacenter, partition}].serverNode)
			require.NoError(t, ct.Sprawl.Relaunch(cfg))
			// Clusters for imported services rely on outlier detection for
			// failovers, NOT eds_health_status. This means that killing the
			// node above does not actually make the envoy cluster UNHEALTHY
			// so we do not assert for it.
			expectUID := topology.ServiceID{
				Name:      u.ID.Name,
				Partition: "default",
			}
			expectUID.Normalize()
			ct.Assert.FortioFetch2FortioName(t, client, u, peerClu.Name, expectUID)

			if utils.IsEnterprise() {
				fmt.Println("### Failover to peer target in non-default partition")
				cfg = ct.Sprawl.Config()
				DisableNode(t, cfg, clu.Name, s.ac6[nodeKey{clu.Datacenter, partition}].serverNode)
				DisableNode(t, cfg, peerClu.Name, s.ac6[nodeKey{peerClu.Datacenter, "default"}].serverNode)
				require.NoError(t, ct.Sprawl.Relaunch(cfg))
				// Retry until outlier_detection deems the cluster
				// unhealthy and fails over to peer part1.
				expectUID = topology.ServiceID{
					Name:      u.ID.Name,
					Partition: "part1",
				}
				expectUID.Normalize()
				ct.Assert.FortioFetch2FortioName(t, client, u, peerClu.Name, expectUID)
			}

			fmt.Println("### Failover to dc3 peer target")
			cfg = ct.Sprawl.Config()
			DisableNode(t, cfg, clu.Name, s.ac6[nodeKey{clu.Datacenter, partition}].serverNode)
			// Disable all partitions for peer
			for _, part := range peerClu.Partitions {
				DisableNode(t, cfg, peerClu.Name, s.ac6[nodeKey{peerClu.Datacenter, part.Name}].serverNode)
			}
			require.NoError(t, ct.Sprawl.Relaunch(cfg))
			// This will retry until outlier_detection deems the cluster
			// unhealthy and fails over to dc3.
			expectUID = topology.ServiceID{
				Name:      u.ID.Name,
				Partition: "default",
			}
			expectUID.Normalize()
			ct.Assert.FortioFetch2FortioName(t, client, u, "dc3", expectUID)
		})
	}
}

func clusterPrefix(u *topology.Upstream, dc string) string {
	u.ID.Normalize()
	switch u.ID.Partition {
	case "default":
		return fmt.Sprintf("%s.%s.%s.internal", u.ID.Name, u.ID.Namespace, dc)
	default:
		return fmt.Sprintf("%s.%s.%s.%s.internal-v1", u.ID.Name, u.ID.Namespace, u.ID.Partition, dc)
	}
}
