package peering

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/testing/deployer/topology"

	"github.com/hashicorp/consul/api"
)

var ac1BasicSuites []*ac1BasicSuite = []*ac1BasicSuite{
	{DC: "dc1", Peer: "dc2"},
	{DC: "dc2", Peer: "dc1"},
}

func TestAC1Basic(t *testing.T) {
	if !*FlagNoReuseCommonTopo {
		t.Skip("NoReuseCommonTopo unset")
	}
	if allowParallelCommonTopo {
		t.Parallel()
	}
	ct := NewCommonTopo(t)
	for _, s := range ac1BasicSuites {
		s.setup(t, ct)
	}
	ct.Launch(t)
	for _, s := range ac1BasicSuites {
		s := s
		t.Run(s.testName(), func(t *testing.T) {
			t.Parallel()
			s.test(t, ct)
		})
	}
}

type ac1BasicSuite struct {
	// inputs
	DC   string
	Peer string

	// test points
	sidServerHTTP  topology.ServiceID
	sidServerTCP   topology.ServiceID
	nodeServerHTTP topology.NodeID
	nodeServerTCP  topology.NodeID

	// 1.1
	sidClientTCP  topology.ServiceID
	nodeClientTCP topology.NodeID

	// 1.2
	sidClientHTTP  topology.ServiceID
	nodeClientHTTP topology.NodeID

	upstreamHTTP *topology.Upstream
	upstreamTCP  *topology.Upstream
}

var _ commonTopoSuite = (*ac1BasicSuite)(nil)

func (s *ac1BasicSuite) testName() string {
	return fmt.Sprintf("ac1 basic %s->%s", s.DC, s.Peer)
}

// creates clients in s.DC and servers in s.Peer
func (s *ac1BasicSuite) setup(t *testing.T, ct *commonTopo) {
	clu := ct.ClusterByDatacenter(t, s.DC)
	peerClu := ct.ClusterByDatacenter(t, s.Peer)

	partition := "default"
	peer := LocalPeerName(peerClu, "default")
	cluPeerName := LocalPeerName(clu, "default")
	const prefix = "ac1-"

	tcpServerSID := topology.ServiceID{
		Name:      prefix + "server-tcp",
		Partition: partition,
	}
	httpServerSID := topology.ServiceID{
		Name:      prefix + "server-http",
		Partition: partition,
	}
	upstreamHTTP := &topology.Upstream{
		ID: topology.ServiceID{
			Name:      httpServerSID.Name,
			Partition: partition,
		},
		LocalPort: 5001,
		Peer:      peer,
	}
	upstreamTCP := &topology.Upstream{
		ID: topology.ServiceID{
			Name:      tcpServerSID.Name,
			Partition: partition,
		},
		LocalPort: 5000,
		Peer:      peer,
	}

	// Make clients which have server upstreams
	setupClientServiceAndConfigs := func(protocol string) (serviceExt, *topology.Node) {
		sid := topology.ServiceID{
			Name:      prefix + "client-" + protocol,
			Partition: partition,
		}
		svc := serviceExt{
			Service: NewFortioServiceWithDefaults(
				clu.Datacenter,
				sid,
				func(s *topology.Service) {
					s.Upstreams = []*topology.Upstream{
						upstreamTCP,
						upstreamHTTP,
					}
				},
			),
			Config: &api.ServiceConfigEntry{
				Kind:      api.ServiceDefaults,
				Name:      sid.Name,
				Partition: ConfigEntryPartition(sid.Partition),
				Protocol:  protocol,
				UpstreamConfig: &api.UpstreamConfiguration{
					Defaults: &api.UpstreamConfig{
						MeshGateway: api.MeshGatewayConfig{
							Mode: api.MeshGatewayModeLocal,
						},
					},
				},
			},
		}

		node := ct.AddServiceNode(clu, svc)

		return svc, node
	}
	tcpClient, tcpClientNode := setupClientServiceAndConfigs("tcp")
	httpClient, httpClientNode := setupClientServiceAndConfigs("http")

	httpServer := serviceExt{
		Service: NewFortioServiceWithDefaults(
			peerClu.Datacenter,
			httpServerSID,
			nil,
		),
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      httpServerSID.Name,
			Partition: ConfigEntryPartition(httpServerSID.Partition),
			Protocol:  "http",
		},
		Exports: []api.ServiceConsumer{{Peer: cluPeerName}},
		Intentions: &api.ServiceIntentionsConfigEntry{
			Kind:      api.ServiceIntentions,
			Name:      httpServerSID.Name,
			Partition: ConfigEntryPartition(httpServerSID.Partition),
			Sources: []*api.SourceIntention{
				{
					Name:   tcpClient.ID.Name,
					Peer:   cluPeerName,
					Action: api.IntentionActionAllow,
				},
				{
					Name:   httpClient.ID.Name,
					Peer:   cluPeerName,
					Action: api.IntentionActionAllow,
				},
			},
		},
	}
	tcpServer := serviceExt{
		Service: NewFortioServiceWithDefaults(
			peerClu.Datacenter,
			tcpServerSID,
			nil,
		),
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      tcpServerSID.Name,
			Partition: ConfigEntryPartition(tcpServerSID.Partition),
			Protocol:  "tcp",
		},
		Exports: []api.ServiceConsumer{{Peer: cluPeerName}},
		Intentions: &api.ServiceIntentionsConfigEntry{
			Kind:      api.ServiceIntentions,
			Name:      tcpServerSID.Name,
			Partition: ConfigEntryPartition(tcpServerSID.Partition),
			Sources: []*api.SourceIntention{
				{
					Name:   tcpClient.ID.Name,
					Peer:   cluPeerName,
					Action: api.IntentionActionAllow,
				},
				{
					Name:   httpClient.ID.Name,
					Peer:   cluPeerName,
					Action: api.IntentionActionAllow,
				},
			},
		},
	}

	httpServerNode := ct.AddServiceNode(peerClu, httpServer)
	tcpServerNode := ct.AddServiceNode(peerClu, tcpServer)

	s.sidClientHTTP = httpClient.ID
	s.nodeClientHTTP = httpClientNode.ID()
	s.sidClientTCP = tcpClient.ID
	s.nodeClientTCP = tcpClientNode.ID()
	s.upstreamHTTP = upstreamHTTP
	s.upstreamTCP = upstreamTCP

	// these are references in Peer
	s.sidServerHTTP = httpServerSID
	s.nodeServerHTTP = httpServerNode.ID()
	s.sidServerTCP = tcpServerSID
	s.nodeServerTCP = tcpServerNode.ID()
}

