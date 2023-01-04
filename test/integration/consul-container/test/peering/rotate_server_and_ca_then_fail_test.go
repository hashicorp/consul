package peering

import (
	"context"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libagent "github.com/hashicorp/consul/test/integration/consul-container/libs/agent"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/test/integration/consul-container/test/topology"
)

// TestPeering_RotateServerAndCAThenFail_
// This test runs a few scenarios back to back
//  1. It makes sure that the peering stream send server address updates between peers.
//     It also verifies that dialing clusters will use this stored information to supersede the addresses
//     encoded in the peering token.
//  2. Rotate the CA in the exporting cluster and ensure services don't break
//  3. Terminate the server nodes in the exporting cluster and make sure the importing cluster can still dial it's
//     upstream.
//
// ## Steps
//
// ### Setup
//   - Setup the basic peering topology: 2 clusters, exporting service from accepting cluster to dialing cluster
//
// ### Part 1
//   - Incrementally replace the follower nodes.
//   - Replace the leader agent
//   - Verify the dialer can reach the new server nodes and the service becomes available.
//
// ### Part 2
//   - Push an update to the CA Configuration in the exporting cluster and wait for the new root to be generated
//   - Verify envoy client sidecar has two certificates for the upstream server
//   - Make sure there is still service connectivity from the importing cluster
//
// ### Part 3
//   - Terminate the server nodes in the exporting cluster
//   - Make sure there is still service connectivity from the importing cluster
func TestPeering_RotateServerAndCAThenFail_(t *testing.T) {
	acceptingCluster, dialingCluster, _, staticClientSvcSidecar := topology.BasicPeeringTwoClustersSetup(t, *utils.TargetVersion)
	defer func() {
		err := acceptingCluster.Terminate()
		require.NoErrorf(t, err, "termining accepting cluster")
		dialingCluster.Terminate()
		require.NoErrorf(t, err, "termining dialing cluster")
	}()

	dialingClient, err := dialingCluster.GetClient(nil, false)
	require.NoError(t, err)
	_, port := staticClientSvcSidecar.GetAddr()

	acceptingClient, err := acceptingCluster.GetClient(nil, false)
	require.NoError(t, err)

	t.Run("test rotating servers", func(t *testing.T) {

		// Start by replacing the Followers
		leader, err := acceptingCluster.Leader()
		require.NoError(t, err)

		followers, err := acceptingCluster.Followers()
		require.NoError(t, err)
		require.Len(t, followers, 2)

		for idx, follower := range followers {
			t.Log("Removing follower", idx)
			rotateServer(t, acceptingCluster, acceptingClient, acceptingCluster.BuildContext, follower)
		}

		t.Log("Removing leader")
		rotateServer(t, acceptingCluster, acceptingClient, acceptingCluster.BuildContext, leader)

		libassert.PeeringStatus(t, acceptingClient, topology.AcceptingPeerName, api.PeeringStateActive)
		libassert.PeeringExports(t, acceptingClient, topology.AcceptingPeerName, 1)

		libassert.HTTPServiceEchoes(t, "localhost", port)
	})

	t.Run("rotate exporting cluster's root CA", func(t *testing.T) {
		// we will verify that the peering on the dialing side persists the updates CAs
		peeringBefore, peerMeta, err := dialingClient.Peerings().Read(context.Background(), topology.DialingPeerName, &api.QueryOptions{})
		require.NoError(t, err)

		_, caMeta, err := acceptingClient.Connect().CAGetConfig(&api.QueryOptions{})
		require.NoError(t, err)

		// There should be one root cert
		rootList, _, err := acceptingClient.Connect().CARoots(&api.QueryOptions{})
		require.NoError(t, err)
		require.Len(t, rootList.Roots, 1)

		req := &api.CAConfig{
			Provider: "consul",
			Config: map[string]interface{}{
				"PrivateKeyType": "ec",
				"PrivateKeyBits": 384,
			},
		}
		_, err = acceptingClient.Connect().CASetConfig(req, &api.WriteOptions{})
		require.NoError(t, err)

		// wait up to 30 seconds for the update
		_, _, err = acceptingClient.Connect().CAGetConfig(&api.QueryOptions{
			WaitIndex: caMeta.LastIndex,
			WaitTime:  30 * time.Second,
		})
		require.NoError(t, err)

		// The peering object should reflect the update
		peeringAfter, _, err := dialingClient.Peerings().Read(context.Background(), topology.DialingPeerName, &api.QueryOptions{
			WaitIndex: peerMeta.LastIndex,
			WaitTime:  30 * time.Second,
		})
		require.NotEqual(t, peeringBefore.PeerCAPems, peeringAfter.PeerCAPems)
		require.Len(t, peeringAfter.PeerCAPems, 2)
		require.NoError(t, err)

		// There should be two root certs now on the accepting side
		rootList, _, err = acceptingClient.Connect().CARoots(&api.QueryOptions{})
		require.NoError(t, err)
		require.Len(t, rootList.Roots, 2)

		// Connectivity should still be contained
		_, port := staticClientSvcSidecar.GetAddr()
		libassert.HTTPServiceEchoes(t, "localhost", port)

		verifySidecarHasTwoRootCAs(t, staticClientSvcSidecar)
	})

	t.Run("terminate exporting clusters servers and ensure imported services are still reachable", func(t *testing.T) {
		// Keep this list for later
		newNodes, err := acceptingCluster.Clients()
		require.NoError(t, err)

		serverNodes, err := acceptingCluster.Servers()
		require.NoError(t, err)
		for _, node := range serverNodes {
			require.NoError(t, node.Terminate())
		}

		// Remove the nodes from the cluster to prevent double-termination
		acceptingCluster.Agents = newNodes

		// ensure any transitory actions like replication cleanup would not affect the next verifications
		time.Sleep(30 * time.Second)

		_, port := staticClientSvcSidecar.GetAddr()
		libassert.HTTPServiceEchoes(t, "localhost", port)
	})
}

