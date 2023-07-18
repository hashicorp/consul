package peering

import (
	"fmt"

	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/stretchr/testify/require"
)

type ac5_1NoSvcMeshSuite struct {
	DC   string
	Peer string

	serverSID topology.ServiceID
	clientSID topology.ServiceID
}

var (
	ac5_1NoSvcMeshSuites []commonTopoSuite = []commonTopoSuite{
		&ac5_1NoSvcMeshSuite{DC: "dc1", Peer: "dc2"},
		&ac5_1NoSvcMeshSuite{DC: "dc2", Peer: "dc1"},
	}
)

func TestAC5ServiceMeshDisabledSuite(t *testing.T) {
	setupAndRunTestSuite(t, ac5_1NoSvcMeshSuites, true, true)
}

func (s *ac5_1NoSvcMeshSuite) testName() string {
	return fmt.Sprintf("ac5.1 no service mesh %s->%s", s.DC, s.Peer)
}

// creates clients in s.DC and servers in s.Peer
func (s *ac5_1NoSvcMeshSuite) setup(t *testing.T, ct *commonTopo) {
	clu := ct.ClusterByDatacenter(t, s.DC)
	peerClu := ct.ClusterByDatacenter(t, s.Peer)

	// TODO: handle all partitions
	partition := "default"
	peer := LocalPeerName(peerClu, partition)

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

	ct.AddServiceNode(clu, server)

	s.clientSID = clientSID
	s.serverSID = serverSID
}

func (s *ac5_1NoSvcMeshSuite) test(t *testing.T, ct *commonTopo) {
	dc := ct.Sprawl.Topology().Clusters[s.DC]
	peer := ct.Sprawl.Topology().Clusters[s.Peer]
	cl := ct.APIClientForCluster(t, dc)
	peerName := LocalPeerName(peer, "default")

	s.testServiceHealthInCatalog(t, ct, cl, peerName)
	s.testProxyDisabledInDC2(t, cl, peerName)
}

func (s *ac5_1NoSvcMeshSuite) testServiceHealthInCatalog(t *testing.T, ct *commonTopo, cl *api.Client, peer string) {
	t.Run("validate service health in catalog", func(t *testing.T) {
		libassert.CatalogServiceExists(t, cl, s.clientSID.Name, &api.QueryOptions{
			Peer: peer,
		})
		require.NotEqual(t, s.serverSID.Name, s.Peer)
		assertServiceHealth(t, cl, s.serverSID.Name, 1)
	})
}

func (s *ac5_1NoSvcMeshSuite) testProxyDisabledInDC2(t *testing.T, cl *api.Client, peer string) {
	t.Run("service mesh is disabled", func(t *testing.T) {
		var (
			services map[string][]string
			err      error
			expected = fmt.Sprintf("%s-sidecar-proxy", s.clientSID.Name)
		)
		retry.Run(t, func(r *retry.R) {
			services, _, err = cl.Catalog().Services(&api.QueryOptions{
				Peer: peer,
			})
			require.NoError(r, err, "error reading service data")
			require.Greater(r, len(services), 0, "did not find service(s) in catalog")
		})
		require.NotContains(t, services, expected, fmt.Sprintf("error: should not create proxy for service: %s", services))
	})
}
