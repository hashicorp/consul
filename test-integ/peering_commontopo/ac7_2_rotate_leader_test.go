package peering

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testingconsul/topology"
	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// TestAC7_2RotateLeader ensures that after a leader rotation, information continues to replicate to peers
// NOTE: because suiteRotateLeader needs to mutate the topo, we actually *DO NOT* share a topo
func TestAC7_2RotateLeader(t *testing.T) {
	if allowParallelCommonTopo {
		t.Parallel()
	}
	ct := NewCommonTopo(t)
	for _, s := range ac7_2RotateLeaderSuites {
		s.setup(t, ct)
	}
	ct.Launch(t)
	for _, s := range ac7_2RotateLeaderSuites {
		t.Run(s.testName(), func(t *testing.T) { s.test(t, ct) })
	}
}

var ac7_2RotateLeaderSuites []*ac7_2RotateLeaderSuite = []*ac7_2RotateLeaderSuite{
	{DC: "dc1", Peer: "dc2"},
	{DC: "dc2", Peer: "dc1"},
}

type ac7_2RotateLeaderSuite struct {
	DC   string
	Peer string

	sidServer  topology.ServiceID
	nodeServer topology.NodeID

	sidClient  topology.ServiceID
	nodeClient topology.NodeID

	upstream *topology.Upstream
}

var _ commonTopoSuite = (*ac7_2RotateLeaderSuite)(nil)

func (s *ac7_2RotateLeaderSuite) testName() string {
	return fmt.Sprintf("ac7.2 rotate leader %s->%s", s.DC, s.Peer)
}

// makes client in clu, server in peerClu
func (s *ac7_2RotateLeaderSuite) setup(t *testing.T, ct *commonTopo) {
	const prefix = "ac7-2-"

	clu := ct.ClusterByDatacenter(t, s.DC)
	peerClu := ct.ClusterByDatacenter(t, s.Peer)
	partition := "default"
	peer := LocalPeerName(peerClu, "default")
	cluPeerName := LocalPeerName(clu, "default")

	server := newFortioServiceWithDefaults(
		peerClu.Datacenter,
		topology.ServiceID{
			Name:      prefix + "server-http",
			Partition: partition,
		},
		nil,
	)

	// Make clients which have server upstreams
	upstream := &topology.Upstream{
		ID: topology.ServiceID{
			Name:      server.ID.Name,
			Partition: partition,
		},
		LocalPort: 5001,
		Peer:      peer,
	}
	// create client in us
	client := newFortioServiceWithDefaults(
		clu.Datacenter,
		topology.ServiceID{
			Name:      prefix + "client",
			Partition: partition,
		},
		func(s *topology.Service) {
			s.Upstreams = []*topology.Upstream{
				upstream,
			}
		},
	)
	clientNode := ct.AddServiceNode(clu, serviceExt{Service: client,
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      client.ID.Name,
			Partition: ConfigEntryPartition(client.ID.Partition),
			Protocol:  "http",
			UpstreamConfig: &api.UpstreamConfiguration{
				Defaults: &api.UpstreamConfig{
					MeshGateway: api.MeshGatewayConfig{
						Mode: api.MeshGatewayModeLocal,
					},
				},
			},
		},
	})
	// actually to be used by the other pairing
	serverNode := ct.AddServiceNode(peerClu, serviceExt{
		Service: server,
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      server.ID.Name,
			Partition: ConfigEntryPartition(partition),
			Protocol:  "http",
		},
		Exports: []api.ServiceConsumer{{Peer: cluPeerName}},
		Intentions: &api.ServiceIntentionsConfigEntry{
			Kind:      api.ServiceIntentions,
			Name:      server.ID.Name,
			Partition: ConfigEntryPartition(partition),
			Sources: []*api.SourceIntention{
				{
					Name:   client.ID.Name,
					Peer:   cluPeerName,
					Action: api.IntentionActionAllow,
				},
			},
		},
	})

	s.sidClient = client.ID
	s.nodeClient = clientNode.ID()
	s.upstream = upstream
	s.sidServer = server.ID
	s.nodeServer = serverNode.ID()
}

func (s *ac7_2RotateLeaderSuite) test(t *testing.T, ct *commonTopo) {
	dc := ct.Sprawl.Topology().Clusters[s.DC]
	peer := ct.Sprawl.Topology().Clusters[s.Peer]
	clDC := ct.APIClientForCluster(t, dc)
	clPeer := ct.APIClientForCluster(t, peer)

	svcServer := peer.ServiceByID(s.nodeServer, s.sidServer)
	svcClient := dc.ServiceByID(s.nodeClient, s.sidClient)
	ct.Assert.HealthyWithPeer(t, dc.Name, svcServer.ID, LocalPeerName(peer, "default"))

	ct.Assert.FortioFetch2HeaderEcho(t, svcClient, s.upstream)

	// force leader election
	rotateLeader(t, clDC)
	rotateLeader(t, clPeer)

	// unexport httpServer
	ce, _, err := clPeer.ConfigEntries().Get(api.ExportedServices, s.sidServer.Partition, nil)
	require.NoError(t, err)
	// ceAsES = config entry as ExportedServicesConfigEntry
	ceAsES := ce.(*api.ExportedServicesConfigEntry)
	origCE, err := copystructure.Copy(ceAsES)
	require.NoError(t, err)
	found := 0
	foundI := 0
	for i, svc := range ceAsES.Services {
		if svc.Name == s.sidServer.Name && svc.Namespace == utils.DefaultToEmpty(s.sidServer.Namespace) {
			found += 1
			foundI = i
		}
	}
	require.Equal(t, found, 1)
	// remove found entry
	ceAsES.Services = append(ceAsES.Services[:foundI], ceAsES.Services[foundI+1:]...)
	_, _, err = clPeer.ConfigEntries().Set(ceAsES, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		//restore for next pairing
		_, _, err = clPeer.ConfigEntries().Set(origCE.(*api.ExportedServicesConfigEntry), nil)
		require.NoError(t, err)
	})

	// expect health entry in for peer to disappear
	retry.RunWith(&retry.Timer{Timeout: time.Minute, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
		svcs, _, err := clDC.Health().Service(s.sidServer.Name, "", true, utils.CompatQueryOpts(&api.QueryOptions{
			Partition: s.sidServer.Partition,
			Namespace: s.sidServer.Namespace,
			Peer:      LocalPeerName(peer, "default"),
		}))
		require.NoError(r, err)
		assert.Equal(r, len(svcs), 0, "health entry for imported service gone")
	})
}

func rotateLeader(t *testing.T, cl *api.Client) {
	t.Helper()
	oldLeader := findLeader(t, cl)
	cl.Operator().RaftLeaderTransfer(nil)
	retry.RunWith(&retry.Timer{Timeout: 30 * time.Second, Wait: time.Second}, t, func(r *retry.R) {
		newLeader := findLeader(r, cl)
		require.NotEqual(r, oldLeader.ID, newLeader.ID)
	})
}

func findLeader(t require.TestingT, cl *api.Client) *api.RaftServer {
	raftConfig, err := cl.Operator().RaftGetConfiguration(nil)
	require.NoError(t, err)
	var leader *api.RaftServer
	for _, svr := range raftConfig.Servers {
		if svr.Leader {
			leader = svr
		}
	}
	require.NotNil(t, leader)
	return leader
}
