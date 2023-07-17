package peering

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/itchyny/gojq"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
)

var ac3SvcDefaultsSuites []*ac3SvcDefaultsSuite = []*ac3SvcDefaultsSuite{
	{DC: "dc1", Peer: "dc2"},
	{DC: "dc2", Peer: "dc1"},
}

func TestAC3SvcDefaults(t *testing.T) {
	if !*FlagNoReuseCommonTopo {
		t.Skip("NoReuseCommonTopo unset")
	}
	if allowParallelCommonTopo {
		t.Parallel()
	}
	ct := NewCommonTopo(t)

	for _, s := range ac3SvcDefaultsSuites {
		s.setup(t, ct)
	}
	ct.Launch(t)
	for _, s := range ac3SvcDefaultsSuites {
		s := s
		t.Run(s.testName(), func(t *testing.T) {
			t.Parallel()
			s.test(t, ct)
		})
	}
}

type ac3SvcDefaultsSuite struct {
	// inputs
	DC   string
	Peer string

	// test points
	sidServer  topology.ServiceID
	nodeServer topology.NodeID
	sidClient  topology.ServiceID
	nodeClient topology.NodeID

	upstream *topology.Upstream
}

var _ commonTopoSuite = (*ac3SvcDefaultsSuite)(nil)

func (s *ac3SvcDefaultsSuite) testName() string {
	return fmt.Sprintf("ac3 service defaults upstreams %s -> %s", s.DC, s.Peer)
}

