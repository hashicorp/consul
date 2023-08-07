// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package peering

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/stretchr/testify/require"
)

type ac4ProxyDefaultsSuite struct {
	DC   string
	Peer string

	nodeClient topology.NodeID
	nodeServer topology.NodeID

	serverSID topology.ServiceID
	clientSID topology.ServiceID
	upstream  *topology.Upstream
}

var ac4ProxyDefaultsSuites []sharedTopoSuite = []sharedTopoSuite{
	&ac4ProxyDefaultsSuite{DC: "dc1", Peer: "dc2"},
	&ac4ProxyDefaultsSuite{DC: "dc2", Peer: "dc1"},
}

func TestAC4ProxyDefaults(t *testing.T) {
	runShareableSuites(t, ac4ProxyDefaultsSuites)
}

func (s *ac4ProxyDefaultsSuite) testName() string {
	return fmt.Sprintf("ac4 proxy defaults %s->%s", s.DC, s.Peer)
}

// creates clients in s.DC and servers in s.Peer
func (s *ac4ProxyDefaultsSuite) setup(t *testing.T, ct *commonTopo) {
	clu := ct.ClusterByDatacenter(t, s.DC)
	peerClu := ct.ClusterByDatacenter(t, s.Peer)

	partition := "default"
	peer := LocalPeerName(peerClu, "default")
	cluPeerName := LocalPeerName(clu, "default")

	serverSID := topology.ServiceID{
		Name:      "ac4-server-http",
		Partition: partition,
	}
	// Define server as upstream for client
	upstream := &topology.Upstream{
		ID:        serverSID,
		LocalPort: 5000,
		Peer:      peer,
	}

	// Make client which will dial server
	clientSID := topology.ServiceID{
		Name:      "ac4-http-client",
		Partition: partition,
	}
	client := serviceExt{
		Service: NewFortioServiceWithDefaults(
			clu.Datacenter,
			clientSID,
			func(s *topology.Service) {
				s.Upstreams = []*topology.Upstream{
					upstream,
				}
			},
		),
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      clientSID.Name,
			Partition: ConfigEntryPartition(clientSID.Partition),
			Protocol:  "http",
			UpstreamConfig: &api.UpstreamConfiguration{
				Defaults: &api.UpstreamConfig{
					MeshGateway: api.MeshGatewayConfig{
						Mode: api.MeshGatewayModeLocal,
					},
				},
			},
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

	peerClu.InitialConfigEntries = append(peerClu.InitialConfigEntries,
		&api.ProxyConfigEntry{
			Kind:      api.ProxyDefaults,
			Name:      api.ProxyConfigGlobal,
			Partition: ConfigEntryPartition(server.ID.Partition),
			Config: map[string]interface{}{
				"protocol":                 "http",
				"local_request_timeout_ms": 500,
			},
			MeshGateway: api.MeshGatewayConfig{
				Mode: api.MeshGatewayModeLocal,
			},
		},
	)

	serverNode := ct.AddServiceNode(peerClu, server)

	s.clientSID = clientSID
	s.serverSID = serverSID
	s.nodeServer = serverNode.ID()
	s.nodeClient = clientNode.ID()
	s.upstream = upstream
}

func (s *ac4ProxyDefaultsSuite) test(t *testing.T, ct *commonTopo) {
	var client *topology.Service

	dc := ct.Sprawl.Topology().Clusters[s.DC]
	peer := ct.Sprawl.Topology().Clusters[s.Peer]

	clientSVC := dc.ServiceByID(
		s.nodeClient,
		s.clientSID,
	)
	serverSVC := peer.ServiceByID(
		s.nodeServer,
		s.serverSID,
	)

	// preconditions check
	ct.Assert.HealthyWithPeer(t, dc.Name, serverSVC.ID, LocalPeerName(peer, "default"))
	ct.Assert.UpstreamEndpointHealthy(t, clientSVC, s.upstream)
	ct.Assert.FortioFetch2HeaderEcho(t, clientSVC, s.upstream)

	t.Run("Validate services exist in catalog", func(t *testing.T) {
		dcSvcs := dc.ServicesByID(s.clientSID)
		require.Len(t, dcSvcs, 1, "expected exactly one client")
		client = dcSvcs[0]
		require.Len(t, client.Upstreams, 1, "expected exactly one upstream for client")

		server := dc.ServicesByID(s.serverSID)
		require.Len(t, server, 1, "expected exactly one server")
		require.Len(t, server[0].Upstreams, 0, "expected no upstream for server")
	})

	t.Run("peered upstream exists in catalog", func(t *testing.T) {
		ct.Assert.CatalogServiceExists(t, s.DC, s.upstream.ID.Name, &api.QueryOptions{
			Peer: s.upstream.Peer,
		})
	})

	t.Run("HTTP service fails due to connection timeout", func(t *testing.T) {
		url504 := fmt.Sprintf("http://localhost:%d/fortio/fetch2?url=%s", client.ExposedPort,
			url.QueryEscape(fmt.Sprintf("http://localhost:%d/?delay=1000ms", s.upstream.LocalPort)),
		)

		url200 := fmt.Sprintf("http://localhost:%d/fortio/fetch2?url=%s", client.ExposedPort,
			url.QueryEscape(fmt.Sprintf("http://localhost:%d/", s.upstream.LocalPort)),
		)

		// validate request timeout error where service has 1000ms response delay and
		// proxy default is set to local_request_timeout_ms: 500ms
		// return 504
		httpClient := cleanhttp.DefaultClient()
		req, err := http.NewRequest(http.MethodGet, url504, nil)
		require.NoError(t, err)

		res, err := httpClient.Do(req)
		require.NoError(t, err)

		defer res.Body.Close()
		require.Equal(t, http.StatusGatewayTimeout, res.StatusCode)

		// validate successful GET request where service has no response delay and
		// proxy default is set to local_request_timeout_ms: 500ms
		// return 200
		req, err = http.NewRequest(http.MethodGet, url200, nil)
		require.NoError(t, err)

		res, err = httpClient.Do(req)
		require.NoError(t, err)

		defer res.Body.Close()
		require.Equal(t, http.StatusOK, res.StatusCode)
	})
}
