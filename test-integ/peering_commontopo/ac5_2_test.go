package peering

import (
	"fmt"
	"time"

	"testing"

	"github.com/hashicorp/consul-topology/topology"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
)

// This tests prepared query failover across 3 clusters. The following test case covers:
// 1. Create a prepared query with failover target to DC2 and DC3 cluster.
// Test will fail if not run in parallel
type preparedQueryFailoverSuite struct {
	DC   string
	Peer string

	nodeServer   []*topology.Node
	dc3ServerSID topology.ServiceID

	serverSID    topology.ServiceID
	clientSID    topology.ServiceID
	dc3APIClient *api.Client
}

var (
	preparedQueryFailoverSuites []*preparedQueryFailoverSuite = []*preparedQueryFailoverSuite{
		{DC: "dc1", Peer: "dc2"},
		{DC: "dc2", Peer: "dc1"},
	}
	query *api.PreparedQuery
	def   *api.PreparedQueryDefinition
)

func TestPreparedQueryFailoverSuite(t *testing.T) {
	var querySuite *preparedQueryFailoverSuite

	if !*FlagNoReuseCommonTopo {
		t.Skip("NoReuseCommonTopo unset")
	}
	t.Parallel()
	ct := NewCommonTopo(t)

	for _, s := range preparedQueryFailoverSuites {
		s.setup(t, ct)
		querySuite = s
	}
	// setup dc3 cluster and disable server node in dc2
	querySuite.setupForDC3(ct, ct.DC3, ct.DC1, ct.DC2)
	ct.Launch(t)

	for _, s := range preparedQueryFailoverSuites {
		s := s
		t.Run(fmt.Sprintf("%s_%s", s.DC, s.Peer), func(t *testing.T) {
			t.Parallel()
			s.test(t, ct)
		})
	}
}

func (s *preparedQueryFailoverSuite) testName() string {
	return "prepared query failover assertions"
}

// creates clients in s.DC and servers in s.Peer
func (s *preparedQueryFailoverSuite) setup(t *testing.T, ct *commonTopo) {
	clu := ct.ClusterByDatacenter(t, s.DC)
	peerClu := ct.ClusterByDatacenter(t, s.Peer)

	// TODO: handle all partitions
	partition := "default"
	peer := LocalPeerName(peerClu, "default")
	cluPeerName := LocalPeerName(clu, "default")

	serverSID := topology.ServiceID{
		Name:      "ac5-server-http",
		Partition: partition,
	}

	// Make client which will dial server
	clientSID := topology.ServiceID{
		Name:      "ac5-client-http",
		Partition: partition,
	}

	// disable service mesh for client in DC2
	fmt.Println("Creating client in cluster: ", s.DC)
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
			peerClu.Datacenter,
			serverSID,
			nil,
		),
		Exports: []api.ServiceConsumer{{Peer: cluPeerName}},
	}

	serverNode := ct.AddServiceNode(peerClu, server)

	s.clientSID = clientSID
	s.serverSID = serverSID
	s.nodeServer = append(s.nodeServer, serverNode)
}

func (s *preparedQueryFailoverSuite) setupForDC3(ct *commonTopo, clu, peer1, peer2 *topology.Cluster) {
	var (
		peers     []string
		partition = "default"
	)
	peers = append(peers, LocalPeerName(peer1, "default"))
	peers = append(peers, LocalPeerName(peer2, "default"))

	serverSID := topology.ServiceID{
		Name:      "ac5-server-http",
		Partition: partition,
	}

	clientSID := topology.ServiceID{
		Name:      "ac5-client-http",
		Partition: partition,
	}

	// disable service mesh for client in DC3
	fmt.Println("Creating client in cluster: ", clu.Datacenter)
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

	// Make HTTP server
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

	s.dc3ServerSID = serverSID
	s.nodeServer = append(s.nodeServer, serverNode)
}

func (s *preparedQueryFailoverSuite) test(t *testing.T, ct *commonTopo) {
	var partition = "default"
	dc := ct.Sprawl.Topology().Clusters[s.DC]
	apiClient := ct.APIClientForCluster(t, dc)

	dc3 := ct.Sprawl.Topology().Clusters[ct.DC3.Name]
	s.dc3APIClient = ct.APIClientForCluster(t, dc3)

	// get peer names for dc2 & dc3 cluster respectively
	var peers []string
	peers = append(peers, LocalPeerName(ct.DC2, partition))
	peers = append(peers, LocalPeerName(ct.DC3, partition))

	s.testServiceHealthCheck(t, apiClient)
	s.testQueryTwoFailovers(t, apiClient, ct, peers)
}

