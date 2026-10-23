// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: MPL-2.0

package gateways

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcerts "github.com/hashicorp/consul/test/integration/consul-container/libs/certs"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

// TestTerminatingGatewayLeafCertRotationIsHitless asserts that the leaf
// certificate a terminating gateway presents to downstream mesh clients is
// delivered to Envoy over SDS rather than embedded inline in the gateway's
// listener, and that rotating that leaf therefore does not rebuild the listener
// or drain established connections.
//
// A terminating gateway holds one leaf per linked service, and each linked
// service gets its own filter chain on the shared :8443 listener. Before those
// leaves were served over SDS, makeFilterChainTerminatingGateway inlined the
// cert/key/CA bytes into each chain's DownstreamTlsContext. Every rotation
// changed the listener proto, which changed the filter chain hash, which made
// Envoy drain the old chain and terminate every established connection through
// it. Short-lived HTTP requests hide this; long-lived TCP workloads (e.g. AMQP)
// see a full disconnect wave once per leaf TTL.
//
// This is the terminating gateway counterpart to
// TestConnectProxyLeafCertRotationIsHitless.
//
// Steps:
//   - Create a cluster with an external (non-mesh) tcp service reachable only
//     through a terminating gateway, plus a static-client sidecar. The gateway
//     is given a short Envoy drain time so a regression fails fast rather than
//     outliving the test on Envoy's 600s default.
//   - Assert the gateway listener references the linked service's leaf and the
//     CA roots by SDS name and carries no inline certificate material.
//   - Open a long-lived TCP connection through the gateway and keep it busy.
//   - Force a CA root rotation, which re-issues every leaf immediately.
//   - Assert the leaf actually rotated, that the long-lived connection is still
//     usable, and that Envoy never drained a filter chain.
func TestTerminatingGatewayLeafCertRotationIsHitless(t *testing.T) {
	t.Parallel()
	var deferClean utils.ResettableDefer
	defer deferClean.Execute()

	cluster, _, client := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter: "dc1",
		},
		ApplyDefaultProxySettings: true,
	})
	node := cluster.Clients()[0]

	// A raw TCP proxy is what long-lived protocols such as AMQP use, and it is
	// where a filter chain drain is immediately fatal.
	ok, _, err := client.ConfigEntries().Set(&api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     externalServerName,
		Protocol: "tcp",
	}, nil)
	require.NoError(t, err, "error writing tcp service-defaults")
	require.True(t, ok, "did not write tcp service-defaults")

	// An external server that is not part of the mesh and has no proxy.
	externalServerPort := 8083
	externalServerGRPCPort := 8079
	externalServer, err := libservice.NewExampleService(context.Background(), externalServerName, externalServerPort, externalServerGRPCPort, node)
	require.NoError(t, err)
	deferClean.Add(func() {
		_ = externalServer.Terminate()
	})

	registerExternalService(t, client, externalServerName, "", "", "localhost", externalServerPort)
	createTerminatingGatewayConfigEntry(t, client, "", "", externalServerName)

	gwCfg := libservice.GatewayConfig{
		Name: api.TerminatingGateway,
		Kind: "terminating",
		// Without a short drain time a regression would keep the connection
		// alive past the end of the test and look like a pass.
		CustomContainerConfig: libcerts.ShortDrainEnvoy,
	}
	gatewayService, err := libservice.NewGatewayService(context.Background(), gwCfg, node)
	require.NoError(t, err)
	libassert.AssertContainerState(t, gatewayService, "running")
	libassert.CatalogServiceExists(t, client, gwCfg.Name, nil)

	staticClient, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false, nil)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, "static-client-sidecar-proxy", nil)

	_, gatewayAdminPort := gatewayService.GetAdminAddr()
	_, upstreamPort := staticClient.GetAddr()

	leafSecretName := libcerts.TerminatingGatewayLeafSecretName(externalServerName)

	t.Run("gateway listener uses SDS instead of inline certificates", func(t *testing.T) {
		// The gateway's downstream listener is bound to :8443, so waiting on
		// that is enough to know the listener has been programmed.
		listeners, secrets := libcerts.ListenerAndSecretDumps(t, gatewayAdminPort, ":8443")

		require.Contains(t, listeners, leafSecretName,
			"the gateway listener does not reference the linked service's leaf certificate over SDS")
		require.Contains(t, listeners, libcerts.ConnectRootSecretName,
			"the gateway listener does not reference the CA roots over SDS")

		// Peered filter chains legitimately inline their trust bundles for the
		// SPIFFE validator, but there is no peering in this topology, so any
		// inline material here means the leaf is still embedded in the listener.
		require.NotContains(t, listeners, "inline_string",
			"certificate material is still embedded inline in the gateway listener, so rotation will rebuild it")

		require.NotEmpty(t, secrets, "config dump contained no secrets section")
		require.Contains(t, secrets, leafSecretName,
			"envoy never received the linked service's leaf certificate as an SDS secret")
		require.Contains(t, secrets, libcerts.ConnectRootSecretName,
			"envoy never received the CA roots as an SDS secret")
	})

	t.Run("leaf rotation does not drain established connections", func(t *testing.T) {
		// Make sure the path through the gateway works at all before we depend
		// on it staying up.
		assertHTTPRequestToServiceAddress(t, staticClient, externalServerName, libcluster.ServiceUpstreamLocalBindPort, true)

		upstreamAddr := fmt.Sprintf("localhost:%d", upstreamPort)
		conn := libcerts.DialMesh(t, upstreamAddr, "/debug?env=dump")
		require.NoError(t, conn.Exchange(), "long-lived connection was not usable before rotation")

		serialBefore := libcerts.EnvoyLeafSerial(t, gatewayAdminPort)

		// How many times the listener had been programmed before the rotation.
		// Unlike a sidecar's public listener, a gateway listener can legitimately
		// be programmed more than once while config entries and linked service
		// state settle, so the regression is "rotation added an update", not an
		// absolute count.
		logsBefore, err := gatewayService.GetLogs()
		require.NoError(t, err)
		ldsUpdatesBefore := strings.Count(logsBefore, "lds: add/update listener")

		libcerts.RotateConnectCARoot(t, client)

		// Wait for the new leaf to actually reach the gateway, otherwise we
		// would be asserting on a rotation that never happened.
		libcerts.RequireLeafRotated(t, gatewayAdminPort, serialBefore)

		// Keep using the *same* TCP connection across, and well past, the
		// rotation. With a drain time of 5s, a regression closes this
		// connection long before the loop finishes.
		libcerts.RequireSurvivesRotation(t, conn)

		// The connection surviving is the user-visible guarantee; no drain at
		// all is the mechanism, and asserting it gives a much clearer failure.
		logs, err := gatewayService.GetLogs()
		require.NoError(t, err)
		require.NotContains(t, logs, "filter chains in listener",
			"envoy drained a filter chain, so the gateway listener was rebuilt on leaf rotation")

		// Rotating the leaf must not reprogram the listener at all; only the
		// SDS secrets should change.
		require.Equal(t, ldsUpdatesBefore, strings.Count(logs, "lds: add/update listener"),
			"the gateway listener was updated by the leaf rotation, so it is still churning LDS")
	})
}
