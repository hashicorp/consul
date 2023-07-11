package peering

import (
	"fmt"
	"time"

	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testingconsul/topology"
	"github.com/stretchr/testify/require"
)

// 1. Setup: put health service instances in each of the 3 clusters and create the PQ in one of them
// 2. Execute the PQ: Validate that failover count == 0 and that the pq results come from the local cluster
// 3. Register a failing TTL health check with the agent managing the service instance in the local cluster
// 4. Execute the PQ: Validate that failover count == 1 and that the pq results come from the first failover target peer
// 5. Register a failing TTL health check with the agent managing the service instance in the first failover peer
// 6. Execute the PQ: Validate that failover count == 2 and that the pq results come from the second failover target
// 7. Delete failing health check from step 5
// 8. Repeat step 4
// 9. Delete failing health check from step 3
// 10. Repeat step 2
type preparedQueryFailoverSuite struct {
	clientSID  topology.ServiceID
	serverSID  topology.ServiceID
	nodeServer topology.NodeID
	ct         *commonTopo
}

var ac5Context = make(map[nodeKey]preparedQueryFailoverSuite)

func TestPreparedQueryFailoverSuite(t *testing.T) {
	t.Parallel()
	s := preparedQueryFailoverSuite{}
	ct := NewCommonTopo(t)
	s.ct = ct

	s.setup(t, ct)
	ct.Launch(t)
	s.test(t, ct)
}

func (s *preparedQueryFailoverSuite) testName() string {
	return "prepared query failover assertions"
}

func (s *preparedQueryFailoverSuite) setup(t *testing.T, ct *commonTopo) {
	s.setupDC(ct, ct.DC1, ct.DC2)
	s.setupDC(ct, ct.DC2, ct.DC1)
	s.setupDC3(ct, ct.DC3, ct.DC1, ct.DC2)
}

func (s *preparedQueryFailoverSuite) setupDC(ct *commonTopo, clu, peerClu *topology.Cluster) {
	// TODO: handle all partitions
	partition := "default"
	peer := LocalPeerName(peerClu, partition)

	serverSID := topology.ServiceID{
		Name:      "ac5-server-http",
		Partition: partition,
	}

	clientSID := topology.ServiceID{
		Name:      "ac5-client-http",
		Partition: partition,
	}

	client := serviceExt{
		Service: NewFortioServiceWithDefaults(
			clu.Datacenter,
			clientSID,
			func(s *topology.Service) {
				s.EnvoyAdminPort = 0
				s.DisableServiceMesh = true
			},
		),
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      clientSID.Name,
			Partition: ConfigEntryPartition(clientSID.Partition),
			Protocol:  "http",
		},
		Exports: []api.ServiceConsumer{{Peer: peer}},
	}

	ct.AddServiceNode(clu, client)

	server := serviceExt{
		Service: NewFortioServiceWithDefaults(
			clu.Datacenter,
			serverSID,
			nil,
		),
		Exports: []api.ServiceConsumer{{Peer: peer}},
	}
	serverNode := ct.AddServiceNode(clu, server)

	ac5Context[nodeKey{clu.Datacenter, partition}] = preparedQueryFailoverSuite{
		clientSID:  clientSID,
		serverSID:  serverSID,
		nodeServer: serverNode.ID(),
	}
}

func (s *preparedQueryFailoverSuite) setupDC3(ct *commonTopo, clu, peer1, peer2 *topology.Cluster) {
	var (
		peers     []string
		partition = "default"
	)
	peers = append(peers, LocalPeerName(peer1, partition), LocalPeerName(peer2, partition))

	serverSID := topology.ServiceID{
		Name:      "ac5-server-http",
		Partition: partition,
	}

	clientSID := topology.ServiceID{
		Name:      "ac5-client-http",
		Partition: partition,
	}

	// disable service mesh for client in DC3
	client := serviceExt{
		Service: NewFortioServiceWithDefaults(
			clu.Datacenter,
			clientSID,
			func(s *topology.Service) {
				s.EnvoyAdminPort = 0
				s.DisableServiceMesh = true
			},
		),
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      clientSID.Name,
			Partition: ConfigEntryPartition(clientSID.Partition),
			Protocol:  "http",
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
	}

	ct.AddServiceNode(clu, client)

	server := serviceExt{
		Service: NewFortioServiceWithDefaults(
			clu.Datacenter,
			serverSID,
			nil,
		),
		Exports: func() []api.ServiceConsumer {
			var consumers []api.ServiceConsumer
			for _, peer := range peers {
				consumers = append(consumers, api.ServiceConsumer{
					Peer: peer,
				})
			}
			return consumers
		}(),
	}

	serverNode := ct.AddServiceNode(clu, server)

	ac5Context[nodeKey{clu.Datacenter, partition}] = preparedQueryFailoverSuite{
		clientSID:  clientSID,
		serverSID:  serverSID,
		nodeServer: serverNode.ID(),
	}
}

