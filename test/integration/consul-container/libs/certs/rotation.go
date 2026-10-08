// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

// Package libcerts contains helpers for exercising Connect leaf certificate
// rotation from integration tests: forcing a rotation, observing which leaf
// Envoy is currently serving, and holding a long-lived connection open across
// the rotation to prove it is not disrupted.
package libcerts

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
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
)

// SDS resource names emitted by agent/xds for CA roots and for a Connect
// sidecar's own leaf certificate. Terminating gateways hold one leaf per linked
// service and so use TerminatingGatewayLeafSecretName instead.
const (
	ConnectLeafSecretName = "connect-leaf"
	ConnectRootSecretName = "connect-root"
)

// TerminatingGatewayLeafSecretName returns the SDS resource name of the leaf
// certificate a terminating gateway presents on behalf of a linked service.
// It mirrors the naming used by agent/xds.
func TerminatingGatewayLeafSecretName(service string) string {
	return service + "-connect-leaf"
}

// ShortDrainEnvoy appends Envoy drain flags to a container's command so that a
// filter chain drain closes connections within the lifetime of a test instead
// of Envoy's 600s default. Without this a regression looks like a pass, because
// the connection outlives the test rather than the drain.
func ShortDrainEnvoy(req testcontainers.ContainerRequest) testcontainers.ContainerRequest {
	req.Cmd = append(req.Cmd,
		"--drain-time-s", "5",
		"--parent-shutdown-time-s", "10",
		"--drain-strategy", "immediate",
	)
	return req
}

// RotateConnectCARoot forces an immediate re-issue of every leaf certificate by
// changing the built-in CA's private key parameters, which generates a new root.
// Simply lowering LeafCertTTL does not re-issue already-issued leaves, and the
// minimum LeafCertTTL is one hour, so waiting for a natural rotation is not an
// option inside a test.
func RotateConnectCARoot(t *testing.T, client *api.Client) {
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

// EnvoyLeafSerial returns the serial numbers of the certificates Envoy is
// currently serving, which change when the leaf is rotated.
func EnvoyLeafSerial(t require.TestingT, adminPort int) string {
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

// RequireLeafRotated blocks until Envoy is serving a different certificate set
// than serialBefore, so that a test never asserts on a rotation that has not
// actually reached the proxy yet.
func RequireLeafRotated(t *testing.T, adminPort int, serialBefore string) {
	t.Helper()

	require.Eventually(t, func() bool {
		return EnvoyLeafSerial(t, adminPort) != serialBefore
	}, 90*time.Second, 2*time.Second,
		"envoy is still serving the pre-rotation leaf certificate")
}

// ListenerAndSecretDumps splits an Envoy config dump into its raw listeners and
// secrets sections, so a test can assert on what the listener references versus
// what Envoy actually received over SDS. waitFor is a substring that must be
// present in the dump before it is considered ready.
func ListenerAndSecretDumps(t *testing.T, adminPort int, waitFor string) (listeners string, secrets string) {
	t.Helper()

	var dump string
	require.Eventually(t, func() bool {
		out, _, err := libassert.GetEnvoyOutput(adminPort, "config_dump", nil)
		if err != nil || !strings.Contains(out, waitFor) {
			return false
		}
		dump = out
		return true
	}, 60*time.Second, time.Second,
		"envoy never reported a config dump containing %q", waitFor)

	var cfgDump struct {
		Configs []json.RawMessage `json:"configs"`
	}
	require.NoError(t, json.Unmarshal([]byte(dump), &cfgDump))

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
	return listeners, secrets
}

// MeshConn is a single long-lived TCP connection through the mesh that is
// reused for many request/response exchanges, emulating a long-lived protocol
// such as AMQP riding on top of a Connect TCP proxy.
type MeshConn struct {
	conn net.Conn
	br   *bufio.Reader
	addr string
	path string
}

// DialMesh opens a long-lived connection to addr. path is the HTTP path used
// for each keep-alive exchange on that connection.
func DialMesh(t *testing.T, addr, path string) *MeshConn {
	t.Helper()

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	require.NoError(t, err, "could not open a connection through the mesh")
	t.Cleanup(func() { _ = conn.Close() })

	return &MeshConn{conn: conn, br: bufio.NewReader(conn), addr: addr, path: path}
}

// Exchange performs one keep-alive HTTP round trip on the existing connection.
// It deliberately does not use http.Client, because the whole point is that the
// *same* TCP connection survives; a client with connection pooling would
// silently redial and mask a drain.
func (m *MeshConn) Exchange() error {
	if err := m.conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, "http://"+m.addr+m.path, nil)
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

// RequireSurvivesRotation keeps using the same TCP connection well past the
// rotation. With a short Envoy drain time, a regression closes this connection
// long before the loop finishes.
func RequireSurvivesRotation(t *testing.T, conn *MeshConn) {
	t.Helper()

	for i := 0; i < 10; i++ {
		require.NoErrorf(t, conn.Exchange(),
			"long-lived connection was terminated by the leaf certificate rotation (exchange %d)", i+1)
		time.Sleep(time.Second)
	}
}
