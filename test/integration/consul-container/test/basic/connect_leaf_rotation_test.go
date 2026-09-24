// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package basic

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// These mirror the unexported SDS resource names emitted by agent/xds for a
// Connect sidecar's own leaf certificate and CA roots.
const (
	connectLeafSecretName = "connect-leaf"
	connectRootSecretName = "connect-root"
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
		shortDrainSidecar,
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
		requirePublicListenerUsesSDS(t, serverAdminPort)
	})

	t.Run("leaf rotation does not drain established connections", func(t *testing.T) {
		upstreamAddr := fmt.Sprintf("localhost:%d", upstreamPort)

		// Make sure the path works at all before we depend on it staying up.
		libassert.HTTPServiceEchoes(t, "localhost", upstreamPort, "")

		conn := dialMesh(t, upstreamAddr)
		require.NoError(t, conn.exchange(), "long-lived connection was not usable before rotation")

		serialBefore := envoyLeafSerial(t, serverAdminPort)

		rotateConnectCARoot(t, client)

		// Wait for the new leaf to actually reach Envoy, otherwise we would be
		// asserting on a rotation that never happened.
		retry.RunWith(&retry.Timer{Timeout: 90 * time.Second, Wait: 2 * time.Second}, t, func(r *retry.R) {
			require.NotEqual(r, serialBefore, envoyLeafSerial(r, serverAdminPort),
				"envoy is still serving the pre-rotation leaf certificate")
		})

		// Keep using the *same* TCP connection across, and well past, the
		// rotation. With a drain time of 5s, a regression closes this
		// connection long before the loop finishes.
		for i := 0; i < 10; i++ {
			require.NoErrorf(t, conn.exchange(),
				"long-lived connection was terminated by the leaf certificate rotation (exchange %d)", i+1)
			time.Sleep(time.Second)
		}

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

// requirePublicListenerUsesSDS asserts that the sidecar's public listener
// references its leaf and roots by SDS name and carries no inline certificate
// material, and that Envoy has actually received those secrets.
func requirePublicListenerUsesSDS(t *testing.T, adminPort int) {
	t.Helper()

	var dump string
	retry.RunWith(&retry.Timer{Timeout: 60 * time.Second, Wait: time.Second}, t, func(r *retry.R) {
		out, _, err := libassert.GetEnvoyOutput(adminPort, "config_dump", nil)
		require.NoError(r, err, "could not fetch envoy config dump")
		require.Contains(r, out, "public_listener", "envoy has not received the public listener yet")
		dump = out
	})

	var cfgDump struct {
		Configs []json.RawMessage `json:"configs"`
	}
	require.NoError(t, json.Unmarshal([]byte(dump), &cfgDump))

	var listeners, secrets string
	for _, raw := range cfgDump.Configs {
		var typed struct {
			Type string `json:"@type"`
		}
		require.NoError(t, json.Unmarshal(raw, &typed))

		switch {
		case strings.Contains(typed.Type, "ListenersConfigDump"):
			listeners = string(raw)
		case strings.Contains(typed.Type, "SecretsConfigDump"):
			secrets = string(raw)
		}
	}

	require.NotEmpty(t, listeners, "config dump contained no listeners")

	require.Contains(t, listeners, connectLeafSecretName,
		"the public listener does not reference the leaf certificate over SDS")
	require.Contains(t, listeners, connectRootSecretName,
		"the public listener does not reference the CA roots over SDS")
	require.NotContains(t, listeners, "inline_string",
		"certificate material is still embedded inline in the listener, so rotation will rebuild it")

	require.NotEmpty(t, secrets, "config dump contained no secrets section")
	require.Contains(t, secrets, connectLeafSecretName,
		"envoy never received the leaf certificate as an SDS secret")
	require.Contains(t, secrets, connectRootSecretName,
		"envoy never received the CA roots as an SDS secret")
}

// shortDrainSidecar appends Envoy drain flags to the sidecar's command so that
// a filter chain drain (the regression this test guards against) closes
// connections within the lifetime of the test instead of Envoy's 600s default.
func shortDrainSidecar(req testcontainers.ContainerRequest) testcontainers.ContainerRequest {
	req.Cmd = append(req.Cmd,
		"--drain-time-s", "5",
		"--parent-shutdown-time-s", "10",
		"--drain-strategy", "immediate",
	)
	return req
}

// meshConn is a single long-lived TCP connection through the mesh that is
// reused for many request/response exchanges, emulating a long-lived protocol
// such as AMQP riding on top of a Connect TCP proxy.
type meshConn struct {
	conn net.Conn
	br   *bufio.Reader
	addr string
}

func dialMesh(t *testing.T, addr string) *meshConn {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	require.NoError(t, err, "could not open a connection through the mesh")
	t.Cleanup(func() { _ = conn.Close() })

	return &meshConn{conn: conn, br: bufio.NewReader(conn), addr: addr}
}

// exchange performs one keep-alive HTTP round trip on the existing connection.
// It deliberately does not use http.Client, because the whole point of the test
// is that the *same* TCP connection survives; a client with connection pooling
// would silently redial and mask a drain.
func (m *meshConn) exchange() error {
	if err := m.conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, "http://"+m.addr+"/", nil)
	if err != nil {
		return err
	}
	if err := req.Write(m.conn); err != nil {
		return fmt.Errorf("write on long-lived connection failed: %w", err)
	}

	resp, err := http.ReadResponse(m.br, req)
	if err != nil {
		return fmt.Errorf("read on long-lived connection failed: %w", err)
	}
	defer resp.Body.Close()

	// The body must be drained so the connection stays usable for the next
	// exchange rather than being poisoned by unread bytes.
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("draining response body failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d on long-lived connection", resp.StatusCode)
	}
	return nil
}

