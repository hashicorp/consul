// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"

	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// TestPeering_DialerReconnectsAfterAcceptorCARotation verifies that a dialing cluster reconnects to an accepting
// cluster that rotated its Connect CA without cross-signing while the peering was connected.
//
// The dialer receives the accepting cluster's new root over the peering stream while it is connected. When the
// stream is later re-established, the dialer must verify the accepting servers' new peering certificates with
// those updated roots. Previously the dialer kept using the roots it had when its retry loop started, so every
// reconnect failed with "x509: certificate signed by unknown authority" until the dialer's leader changed.
//
// ## Steps
//
//   - Set up the basic peering topology: the accepting cluster (dc1) exports static-server to the dialing cluster (dc2).
//   - Rotate dc1's CA to a new root with ForceWithoutCrossSigning, so the new root can't be verified with the old one.
//   - Wait until the dialer has stored dc1's new active root, while the peering stream stays connected.
//   - Restart dc1's server. It comes back presenting a peering certificate issued by the new root, and the dialer
//     has to reconnect.
//   - Verify that the stream is re-established promptly, that the dialer never failed TLS verification, and that
//     the imported service is still reachable from dc2.
func TestPeering_DialerReconnectsAfterAcceptorCARotation(t *testing.T) {
	t.Parallel()

	accepting, dialing := libtopology.BasicPeeringTwoClustersSetup(t, utils.GetTargetImageName(), utils.TargetVersion,
		libtopology.PeeringClusterSize{
			AcceptingNumServers: 1,
			AcceptingNumClients: 1,
			DialingNumServers:   1,
			DialingNumClients:   1,
		},
		false)
	var (
		acceptingCluster     = accepting.Cluster
		dialingCluster       = dialing.Cluster
		clientSidecarService = dialing.Container
	)

	acceptingClient, err := acceptingCluster.GetClient(nil, false)
	require.NoError(t, err)
	dialingClient, err := dialingCluster.GetClient(nil, false)
	require.NoError(t, err)

	libassert.PeeringStatus(t, dialingClient, libtopology.DialingPeerName, api.PeeringStateActive)
	libassert.PeeringExports(t, acceptingClient, libtopology.AcceptingPeerName, 1)

	_, port := clientSidecarService.GetAddr()
	libassert.HTTPServiceEchoes(t, "localhost", port, "")
	libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName, "")

	testutil.RunStep(t, "rotate the accepting cluster's CA without cross-signing", func(t *testing.T) {
		roots, _, err := acceptingClient.Connect().CARoots(nil)
		require.NoError(t, err)
		oldRootID := roots.ActiveRootID

		_, err = acceptingClient.Connect().CASetConfig(&api.CAConfig{
			Provider:                 "consul",
			ForceWithoutCrossSigning: true,
			Config: map[string]interface{}{
				"PrivateKeyType": "ec",
				"PrivateKeyBits": 384,
			},
		}, nil)
		require.NoError(t, err)

		var newRoot string
		retry.RunWith(&retry.Timer{Timeout: 30 * time.Second, Wait: time.Second}, t, func(r *retry.R) {
			roots, _, err := acceptingClient.Connect().CARoots(nil)
			require.NoError(r, err)
			require.NotEqual(r, oldRootID, roots.ActiveRootID, "accepting cluster has not rotated its root yet")
			for _, root := range roots.Roots {
				if root.Active {
					newRoot = root.RootCertPEM
				}
			}
			require.NotEmpty(r, newRoot)
		})

		// The dialer receives and stores the new root over the connected stream.
		retry.RunWith(&retry.Timer{Timeout: 60 * time.Second, Wait: time.Second}, t, func(r *retry.R) {
			peering, _, err := dialingClient.Peerings().Read(context.Background(), libtopology.DialingPeerName, nil)
			require.NoError(r, err)
			require.NotNil(r, peering)
			require.Equal(r, api.PeeringStateActive, peering.State)
			var found bool
			for _, pem := range peering.PeerCAPems {
				found = found || strings.TrimSpace(pem) == strings.TrimSpace(newRoot)
			}
			require.True(r, found, "dialer has not stored the accepting cluster's new root yet")
		})

		// Give the dialer's peering syncer time to process the update while the stream is still connected. The
		// syncer doesn't restart connected streams, which is what leaves the dialer's retry loop with the roots
		// it started with.
		time.Sleep(5 * time.Second)
	})

	testutil.RunStep(t, "restart the accepting server and verify the dialer reconnects", func(t *testing.T) {
		servers := acceptingCluster.Servers()
		require.Len(t, servers, 1)
		server := servers[0]
		require.NoError(t, server.Upgrade(context.Background(), server.GetConfig()))
		libcluster.WaitForLeader(t, acceptingCluster, acceptingClient)

		// The restarted server tracks streams in memory, so its peering only reports ACTIVE once the dialer has
		// established a new stream. The incoming heartbeat timeout is 2 minutes; reconnecting well within that
		// shows the dialer noticed the dropped stream right away.
		retry.RunWith(&retry.Timer{Timeout: 90 * time.Second, Wait: 2 * time.Second}, t, func(r *retry.R) {
			peering, _, err := acceptingClient.Peerings().Read(context.Background(), libtopology.AcceptingPeerName, nil)
			require.NoError(r, err)
			require.NotNil(r, peering)
			require.Equal(r, api.PeeringStateActive, peering.State)
		})
		libassert.PeeringStatus(t, dialingClient, libtopology.DialingPeerName, api.PeeringStateActive)
		libassert.PeeringExports(t, acceptingClient, libtopology.AcceptingPeerName, 1)

		requireNoLogLine(t, dialingCluster.Servers()[0], "certificate signed by unknown authority")

		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName, "")
	})
}

// requireNoLogLine fails the test if the agent's container log contains substr.
func requireNoLogLine(t *testing.T, agent libcluster.Agent, substr string) {
	t.Helper()

	rc, err := agent.Logs(context.Background())
	require.NoError(t, err)
	defer rc.Close()

	logs, err := io.ReadAll(rc)
	require.NoError(t, err)

	for _, line := range strings.Split(string(logs), "\n") {
		if strings.Contains(line, substr) {
			t.Fatalf("agent %s logged %q: %s", agent.GetName(), substr, line)
		}
	}
}