// creates clients in s.DC and servers in s.Peer
func (s *ac3SvcDefaultsSuite) setup(t *testing.T, ct *commonTopo) {
	clu := ct.ClusterByDatacenter(t, s.DC)
	peerClu := ct.ClusterByDatacenter(t, s.Peer)

	partition := "default"
	peer := LocalPeerName(peerClu, "default")
	cluPeerName := LocalPeerName(clu, "default")

	serverSID := topology.ServiceID{
		Name:      "ac3-server",
		Partition: partition,
	}
	upstream := &topology.Upstream{
		ID: topology.ServiceID{
			Name:      serverSID.Name,
			Partition: partition,
		},
		LocalPort: 5001,
		Peer:      peer,
	}

	sid := topology.ServiceID{
		Name:      "ac3-client",
		Partition: partition,
	}
	client := serviceExt{
		Service: NewFortioServiceWithDefaults(
			clu.Datacenter,
			sid,
			func(s *topology.Service) {
				s.Upstreams = []*topology.Upstream{
					upstream,
				}
			},
		),
		Config: &api.ServiceConfigEntry{
			Kind:      api.ServiceDefaults,
			Name:      sid.Name,
			Partition: ConfigEntryPartition(sid.Partition),
			Protocol:  "http",
			UpstreamConfig: &api.UpstreamConfiguration{
				Overrides: []*api.UpstreamConfig{
					{
						Name:      upstream.ID.Name,
						Namespace: upstream.ID.Namespace,
						Peer:      peer,
						PassiveHealthCheck: &api.PassiveHealthCheck{
							MaxFailures: 1,
							Interval:    10 * time.Minute,
						},
					},
				},
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

	serverNode := ct.AddServiceNode(peerClu, server)

	s.sidClient = client.ID
	s.nodeClient = clientNode.ID()
	s.upstream = upstream

	// these are references in Peer
	s.sidServer = serverSID
	s.nodeServer = serverNode.ID()
}

// make two requests to upstream via client's fetch2 with status=<nonceStatus>
// the first time, it should return nonceStatus
// the second time, we expect the upstream to have been removed from the envoy cluster,
// and thereby get some other 5xx
func (s *ac3SvcDefaultsSuite) test(t *testing.T, ct *commonTopo) {
	dc := ct.Sprawl.Topology().Clusters[s.DC]
	peer := ct.Sprawl.Topology().Clusters[s.Peer]

	// refresh this from Topology
	svcClient := dc.ServiceByID(
		s.nodeClient,
		s.sidClient,
	)
	// our ac has the node/sid for server in the peer DC
	svcServer := peer.ServiceByID(
		s.nodeServer,
		s.sidServer,
	)

	// preconditions
	// these could be done parallel with each other, but complexity
	// probably not worth the speed boost
	ct.Assert.HealthyWithPeer(t, dc.Name, svcServer.ID, LocalPeerName(peer, "default"))
	ct.Assert.UpstreamEndpointHealthy(t, svcClient, s.upstream)
	// TODO: we need to let the upstream start serving properly before we do this. if it
	// isn't ready and returns a 5xx (which it will do if it's not up yet!), it will stick
	// in a down state for PassiveHealthCheck.Interval
	time.Sleep(30 * time.Second)
	ct.Assert.FortioFetch2HeaderEcho(t, svcClient, s.upstream)

	// TODO: use proxied HTTP client
	client := cleanhttp.DefaultClient()
	// TODO: what is default? namespace? partition?
	clusterName := fmt.Sprintf("%s.default.%s.external", s.upstream.ID.Name, s.upstream.Peer)
	nonceStatus := http.StatusInsufficientStorage
	url507 := fmt.Sprintf("http://localhost:%d/fortio/fetch2?url=%s", svcClient.ExposedPort,
		url.QueryEscape(fmt.Sprintf("http://localhost:%d/?status=%d", s.upstream.LocalPort, nonceStatus)),
	)

	// we only make this call once
	req, err := http.NewRequest(http.MethodGet, url507, nil)
	require.NoError(t, err)
	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, nonceStatus, res.StatusCode)

	// this is a modified version of assertEnvoyUpstreamHealthy
	envoyAddr := fmt.Sprintf("localhost:%d", svcClient.ExposedEnvoyAdminPort)
	retry.RunWith(&retry.Timer{Timeout: 30 * time.Second, Wait: 500 * time.Millisecond}, t, func(r *retry.R) {
		// BOOKMARK: avoid libassert, but we need to resurrect this method in asserter first
		clusters, statusCode, err := libassert.GetEnvoyOutputWithClient(client, envoyAddr, "clusters", map[string]string{"format": "json"})
		if err != nil {
			r.Fatal("could not fetch envoy clusters")
		}
		require.Equal(r, 200, statusCode)

		filter := fmt.Sprintf(
			`.cluster_statuses[]
			| select(.name|contains("%s"))
			| [.host_statuses[].health_status.failed_outlier_check]
			|.[0]`,
			clusterName)
		result, err := jqOne(clusters, filter)
		require.NoErrorf(r, err, "could not found cluster name %q: %v \n%s", clusterName, err, clusters)

		resultAsBool, ok := result.(bool)
		require.True(r, ok)
		require.True(r, resultAsBool)
	})

	url200 := fmt.Sprintf("http://localhost:%d/fortio/fetch2?url=%s", svcClient.ExposedPort,
		url.QueryEscape(fmt.Sprintf("http://localhost:%d/", s.upstream.LocalPort)),
	)
	retry.RunWith(&retry.Timer{Timeout: time.Minute * 1, Wait: time.Millisecond * 500}, t, func(r *retry.R) {
		req, err := http.NewRequest(http.MethodGet, url200, nil)
		require.NoError(r, err)
		res, err := client.Do(req)
		require.NoError(r, err)
		defer res.Body.Close()
		require.True(r, res.StatusCode >= 500 && res.StatusCode < 600 && res.StatusCode != nonceStatus)
	})
}

// Executes the JQ filter against the given JSON string.
// Iff there is one result, return that.
func jqOne(config, filter string) (interface{}, error) {
	query, err := gojq.Parse(filter)
	if err != nil {
		return nil, err
	}

	var m interface{}
	err = json.Unmarshal([]byte(config), &m)
	if err != nil {
		return nil, err
	}

	iter := query.Run(m)
	result := []interface{}{}
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}
		result = append(result, v)
	}
	if len(result) != 1 {
		return nil, fmt.Errorf("required result of len 1, but is %d: %v", len(result), result)
	}
	return result[0], nil
}
