// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package peering

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

type ac2DiscoChainSuite struct {
	DC   string
	Peer string

	clientSID topology.ServiceID
}

var ac2DiscoChainSuites []sharedTopoSuite = []sharedTopoSuite{
	&ac2DiscoChainSuite{DC: "dc1", Peer: "dc2"},
	&ac2DiscoChainSuite{DC: "dc2", Peer: "dc1"},
}

func TestAC2DiscoChain(t *testing.T) {
	runShareableSuites(t, ac2DiscoChainSuites)
}

func (s *ac2DiscoChainSuite) testName() string {
	return fmt.Sprintf("ac2 disco chain %s->%s", s.DC, s.Peer)
}

func (s *ac2DiscoChainSuite) setup(t *testing.T, ct *commonTopo) {
	clu := ct.ClusterByDatacenter(t, s.DC)
	peerClu := ct.ClusterByDatacenter(t, s.Peer)
	partition := "default"
	peer := LocalPeerName(peerClu, "default")

	// Make an HTTP server with discovery chain config entries
	server := NewFortioServiceWithDefaults(
		clu.Datacenter,
		topology.ServiceID{
			Name:      "ac2-disco-chain-svc",
			Partition: partition,
		},
		nil,
	)
	ct.ExportService(clu, partition,
		api.ExportedService{
			Name: server.ID.Name,
			Consumers: []api.ServiceConsumer{
				{
					Peer: peer,
				},
			},
		},
	)

	clu.InitialConfigEntries = append(clu.InitialConfigEntries,
		&api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      server.ID.Name,
			Partition: ConfigEntryPartition(partition),
			Protocol:  "http",
		},
		&api.ServiceSplitterConfigEntry{
			Kind:      api.ServiceSplitter,
			Name:      server.ID.Name,
			Partition: ConfigEntryPartition(partition),
			Splits: []api.ServiceSplit{
				{
					Weight: 100.0,
					ResponseHeaders: &api.HTTPHeaderModifiers{
						Add: map[string]string{
							"X-Split": "test",
						},
					},
				},
			},
		},
	)
	ct.AddServiceNode(clu, serviceExt{Service: server})

	// Define server as upstream for client
	upstream := &topology.Upstream{
		ID: topology.ServiceID{
			Name:      server.ID.Name,
			Partition: partition, // TODO: iterate over all possible partitions
		},
		// TODO: we need to expose this on 0.0.0.0 so we can check it
		// through our forward proxy. not realistic IMO
		LocalAddress: "0.0.0.0",
		LocalPort:    5000,
		Peer:         peer,
	}

	// Make client which will dial server
	clientSID := topology.ServiceID{
		Name:      "ac2-client",
		Partition: partition,
	}
	client := NewFortioServiceWithDefaults(
		clu.Datacenter,
		clientSID,
		func(s *topology.Service) {
			s.Upstreams = []*topology.Upstream{
				upstream,
			}
		},
	)
	ct.ExportService(clu, partition,
		api.ExportedService{
			Name: client.ID.Name,
			Consumers: []api.ServiceConsumer{
				{
					Peer: peer,
				},
			},
		},
	)
	ct.AddServiceNode(clu, serviceExt{Service: client})

	clu.InitialConfigEntries = append(clu.InitialConfigEntries,
		&api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      client.ID.Name,
			Partition: ConfigEntryPartition(partition),
			Protocol:  "http",
			UpstreamConfig: &api.UpstreamConfiguration{
				Defaults: &api.UpstreamConfig{
					MeshGateway: api.MeshGatewayConfig{
						Mode: api.MeshGatewayModeLocal,
					},
				},
			},
		},
	)

	// Add intention allowing client to call server
	clu.InitialConfigEntries = append(clu.InitialConfigEntries,
		&api.ServiceIntentionsConfigEntry{
			Kind:      api.ServiceIntentions,
			Name:      server.ID.Name,
			Partition: ConfigEntryPartition(partition),
			Sources: []*api.SourceIntention{
				{
					Name:   client.ID.Name,
					Peer:   peer,
					Action: api.IntentionActionAllow,
				},
			},
		},
	)

	s.clientSID = clientSID
}

func (s *ac2DiscoChainSuite) test(t *testing.T, ct *commonTopo) {
	dc := ct.Sprawl.Topology().Clusters[s.DC]

	svcs := dc.ServicesByID(s.clientSID)
	require.Len(t, svcs, 1, "expected exactly one client in datacenter")

	client := svcs[0]
	require.Len(t, client.Upstreams, 1, "expected exactly one upstream for client")
	u := client.Upstreams[0]

	t.Run("peered upstream exists in catalog", func(t *testing.T) {
		t.Parallel()
		ct.Assert.CatalogServiceExists(t, s.DC, u.ID.Name, &api.QueryOptions{
			Peer: u.Peer,
		})
	})

	t.Run("peered upstream endpoint status is healthy", func(t *testing.T) {
		t.Parallel()
		ct.Assert.UpstreamEndpointStatus(t, client, peerClusterPrefix(u), "HEALTHY", 1)
	})

	t.Run("response contains header injected by splitter", func(t *testing.T) {
		t.Parallel()
		// TODO: not sure we should call u.LocalPort? it's not realistic from a security
		// standpoint. prefer the fortio fetch2 stuff myself
		ct.Assert.HTTPServiceEchoesResHeader(t, client, u.LocalPort, "",
			map[string]string{
				"X-Split": "test",
			},
		)
	})
}

// For reference see consul/xds/clusters.go:
//
//	func (s *ResourceGenerator) getTargetClusterName
//
// and connect/sni.go
func peerClusterPrefix(u *topology.Upstream) string {
	if u.Peer == "" {
		panic("upstream is not from a peer")
	}
	u.ID.Normalize()
	return u.ID.Name + "." + u.ID.Namespace + "." + u.Peer + ".external"
}
