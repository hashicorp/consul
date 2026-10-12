// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package basic

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcerts "github.com/hashicorp/consul/test/integration/consul-container/libs/certs"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// TestConnectProxyLeafCertRotationIsHitless asserts that a Connect sidecar's
// leaf certificate is delivered to Envoy over SDS rather than being embedded
// inline in the public listener, and that rotating the leaf therefore does not
// rebuild the listener or drain established connections.
//
// Before leaf certs were served over SDS, the cert/key/CA bytes were inlined
// into the DownstreamTlsContext on the public listener. Every rotation changed
// the listener proto, which changed the filter chain hash, which made Envoy
// drain the old filter chain and terminate every established connection. That
// is invisible to short-lived HTTP requests but fatal to long-lived TCP
// workloads (e.g. AMQP), which saw a full disconnect wave once per leaf TTL.
//
// Steps:
//   - Create a single agent cluster with a tcp static-server + sidecar and a
//     static-client sidecar pointed at it. The server sidecar is given a short
//     Envoy drain time so that a regression fails fast rather than hanging on
//     Envoy's 10 minute default.
//   - Assert the server sidecar's public listener carries SDS references
//     (connect-leaf / connect-root) and no inline certificate material, and
//     that Envoy's secrets config dump is populated.
//   - Open a long-lived TCP connection through the mesh and keep it busy.
//   - Force a CA root rotation, which re-issues every leaf immediately.
//   - Assert the leaf actually rotated, that the long-lived connection is still
//     usable, and that Envoy never drained a filter chain.
func TestConnectProxyLeafCertRotationIsHitless(t *testing.T) {
	t.Parallel()

	cluster, _, _ := topology.NewCluster(t, &topology.ClusterConfig{
		NumServers:                1,
		NumClients:                0,
		ApplyDefaultProxySettings: true,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:             "dc1",
			InjectAutoEncryption:   true,
			InjectGossipEncryption: true,
			AllowHTTPAnyway:        true,
		},
	})

	node := cluster.Agents[0]
	client := node.GetClient()

	// A raw TCP proxy is what long-lived protocols such as AMQP use, and it is
	// where a filter chain drain is immediately fatal.
	ok, _, err := client.ConfigEntries().Set(&api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     libservice.StaticServerServiceName,
		Protocol: "tcp",
	}, nil)
	require.NoError(t, err, "error writing tcp service-defaults")
	require.True(t, ok, "did not write tcp service-defaults")

	_, serverSidecar, err := libservice.CreateAndRegisterStaticServerAndSidecarWithCustomContainerConfig(
		node,
		&libservice.ServiceOpts{
			Name:     libservice.StaticServerServiceName,
			ID:       libservice.StaticServerServiceName,
			HTTPPort: 8080,
			GRPCPort: 8079,
		},
		libcerts.ShortDrainEnvoy,
	)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, libservice.StaticServerServiceName, nil)
	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticServerServiceName), nil)

	clientSidecar, err := libservice.CreateAndRegisterStaticClientSidecar(node, "", false, false, nil)
	require.NoError(t, err)
	libassert.CatalogServiceExists(t, client, fmt.Sprintf("%s-sidecar-proxy", libservice.StaticClientServiceName), nil)

	_, upstreamPort := clientSidecar.GetAddr()
	_, clientAdminPort := clientSidecar.GetAdminAddr()
	_, serverAdminPort := serverSidecar.GetAdminAddr()

	libassert.AssertUpstreamEndpointStatus(t, clientAdminPort, "static-server.default", "HEALTHY", 1)

	t.Run("public listener uses SDS instead of inline certificates", func(t *testing.T) {
		listeners, secrets := libcerts.ListenerAndSecretDumps(t, serverAdminPort, "public_listener")

		require.Contains(t, listeners, libcerts.ConnectLeafSecretName,
			"the public listener does not reference the leaf certificate over SDS")
		require.Contains(t, listeners, libcerts.ConnectRootSecretName,
			"the public listener does not reference the CA roots over SDS")
		require.NotContains(t, listeners, "inline_string",
			"certificate material is still embedded inline in the listener, so rotation will rebuild it")

		require.NotEmpty(t, secrets, "config dump contained no secrets section")
		require.Contains(t, secrets, libcerts.ConnectLeafSecretName,
			"envoy never received the leaf certificate as an SDS secret")
		require.Contains(t, secrets, libcerts.ConnectRootSecretName,
			"envoy never received the CA roots as an SDS secret")
	})

	t.Run("leaf rotation does not drain established connections", func(t *testing.T) {
		upstreamAddr := fmt.Sprintf("localhost:%d", upstreamPort)

		// Make sure the path works at all before we depend on it staying up.
		libassert.HTTPServiceEchoes(t, "localhost", upstreamPort, "")

		conn := libcerts.DialMesh(t, upstreamAddr, "/")
		require.NoError(t, conn.Exchange(), "long-lived connection was not usable before rotation")

		serialBefore := libcerts.EnvoyLeafSerial(t, serverAdminPort)

		libcerts.RotateConnectCARoot(t, client)

		// Wait for the new leaf to actually reach Envoy, otherwise we would be
		// asserting on a rotation that never happened.
		libcerts.RequireLeafRotated(t, serverAdminPort, serialBefore)

		// Keep using the *same* TCP connection across, and well past, the
		// rotation. With a drain time of 5s, a regression closes this
		// connection long before the loop finishes.
		libcerts.RequireSurvivesRotation(t, conn)

		// The connection surviving is the user-visible guarantee; no drain at
		// all is the mechanism, and asserting it gives a much clearer failure.
		logs, err := serverSidecar.GetLogs()
		require.NoError(t, err)
		require.NotContains(t, logs, "draining 1 filter chains",
			"envoy drained a filter chain, so the public listener was rebuilt on leaf rotation")

		// The listener should have been programmed exactly once, at startup.
		require.Equal(t, 1, strings.Count(logs, "lds: add/update listener 'public_listener"),
			"the public listener was updated more than once, so leaf rotation is still churning LDS")
	})
}