func (s *preparedQueryFailoverSuite) createPreparedQuery(t *testing.T, c *api.Client, serviceName, partition string) (*api.PreparedQueryDefinition, *api.PreparedQuery) {
	var (
		peers []string
		err   error
	)
	peers = append(peers, LocalPeerName(s.ct.DC2, partition), LocalPeerName(s.ct.DC3, partition))

	def := &api.PreparedQueryDefinition{
		Name: "ac5-prepared-query",
		Service: api.ServiceQuery{
			Service:     serviceName,
			Partition:   ConfigEntryPartition(partition),
			OnlyPassing: true,
			Failover: api.QueryFailoverOptions{
				Targets: func() []api.QueryFailoverTarget {
					var queryFailoverTargets []api.QueryFailoverTarget
					for _, peer := range peers {
						queryFailoverTargets = append(queryFailoverTargets, api.QueryFailoverTarget{
							Peer: peer,
						})
					}
					return queryFailoverTargets
				}(),
			},
		},
	}

	query := c.PreparedQuery()
	def.ID, _, err = query.Create(def, nil)
	require.NoError(t, err, "error creating prepared query in cluster")

	return def, query
}

func (s *preparedQueryFailoverSuite) test(t *testing.T, ct *commonTopo) {
	partition := "default"
	dc1 := ct.Sprawl.Topology().Clusters[ct.DC1.Name]
	dc2 := ct.Sprawl.Topology().Clusters[ct.DC2.Name]
	dc3 := ct.Sprawl.Topology().Clusters[ct.DC3.Name]

	type testcase struct {
		cluster       *topology.Cluster
		peer          *topology.Cluster
		targetCluster *topology.Cluster
	}
	tcs := []testcase{
		{
			cluster:       dc1,
			peer:          dc2,
			targetCluster: dc3,
		},
	}
	for _, tc := range tcs {
		client := ct.APIClientForCluster(t, tc.cluster)

		t.Run(s.testName(), func(t *testing.T) {
			svc := ac5Context[nodeKey{tc.cluster.Name, partition}]
			require.NotNil(t, svc.serverSID.Name, "expected service name to not be nil")
			require.NotNil(t, svc.nodeServer, "expected node server to not be nil")

			assertServiceHealth(t, client, svc.serverSID.Name, 1)
			def, _ := s.createPreparedQuery(t, client, svc.serverSID.Name, partition)
			s.testPreparedQueryZeroFailover(t, client, def, tc.cluster)
			s.testPreparedQuerySingleFailover(t, client, def, tc.cluster, tc.peer, partition)
			s.testPreparedQueryTwoFailovers(t, client, def, tc.cluster, tc.peer, tc.targetCluster, partition)

			// delete failing health check in peer cluster & validate single failover
			s.testPQSingleFailover(t, client, def, tc.cluster, tc.peer, partition)
			// delete failing health check in cluster & validate zero failover
			s.testPQZeroFailover(t, client, def, tc.cluster, tc.peer, partition)
		})
	}
}