func (s *preparedQueryFailoverSuite) testServiceHealthCheck(t *testing.T, apiClient *api.Client) {
	t.Run("validate service health in cluster", func(t *testing.T) {
		// preconditions check
		assertServiceHealth(t, s.dc3APIClient, s.serverSID.Name, 1) // validate server is healthy in dc3 cluster
		assertServiceHealth(t, apiClient, s.serverSID.Name, 1)
	})
}

func (s *preparedQueryFailoverSuite) testQueryTwoFailovers(t *testing.T, c *api.Client, ct *commonTopo, peers []string) {
	if s.DC == "dc2" {
		// TO-DO: currently failing
		t.Skip()
	}

	t.Run("prepared query with two failovers", func(t *testing.T) {
		var err error

		// disable dc1 & dc2 and relaunch topology
		cfg := ct.Sprawl.Config()
		require.NoError(t, disableNodeInCluster(t, cfg, ct.DC1.Name, s.serverSID.Name, true)) //dc1
		require.NoError(t, disableNodeInCluster(t, cfg, ct.DC2.Name, s.serverSID.Name, true)) //dc2
		require.NoError(t, ct.Sprawl.Relaunch(cfg))

		// assert server health status
		dc2 := ct.Sprawl.Topology().Clusters[ct.DC2.Name]
		dc2APIClient := ct.APIClientForCluster(t, dc2)

		assertServiceHealth(t, c, s.serverSID.Name, 0)              //dc1
		assertServiceHealth(t, dc2APIClient, s.serverSID.Name, 0)   //dc2
		assertServiceHealth(t, s.dc3APIClient, s.serverSID.Name, 1) //dc3 is status should be unchanged

		// create prepared query definition
		def = &api.PreparedQueryDefinition{
			Name: "ac5-prepared-query",
			Service: api.ServiceQuery{
				Service:     s.serverSID.Name,
				Partition:   ConfigEntryPartition(s.serverSID.Partition),
				OnlyPassing: true,
				// create failover for peer in dc2 and dc3 cluster
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

		query = c.PreparedQuery()
		def.ID, _, err = query.Create(def, nil)
		require.NoError(t, err)

		// Read registered query
		queryDef, _, err := query.Get(def.ID, nil)
		require.NoError(t, err)
		require.Len(t, queryDef, 1, "expected exactly 1 prepared query")
		require.Equal(t, 2, len(queryDef[0].Service.Failover.Targets), "expected 2 failover targets for dc2 & dc3")
		fmt.Println("PreparedQuery with failover created successfully.")

		// expected outcome should show 2 failovers
		queryResult, _, err := query.Execute(def.ID, nil)
		require.NoError(t, err)
		require.Equal(t, 2, queryResult.Failovers, "expected 2 failovers to dc3")
		// should failover to peer in DC2 cluster
		require.Equal(t, ct.DC3.Datacenter, queryResult.Nodes[0].Node.Datacenter)
		// failover to nearest cluster
		require.Equal(t, "peer-dc3-default", queryResult.Nodes[0].Checks[0].PeerName)
	})
}

// disableNodeInCluster disables node in specified datacenter
func disableNodeInCluster(t *testing.T, cfg *topology.Config, clusterName, serviceName string, status bool) error {
	nodes := cfg.Cluster(clusterName).Nodes
	serverNode := fmt.Sprintf("%s-%s", clusterName, serviceName)
	for _, node := range nodes {
		if node.Name == serverNode {
			node.Disabled = status
			fmt.Printf("Node: %s Node Disabled: %v Cluster: %s\n", node.Name, node.Disabled, node.Cluster)
			return nil
		}
	}
	return fmt.Errorf("Failed to disable node")
}

// assertServiceHealth checks that a service health status before running tests
func assertServiceHealth(t *testing.T, cl *api.Client, serverSVC string, count int) {
	t.Helper()
	retry.RunWith(&retry.Timer{Timeout: time.Second * 30, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
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
