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

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

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
// The dialer's retry loop is never restarted in this test (its single server keeps leadership throughout), so
// every reconnect comes from the loop that was started before the CA rotations. Two ways of dropping the stream
// are covered:
//
//   - The accepting server restarts.
//   - The connection drops without any restart or leader change: the accepting server is paused for longer than
//     the dialer's gRPC keepalive timeout, so the dialer closes the stream and has to redial.
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
	requireServiceReachable := func(t *testing.T) {
		libassert.HTTPServiceEchoes(t, "localhost", port, "")
		libassert.AssertFortioName(t, fmt.Sprintf("http://localhost:%d", port), libservice.StaticServerServiceName, "")
	}
	requireServiceReachable(t)

	acceptingServer := acceptingCluster.Servers()[0]
	dialingServer := dialingCluster.Servers()[0]

	// requireReconnected waits until the accepting server reports the peering ACTIVE again. Its stream status is
	// in memory, so it only becomes ACTIVE once the dialer has established a new stream. Reconnecting well within
	// the 2 minute incoming heartbeat timeout shows the dialer noticed the dropped stream right away.
	requireReconnected := func(t *testing.T) {
		retry.RunWith(&retry.Timer{Timeout: 120 * time.Second, Wait: 2 * time.Second}, t, func(r *retry.R) {
			peering, _, err := acceptingClient.Peerings().Read(context.Background(), libtopology.AcceptingPeerName, nil)
			require.NoError(r, err)
			require.NotNil(r, peering)
			require.Equal(r, api.PeeringStateActive, peering.State)
		})
		libassert.PeeringStatus(t, dialingClient, libtopology.DialingPeerName, api.PeeringStateActive)
		libassert.PeeringExports(t, acceptingClient, libtopology.AcceptingPeerName, 1)
		requireNoLogLine(t, dialingServer, "certificate signed by unknown authority")
		requireServiceReachable(t)
	}

	testutil.RunStep(t, "rotate the accepting cluster's CA, then restart the accepting server", func(t *testing.T) {
		rotateAcceptingCA(t, acceptingClient, dialingClient, "ec", 384)

		require.NoError(t, acceptingServer.Upgrade(context.Background(), acceptingServer.GetConfig()))
		libcluster.WaitForLeader(t, acceptingCluster, acceptingClient)

		requireReconnected(t)
	})

	testutil.RunStep(t, "rotate the accepting cluster's CA again, then drop the connection without a restart", func(t *testing.T) {
		rotateAcceptingCA(t, acceptingClient, dialingClient, "rsa", 2048)

		// Re-read the server: Upgrade in the previous step replaced its container.
		acceptingServer = acceptingCluster.Servers()[0]
		containerID := acceptingServer.GetConsulContainer().GetContainerID()
		leaderBefore := dialingLeader(t, dialingClient)

		// The dialer pings every 30s and gives up after 10s without a reply, so pausing the accepting server for
		// longer than that makes the dialer close the stream and redial.
		unpause := pauseContainer(t, acceptingServer.GetConsulContainer())
		retry.RunWith(&retry.Timer{Timeout: 90 * time.Second, Wait: 2 * time.Second}, t, func(r *retry.R) {
			peering, _, err := dialingClient.Peerings().Read(context.Background(), libtopology.DialingPeerName, nil)
			require.NoError(r, err)
			require.NotNil(r, peering)
			require.NotEqual(r, api.PeeringStateActive, peering.State, "dialer has not noticed the dropped stream yet")
		})
		unpause()

		requireReconnected(t)

		require.Equal(t, containerID, acceptingCluster.Servers()[0].GetConsulContainer().GetContainerID(),
			"accepting server must not have been restarted")
		require.Equal(t, leaderBefore, dialingLeader(t, dialingClient), "dialing cluster's leader must not change")
	})
}

// rotateAcceptingCA rotates the accepting cluster's Connect CA to a new root without cross-signing, so the new root
// can't be verified with the old one. It waits until the dialer has stored the new root while the peering stream
// stays connected.
func rotateAcceptingCA(t *testing.T, acceptingClient, dialingClient *api.Client, keyType string, keyBits int) {
	t.Helper()

	roots, _, err := acceptingClient.Connect().CARoots(nil)
	require.NoError(t, err)
	oldRootID := roots.ActiveRootID

	_, err = acceptingClient.Connect().CASetConfig(&api.CAConfig{
		Provider:                 "consul",
		ForceWithoutCrossSigning: true,
		Config: map[string]interface{}{
			"PrivateKeyType": keyType,
			"PrivateKeyBits": keyBits,
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

	// Give the dialer's peering syncer time to process the update while the stream is still connected. The syncer
	// doesn't restart connected streams, which is what leaves the dialer's retry loop with the roots it started with.
	time.Sleep(5 * time.Second)
}

// pauseContainer pauses the container and returns a function that unpauses it. The container is also unpaused when
// the test ends.
func pauseContainer(t *testing.T, c testcontainers.Container) func() {
	t.Helper()

	ctx := context.Background()
	cli, err := testcontainers.NewDockerClientWithOpts(ctx)
	require.NoError(t, err)

	id := c.GetContainerID()
	_, err = cli.ContainerPause(ctx, id, client.ContainerPauseOptions{})
	require.NoError(t, err)

	paused := true
	unpause := func() {
		if paused {
			_, err := cli.ContainerUnpause(ctx, id, client.ContainerUnpauseOptions{})
			require.NoError(t, err)
			paused = false
		}
	}
	t.Cleanup(func() {
		if paused {
			_, _ = cli.ContainerUnpause(ctx, id, client.ContainerUnpauseOptions{})
		}
		_ = cli.Close()
	})
	return unpause
}

func dialingLeader(t *testing.T, c *api.Client) string {
	t.Helper()
	leader, err := c.Status().Leader()
	require.NoError(t, err)
	require.NotEmpty(t, leader)
	return leader
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