func (s *preparedQueryFailoverSuite) testPreparedQueryZeroFailover(t *testing.T, cl *api.Client, def *api.PreparedQueryDefinition, cluster *topology.Cluster) {
	t.Run(fmt.Sprintf("prepared query should not failover %s", cluster.Name), func(t *testing.T) {

		// Validate prepared query exists in cluster
		queryDef, _, err := cl.PreparedQuery().Get(def.ID, nil)
		require.NoError(t, err)
		require.Len(t, queryDef, 1, "expected 1 prepared query")
		require.Equal(t, 2, len(queryDef[0].Service.Failover.Targets), "expected 2 prepared query failover targets to dc2 and dc3")

		retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
			queryResult, _, err := cl.PreparedQuery().Execute(def.ID, nil)
			require.NoError(r, err)

			// expected outcome should show 0 failover
			require.Equal(r, 0, queryResult.Failovers, "expected 0 prepared query failover")
			require.Equal(r, cluster.Name, queryResult.Nodes[0].Node.Datacenter, "pq results should come from the local cluster")
		})
	})
}

func (s *preparedQueryFailoverSuite) testPreparedQuerySingleFailover(t *testing.T, cl *api.Client, def *api.PreparedQueryDefinition, cluster, peerClu *topology.Cluster, partition string) {
	t.Run(fmt.Sprintf("prepared query with single failover %s", cluster.Name), func(t *testing.T) {
		cfg := s.ct.Sprawl.Config()
		svc := ac5Context[nodeKey{cluster.Name, partition}]

		nodeCfg := DisableNode(t, cfg, cluster.Name, svc.nodeServer)
		require.NoError(t, s.ct.Sprawl.Relaunch(nodeCfg))

		// assert server health status
		assertServiceHealth(t, cl, svc.serverSID.Name, 0)

		// Validate prepared query exists in cluster
		queryDef, _, err := cl.PreparedQuery().Get(def.ID, nil)
		require.NoError(t, err)
		require.Len(t, queryDef, 1, "expected 1 prepared query")

		pqFailoverTargets := queryDef[0].Service.Failover.Targets
		require.Len(t, pqFailoverTargets, 2, "expected 2 prepared query failover targets to dc2 and dc3")

		retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
			queryResult, _, err := cl.PreparedQuery().Execute(def.ID, nil)
			require.NoError(r, err)

			require.Equal(r, 1, queryResult.Failovers, "expected 1 prepared query failover")
			require.Equal(r, peerClu.Name, queryResult.Nodes[0].Node.Datacenter, fmt.Sprintf("the pq results should originate from peer clu %s", peerClu.Name))
			require.Equal(r, pqFailoverTargets[0].Peer, queryResult.Nodes[0].Checks[0].PeerName,
				fmt.Sprintf("pq results should come from the first failover target peer %s", pqFailoverTargets[0].Peer))
		})
	})
}

func (s *preparedQueryFailoverSuite) testPreparedQueryTwoFailovers(t *testing.T, cl *api.Client, def *api.PreparedQueryDefinition, cluster, peerClu, targetCluster *topology.Cluster, partition string) {
	t.Run(fmt.Sprintf("prepared query with two failovers %s", cluster.Name), func(t *testing.T) {
		cfg := s.ct.Sprawl.Config()

		svc := ac5Context[nodeKey{peerClu.Name, partition}]

		cfg = DisableNode(t, cfg, peerClu.Name, svc.nodeServer)
		require.NoError(t, s.ct.Sprawl.Relaunch(cfg))

		// assert server health status
		assertServiceHealth(t, cl, ac5Context[nodeKey{cluster.Name, partition}].serverSID.Name, 0) // cluster: failing
		assertServiceHealth(t, cl, svc.serverSID.Name, 0)                                          // peer cluster: failing

		queryDef, _, err := cl.PreparedQuery().Get(def.ID, nil)
		require.NoError(t, err)
		require.Len(t, queryDef, 1, "expected 1 prepared query")

		pqFailoverTargets := queryDef[0].Service.Failover.Targets
		require.Len(t, pqFailoverTargets, 2, "expected 2 prepared query failover targets to dc2 and dc3")

		retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
			queryResult, _, err := cl.PreparedQuery().Execute(def.ID, nil)
			require.NoError(r, err)
			require.Equal(r, 2, queryResult.Failovers, "expected 2 prepared query failover")

			require.Equal(r, targetCluster.Name, queryResult.Nodes[0].Node.Datacenter, fmt.Sprintf("the pq results should originate from cluster %s", targetCluster.Name))
			require.Equal(r, pqFailoverTargets[1].Peer, queryResult.Nodes[0].Checks[0].PeerName,
				fmt.Sprintf("pq results should come from the second failover target peer %s", pqFailoverTargets[1].Peer))
		})
	})
}