func terminate(t *testing.T, cluster *libcluster.Cluster) {
	err := cluster.Terminate()
	require.NoError(t, err)
}

// rotateServer add a new server agent to the cluster, then forces the prior agent to leave.
func rotateServer(t *testing.T, cluster *libcluster.Cluster, client *api.Client, ctx *libagent.BuildContext, node libagent.Agent) {
	conf, err := libagent.NewConfigBuilder(cluster.BuildContext).
		Bootstrap(0).
		Peering(true).
		RetryJoin("agent-3"). // Always use the client agent since it never leaves the cluster
		ToAgentConfig()
	require.NoError(t, err)

	err = cluster.Add([]libagent.Config{*conf})
	require.NoError(t, err, "could not start new node")

	libcluster.WaitForMembers(t, client, 5)

	require.NoError(t, cluster.Remove(node))

	libcluster.WaitForMembers(t, client, 4)
}

func verifySidecarHasTwoRootCAs(t *testing.T, sidecar libservice.Service) {
	connectContainer, ok := sidecar.(*libservice.ConnectContainer)
	require.True(t, ok)
	_, adminPort := connectContainer.GetAdminAddr()

	failer := func() *retry.Timer {
		return &retry.Timer{Timeout: 30 * time.Second, Wait: 1 * time.Second}
	}

	retry.RunWith(failer(), t, func(r *retry.R) {
		dump, err := libservice.GetEnvoyConfigDump(adminPort)
		if err != nil {
			r.Fatal("could not curl envoy configuration")
		}

		// Make sure there are two certs in the sidecar
		filter := `.configs[] | select(.["@type"] | contains("type.googleapis.com/envoy.admin.v3.ClustersConfigDump")).dynamic_active_clusters[] | select(.cluster.name | contains("static-server.default.dialing-to-acceptor.external")).cluster.transport_socket.typed_config.common_tls_context.validation_context.trusted_ca.inline_string`
		results, err := utils.JQFilter(dump, filter)
		if err != nil {
			r.Fatal("could not parse envoy configuration")
		}
		if len(results) != 1 {
			r.Fatal("could not find certificates in cluster TLS context")
		}

		rest := []byte(results[0])
		var count int
		for len(rest) > 0 {
			var p *pem.Block
			p, rest = pem.Decode(rest)
			if p == nil {
				break
			}
			count++
		}

		if count != 2 {
			r.Fatalf("expected 2 TLS certificates and %d present", count)
		}
	})
}
