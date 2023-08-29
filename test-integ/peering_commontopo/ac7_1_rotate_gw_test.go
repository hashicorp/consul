// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

// TestRotateGW ensures that peered services continue to be able to talk to their
// upstreams during a mesh gateway rotation
// NOTE: because suiteRotateGW needs to mutate the topo, we actually *DO NOT* share a topo

type suiteRotateGW struct {
	DC   string
	Peer string

	sidServer  topology.ServiceID
	nodeServer topology.NodeID

	sidClient  topology.ServiceID
	nodeClient topology.NodeID

	upstream *topology.Upstream

	newMGWNodeName string
}

func TestRotateGW(t *testing.T) {
	suites := []*suiteRotateGW{
		{DC: "dc1", Peer: "dc2"},
		{DC: "dc2", Peer: "dc1"},
	}
	ct := NewCommonTopo(t)
	for _, s := range suites {
		s.setup(t, ct)
	}
	ct.Launch(t)
	for _, s := range suites {
		s := s
		t.Run(fmt.Sprintf("%s->%s", s.DC, s.Peer), func(t *testing.T) {
			// no t.Parallel() due to Relaunch
			s.test(t, ct)
		})
	}
}

func (s *suiteRotateGW) setup(t *testing.T, ct *commonTopo) {
	const prefix = "ac7-1-"

	clu := ct.ClusterByDatacenter(t, s.DC)
	peerClu := ct.ClusterByDatacenter(t, s.Peer)
	partition := "default"
	peer := LocalPeerName(peerClu, "default")
	cluPeerName := LocalPeerName(clu, "default")

	server := NewFortioServiceWithDefaults(
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
		// TODO: we shouldn't need this, need to investigate
		LocalAddress: "0.0.0.0",
		LocalPort:    5001,
		Peer:         peer,
	}
	// create client in us
	client := NewFortioServiceWithDefaults(
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

	// add a second mesh gateway "new"
	s.newMGWNodeName = fmt.Sprintf("new-%s-default-mgw", clu.Name)
	clu.Nodes = append(clu.Nodes, newTopologyMeshGatewaySet(
		// TODO: Dataplane
		topology.NodeKindClient,
		"default",
		s.newMGWNodeName,
		1,
		[]string{clu.Datacenter, "wan"},
		func(i int, node *topology.Node) {
			node.Disabled = true
		},
	)...)
}

func (s *suiteRotateGW) test(t *testing.T, ct *commonTopo) {
	dc := ct.Sprawl.Topology().Clusters[s.DC]
	peer := ct.Sprawl.Topology().Clusters[s.Peer]

	svcHTTPServer := peer.ServiceByID(
		s.nodeServer,
		s.sidServer,
	)
	svcHTTPClient := dc.ServiceByID(
		s.nodeClient,
		s.sidClient,
	)
	ct.Assert.HealthyWithPeer(t, dc.Name, svcHTTPServer.ID, LocalPeerName(peer, "default"))

	ct.Assert.FortioFetch2HeaderEcho(t, svcHTTPClient, s.upstream)

	t.Log("relaunching with new gateways")
	cfg := ct.Sprawl.Config()
	for _, n := range dc.Nodes {
		if strings.HasPrefix(n.Name, s.newMGWNodeName) {
			n.Disabled = false
		}
	}
	require.NoError(t, ct.Sprawl.Relaunch(cfg))
	ct.Assert.FortioFetch2HeaderEcho(t, svcHTTPClient, s.upstream)

	t.Log("relaunching without old gateways")
	cfg = ct.Sprawl.Config()
	for _, n := range dc.Nodes {
		if strings.HasPrefix(n.Name, fmt.Sprintf("%s-default-mgw", dc.Name)) {
			n.Disabled = true
		}
	}
	require.NoError(t, ct.Sprawl.Relaunch(cfg))
	ct.Assert.FortioFetch2HeaderEcho(t, svcHTTPClient, s.upstream)
}