func (s *preparedQueryFailoverSuite) testPQSingleFailover(t *testing.T, cl *api.Client, def *api.PreparedQueryDefinition, cluster, peerClu *topology.Cluster, partition string) {
	t.Run(fmt.Sprintf("delete failing health check in %s and validate single failover %s", peerClu.Name, cluster.Name), func(t *testing.T) {
		cfg := s.ct.Sprawl.Config()

		svc := ac5Context[nodeKey{peerClu.Name, partition}]

		cfg = EnableNode(t, cfg, peerClu.Name, svc.nodeServer)
		require.NoError(t, s.ct.Sprawl.Relaunch(cfg))

		queryDef, _, err := cl.PreparedQuery().Get(def.ID, nil)
		require.NoError(t, err)

		pqFailoverTargets := queryDef[0].Service.Failover.Targets
		require.Len(t, pqFailoverTargets, 2, "expected 2 prepared query failover targets to dc2 and dc3")

		retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
			queryResult, _, err := cl.PreparedQuery().Execute(def.ID, nil)
			require.NoError(r, err)
			require.Equal(r, 1, queryResult.Failovers, "expected 1 prepared query failover")

			require.Equal(r, peerClu.Name, queryResult.Nodes[0].Node.Datacenter, fmt.Sprintf("the pq results should originate from cluster %s", peerClu.Name))
			require.Equal(r, pqFailoverTargets[0].Peer, queryResult.Nodes[0].Checks[0].PeerName,
				fmt.Sprintf("pq results should come from the second failover target peer %s", pqFailoverTargets[0].Peer))
		})
	})
}

func (s *preparedQueryFailoverSuite) testPQZeroFailover(t *testing.T, cl *api.Client, def *api.PreparedQueryDefinition, cluster, peerClu *topology.Cluster, partition string) {
	t.Run(fmt.Sprintf("delete failing health check in %s and validate zero failover %s", cluster.Name, cluster.Name), func(t *testing.T) {
		cfg := s.ct.Sprawl.Config()

		svc := ac5Context[nodeKey{cluster.Name, partition}]

		cfg = EnableNode(t, cfg, cluster.Name, svc.nodeServer)
		require.NoError(t, s.ct.Sprawl.Relaunch(cfg))

		// assert server health status
		assertServiceHealth(t, cl, ac5Context[nodeKey{cluster.Name, partition}].serverSID.Name, 1) // cluster: passing
		assertServiceHealth(t, cl, svc.serverSID.Name, 1)                                          // peer cluster: passing

		queryDef, _, err := cl.PreparedQuery().Get(def.ID, nil)
		require.NoError(t, err)

		pqFailoverTargets := queryDef[0].Service.Failover.Targets
		require.Len(t, pqFailoverTargets, 2, "expected 2 prepared query failover targets to dc2 and dc3")

		retry.RunWith(&retry.Timer{Timeout: 10 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
			queryResult, _, err := cl.PreparedQuery().Execute(def.ID, nil)
			require.NoError(r, err)
			// expected outcome should show 0 failover
			require.Equal(r, 0, queryResult.Failovers, "expected 0 prepared query failover")
			require.Equal(r, cluster.Name, queryResult.Nodes[0].Node.Datacenter, "pq results should come from the local cluster")
		})
	})
}

// assertServiceHealth checks that a service health status before running tests
func assertServiceHealth(t *testing.T, cl *api.Client, serverSVC string, count int) {
	t.Helper()
	t.Log("validate service health in catalog")
	retry.RunWith(&retry.Timer{Timeout: time.Second * 20, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
		svcs, _, err := cl.Health().Service(
			serverSVC,
			"",
			true,
			nil,
		)
		require.NoError(r, err)
		require.Equal(r, count, len(svcs))
	})
}