// envoyLeafSerial returns the serial number of the leaf certificate Envoy is
// currently serving, which changes when the leaf is rotated.
func envoyLeafSerial(t require.TestingT, adminPort int) string {
	out, _, err := libassert.GetEnvoyOutput(adminPort, "certs", nil)
	require.NoError(t, err, "could not fetch envoy certs")

	var certs struct {
		Certificates []struct {
			CertChain []struct {
				SerialNumber string `json:"serial_number"`
			} `json:"cert_chain"`
		} `json:"certificates"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &certs))

	var serials []string
	for _, c := range certs.Certificates {
		for _, cc := range c.CertChain {
			serials = append(serials, cc.SerialNumber)
		}
	}
	require.NotEmpty(t, serials, "envoy is not serving any certificates")

	return strings.Join(serials, ",")
}

// rotateConnectCARoot forces an immediate re-issue of every leaf certificate by
// changing the built-in CA's private key parameters, which generates a new root.
// Simply lowering LeafCertTTL does not re-issue already-issued leaves, and the
// minimum LeafCertTTL is one hour, so waiting for a natural rotation is not an
// option inside a test.
func rotateConnectCARoot(t *testing.T, client *api.Client) {
	t.Helper()

	cfg, _, err := client.Connect().CAGetConfig(nil)
	require.NoError(t, err, "could not read connect CA config")

	if cfg.Config == nil {
		cfg.Config = map[string]interface{}{}
	}

	// Toggle the key type so the CA is guaranteed to produce a new root.
	if fmt.Sprintf("%v", cfg.Config["PrivateKeyType"]) == "rsa" {
		cfg.Config["PrivateKeyType"] = "ec"
		cfg.Config["PrivateKeyBits"] = 256
	} else {
		cfg.Config["PrivateKeyType"] = "rsa"
		cfg.Config["PrivateKeyBits"] = 2048
	}

	_, err = client.Connect().CASetConfig(cfg, nil)
	require.NoError(t, err, "could not rotate the connect CA root")
}