// implements https://docs.google.com/document/d/1Fs3gNMhCqE4zVNMFcbzf02ZrB0kxxtJpI2h905oKhrs/edit#heading=h.wtzvyryyb56v
func (s *ac1BasicSuite) test(t *testing.T, ct *commonTopo) {
	dc := ct.Sprawl.Topology().Clusters[s.DC]
	peer := ct.Sprawl.Topology().Clusters[s.Peer]
	ac := s

	// refresh this from Topology
	svcClientTCP := dc.ServiceByID(
		ac.nodeClientTCP,
		ac.sidClientTCP,
	)
	svcClientHTTP := dc.ServiceByID(
		ac.nodeClientHTTP,
		ac.sidClientHTTP,
	)
	// our ac has the node/sid for server in the peer DC
	svcServerHTTP := peer.ServiceByID(
		ac.nodeServerHTTP,
		ac.sidServerHTTP,
	)
	svcServerTCP := peer.ServiceByID(
		ac.nodeServerTCP,
		ac.sidServerTCP,
	)

	// preconditions
	// these could be done parallel with each other, but complexity
	// probably not worth the speed boost
	ct.Assert.HealthyWithPeer(t, dc.Name, svcServerHTTP.ID, LocalPeerName(peer, "default"))
	ct.Assert.HealthyWithPeer(t, dc.Name, svcServerTCP.ID, LocalPeerName(peer, "default"))
	ct.Assert.UpstreamEndpointHealthy(t, svcClientTCP, ac.upstreamTCP)
	ct.Assert.UpstreamEndpointHealthy(t, svcClientTCP, ac.upstreamHTTP)

	tcs := []struct {
		acSub int
		proto string
		svc   *topology.Service
	}{
		{1, "tcp", svcClientTCP},
		{2, "http", svcClientHTTP},
	}
	for _, tc := range tcs {
		tc := tc
		t.Run(fmt.Sprintf("1.%d. %s in A can call HTTP upstream", tc.acSub, tc.proto), func(t *testing.T) {
			t.Parallel()
			ct.Assert.FortioFetch2HeaderEcho(t, tc.svc, ac.upstreamHTTP)
		})
		t.Run(fmt.Sprintf("1.%d. %s in A can call TCP upstream", tc.acSub, tc.proto), func(t *testing.T) {
			t.Parallel()
			ct.Assert.FortioFetch2HeaderEcho(t, tc.svc, ac.upstreamTCP)
		})
		t.Run(fmt.Sprintf("1.%d. via %s in A, FORTIO_NAME of HTTP upstream", tc.acSub, tc.proto), func(t *testing.T) {
			t.Parallel()
			ct.Assert.FortioFetch2FortioName(t,
				tc.svc,
				ac.upstreamHTTP,
				peer.Name,
				svcServerHTTP.ID,
			)
		})
		t.Run(fmt.Sprintf("1.%d. via %s in A, FORTIO_NAME of TCP upstream", tc.acSub, tc.proto), func(t *testing.T) {
			t.Parallel()
			ct.Assert.FortioFetch2FortioName(t,
				tc.svc,
				ac.upstreamTCP,
				peer.Name,
				svcServerTCP.ID,
			)
		})
	}
}
