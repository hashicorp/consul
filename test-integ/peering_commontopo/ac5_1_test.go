package peering

import (
	"fmt"

	"testing"

	"github.com/hashicorp/consul-topology/topology"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/stretchr/testify/require"
)

// Test will fail if not run in parallel
type serviceMeshDisabledSuite struct {
	DC   string
	Peer string

	nodeClient topology.NodeID
	nodeServer topology.NodeID

	serverSID topology.ServiceID
	clientSID topology.ServiceID
}

var (
	serviceMeshDisabledSuites []*serviceMeshDisabledSuite = []*serviceMeshDisabledSuite{
		{DC: "dc1", Peer: "dc2"},
		{DC: "dc2", Peer: "dc1"},
	}
)

func TestServiceMeshDisabledSuite(t *testing.T) {
	if !*FlagNoReuseCommonTopo {
		t.Skip("NoReuseCommonTopo unset")
	}
	t.Parallel()
	ct := NewCommonTopo(t)
	for _, s := range serviceMeshDisabledSuites {
		s.setup(t, ct)
	}
	ct.Launch(t)
	for _, s := range serviceMeshDisabledSuites {
		s := s
		t.Run(fmt.Sprintf("%s_%s", s.DC, s.Peer), func(t *testing.T) {
			t.Parallel()
			s.test(t, ct)
		})
	}
}

func (s *serviceMeshDisabledSuite) testName() string {
	return "Service mesh disabled assertions"
}

// creates clients in s.DC and servers in s.Peer
func (s *serviceMeshDisabledSuite) setup(t *testing.T, ct *commonTopo) {
	clu := ct.ClusterByDatacenter(t, s.DC)
	peerClu := ct.ClusterByDatacenter(t, s.Peer)

	// TODO: handle all partitions
	partition := "default"
	cluPeerName := LocalPeerName(clu, "default")

	serverSID := topology.ServiceID{
		Name:      "ac5-server-http",
		Partition: partition,
	}

	// Make client which will dial server
	clientSID := topology.ServiceID{
		Name:      "ac5-http-client",
		Partition: partition,
	}

	// disable service mesh for client in s.DC
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
	}
	clientNode := ct.AddServiceNode(clu, client)

	server := serviceExt{
		Service: NewFortioServiceWithDefaults(
			peerClu.Datacenter,
			serverSID,
			nil,
		),
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      serverSID.Name,
			Partition: ConfigEntryPartition(serverSID.Partition),
			Protocol:  "http",
		},
		Exports: []api.ServiceConsumer{{Peer: cluPeerName}},
		Intentions: &api.ServiceIntentionsConfigEntry{
			Kind:      api.ServiceIntentions,
			Name:      serverSID.Name,
			Partition: ConfigEntryPartition(serverSID.Partition),
			Sources: []*api.SourceIntention{
				{
					Name:   client.ID.Name,
					Peer:   cluPeerName,
					Action: api.IntentionActionAllow,
				},
			},
		},
	}

	serverNode := ct.AddServiceNode(peerClu, server)

	s.clientSID = clientSID
	s.serverSID = serverSID
	s.nodeServer = serverNode.ID()
	s.nodeClient = clientNode.ID()
}

func (s *serviceMeshDisabledSuite) test(t *testing.T, ct *commonTopo) {
	dc := ct.Sprawl.Topology().Clusters[s.DC]
	peer := ct.Sprawl.Topology().Clusters[s.Peer]
	apiClient := ct.APIClientForCluster(t, dc)
	peerName := LocalPeerName(peer, "default")

	serverSVC := peer.ServiceByID(
		s.nodeServer,
		s.serverSID,
	)

	ct.Assert.HealthyWithPeer(t, dc.Name, serverSVC.ID, peerName)
	s.testExportedServiceInCatalog(t, ct, apiClient, peerName)
	s.testProxyDisabledInDC2(t, apiClient, peerName)
	s.testSingleQueryFailover(t, apiClient, ct, peerName)
}

func (s *serviceMeshDisabledSuite) testExportedServiceInCatalog(t *testing.T, ct *commonTopo, apiClient *api.Client, peer string) {
	t.Run("service exists and is healthy in catalog", func(t *testing.T) {
		libassert.CatalogServiceExists(t, apiClient, s.clientSID.Name, nil)
		libassert.CatalogServiceExists(t, apiClient, s.serverSID.Name, nil)
		libassert.CatalogServiceExists(t, apiClient, s.serverSID.Name, &api.QueryOptions{
			Peer: peer,
		})
		require.NotEqual(t, s.serverSID.Name, s.Peer)
	})
}

func (s *serviceMeshDisabledSuite) testProxyDisabledInDC2(t *testing.T, cl *api.Client, peer string) {
	t.Run("service mesh is disabled for HTTP Service in clients", func(t *testing.T) {
		expected := fmt.Sprintf("%s-sidecar-proxy", s.clientSID.Name)
		services := getServicesInCluster(t, cl, &api.QueryOptions{
			Peer: peer,
		})
		require.NotContains(t, services, expected, fmt.Sprintf("error: should not create proxy for service: %s", services))
	})
}

func (s *serviceMeshDisabledSuite) testSingleQueryFailover(t *testing.T, c *api.Client, ct *commonTopo, peer string) {
	if s.DC == "dc2" {
		// TO-DO: currently failing
		t.Skip()
	}

	t.Run("prepared query with single failover", func(t *testing.T) {
		var err error

		// disable server node in dc1 and relaunch topology
		cfg := ct.Sprawl.Config()
		require.NoError(t, disableNodeInCluster(t, cfg, ct.DC1.Name, s.serverSID.Name, true)) //dc1
		require.NoError(t, ct.Sprawl.Relaunch(cfg))

		// assert server health status
		dc2 := ct.Sprawl.Topology().Clusters[ct.DC2.Name]
		dc2APIClient := ct.APIClientForCluster(t, dc2)

		assertServiceHealth(t, c, s.serverSID.Name, 0)            //dc1 - unhealthy
		assertServiceHealth(t, dc2APIClient, s.serverSID.Name, 1) //dc2 - healthy

		// create prepared query definition
		def = &api.PreparedQueryDefinition{
			Name: "ac5-prepared-query-1failover",
			Service: api.ServiceQuery{
				Service:     s.serverSID.Name,
				Partition:   ConfigEntryPartition(s.serverSID.Partition),
				OnlyPassing: true,
				// create failover for peer in dc2 and dc3 cluster
				Failover: api.QueryFailoverOptions{
					Targets: []api.QueryFailoverTarget{
						{
							Peer: peer,
						},
					},
				},
			},
		}

		query := c.PreparedQuery()
		def.ID, _, err = query.Create(def, nil)
		require.NoError(t, err)

		// Read registered query
		queryDef, _, err := query.Get(def.ID, nil)
		require.NoError(t, err)
		require.Len(t, queryDef, 1, "expected exactly 1 prepared query")
		require.Equal(t, 1, len(queryDef[0].Service.Failover.Targets), "expected 1 failover targets for dc2")
		fmt.Println("PreparedQuery with failover created successfully.")

		// expected outcome should show 2 failovers
		queryResult, _, err := query.Execute(def.ID, nil)
		require.NoError(t, err)
		require.Equal(t, 1, queryResult.Failovers, "expected 1 failover to dc2")
		// should failover to peer in DC2 cluster
		require.Equal(t, ct.DC2.Datacenter, queryResult.Nodes[0].Node.Datacenter)
		// failover to nearest cluster
		require.Equal(t, "peer-dc2-default", queryResult.Nodes[0].Checks[0].PeerName)
	})
}

// getServicesInCluster validates that the service(s) exist in the Consul catalog
func getServicesInCluster(t *testing.T, c *api.Client, opts *api.QueryOptions) map[string][]string {
	var (
		services map[string][]string
		err      error
	)
	retry.Run(t, func(r *retry.R) {
		services, _, err = c.Catalog().Services(opts)
		require.NoError(r, err, "error reading service data")
		if len(services) == 0 {
			r.Fatal("did not find service(s) in catalog")
		}
	})
	return services
}
