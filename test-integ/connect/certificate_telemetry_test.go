// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
	"github.com/hashicorp/consul/testing/deployer/sprawl/sprawltest"
	"github.com/hashicorp/consul/testing/deployer/topology"
)

func Test_CertificateTelemetry(t *testing.T) {
	const (
		clusterName = "dc1"
		serviceName = "web"
		// sprawl bootstraps clusters with this management token.
		managementToken = "root"

		certMetricTolerance = 30 * time.Second
	)

	cfg := &topology.Config{
		Images: utils.TargetImages(),
		Networks: []*topology.Network{
			{Name: clusterName},
		},
		Clusters: []*topology.Cluster{
			{
				Name:       clusterName,
				Enterprise: utils.IsEnterprise(),
				Nodes: []*topology.Node{
					{
						Kind: topology.NodeKindServer,
						Name: "dc1-server1",
						Addresses: []*topology.Address{
							{Network: clusterName},
						},
					},
				},
			},
		},
	}

	sp := sprawltest.Launch(t, cfg)
	t.Logf("launched sprawl topology for cluster=%s", clusterName)

	leaderNode, err := sp.Leader(clusterName)
	require.NoError(t, err)
	t.Logf("leader elected: node_id=%s", leaderNode.ID())

	client, err := sp.APIClientForNode(clusterName, leaderNode.ID(), "")
	require.NoError(t, err)

	httpClient, err := sp.HTTPClientForCluster(clusterName)
	require.NoError(t, err)

	leaderAddr, err := sp.LocalAddressForNode(clusterName, leaderNode.ID())
	require.NoError(t, err)
	t.Logf("leader local address resolved: %s", leaderAddr)

	serverNode := sp.Topology().Clusters[clusterName].FirstServer()
	require.NotNil(t, serverNode)
	t.Logf("server node resolved: name=%s docker=%s", serverNode.Name, serverNode.DockerName())

	err = client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:   serviceName,
		Name: serviceName,
		Port: 8080,
	})
	require.NoError(t, err)
	t.Logf("registered service for leaf issuance: service=%s", serviceName)

	leaf, _, err := client.Agent().ConnectCALeaf(serviceName, nil)
	require.NoError(t, err)
	require.NotEmpty(t, leaf.CertPEM)
	require.NotEmpty(t, leaf.PrivateKeyPEM)
	t.Logf("issued leaf certificate: service=%s valid_after=%s valid_before=%s", serviceName, leaf.ValidAfter.UTC().Format(time.RFC3339), leaf.ValidBefore.UTC().Format(time.RFC3339))

	var (
		lastActiveRootID        string
		lastActiveRootNotAfter  string
		lastConnectRootID       string
		lastConnectRootNotAfter string
		lastLeafCertCount       int
		lastLeafNotAfter        string
		lastServerNotAfter      string
		lastRootValue           float64
		lastSigningValue        float64
		lastAgentTLSValue       float64
		lastLeafValue           float64
		lastRenewalFailure      float64
	)

	retry.Run(t, func(r *retry.R) {
		logRetryStepf(r, "retry step: collecting CA roots and certificate telemetry metrics")

		agentRoots, _, err := client.Agent().ConnectCARoots(nil)
		require.NoError(r, err)
		require.NotEmpty(r, agentRoots.ActiveRootID)
		require.NotEmpty(r, agentRoots.Roots)

		activeRoot := findActiveRoot(r, agentRoots.Roots, agentRoots.ActiveRootID)
		require.NotNil(r, activeRoot.NotBefore)
		require.NotNil(r, activeRoot.NotAfter)
		require.True(r, activeRoot.NotAfter.After(*activeRoot.NotBefore))
		require.True(r, activeRoot.NotAfter.After(time.Now()))
		lastActiveRootID = agentRoots.ActiveRootID
		lastActiveRootNotAfter = activeRoot.NotAfter.UTC().Format(time.RFC3339)
		logRetryStepf(r, "retry step: agent roots loaded active_root_id=%s not_after=%s", agentRoots.ActiveRootID, activeRoot.NotAfter.UTC().Format(time.RFC3339))

		connectRoots, _, err := client.Connect().CARoots(nil)
		require.NoError(r, err)
		require.NotEmpty(r, connectRoots.ActiveRootID)
		require.NotEmpty(r, connectRoots.Roots)

		connectActiveRoot := findActiveRoot(r, connectRoots.Roots, connectRoots.ActiveRootID)
		require.NotNil(r, connectActiveRoot.NotBefore)
		require.NotNil(r, connectActiveRoot.NotAfter)
		require.True(r, connectActiveRoot.NotAfter.After(*connectActiveRoot.NotBefore))
		require.True(r, connectActiveRoot.NotAfter.After(time.Now()))
		lastConnectRootID = connectRoots.ActiveRootID
		lastConnectRootNotAfter = connectActiveRoot.NotAfter.UTC().Format(time.RFC3339)
		logRetryStepf(r, "retry step: connect roots loaded active_root_id=%s not_after=%s", connectRoots.ActiveRootID, connectActiveRoot.NotAfter.UTC().Format(time.RFC3339))

		leafCerts := parsePEMCertificates(r, leaf.CertPEM)
		lastLeafCertCount = len(leafCerts)
		lastLeafNotAfter = leafCerts[0].NotAfter.UTC().Format(time.RFC3339)
		logRetryStepf(r, "retry step: parsed leaf certificate chain cert_count=%d leaf_not_after=%s", len(leafCerts), leafCerts[0].NotAfter.UTC().Format(time.RFC3339))

		serverCert := copyAndParseServerCert(r, sp, serverNode)
		lastServerNotAfter = serverCert.NotAfter.UTC().Format(time.RFC3339)
		logRetryStepf(r, "retry step: parsed server TLS cert not_after=%s", serverCert.NotAfter.UTC().Format(time.RFC3339))

		metricsBody, err := scrapePrometheusMetrics(httpClient, leaderAddr, managementToken)
		require.NoError(r, err)
		logRetryStepf(r, "retry step: scraped prometheus metrics from leader")

		rootValue := promMetricValue(r, metricsBody, "consul_mesh_active_root_ca_expiry", fmt.Sprintf(`datacenter="%s"`, clusterName))
		signingValue := promMetricValue(r, metricsBody, "consul_mesh_active_signing_ca_expiry", fmt.Sprintf(`datacenter="%s"`, clusterName))
		agentTLSValue := promMetricValue(r, metricsBody, "consul_agent_tls_cert_expiry", fmt.Sprintf(`datacenter="%s"`, clusterName), `node="`, `role="server"`)
		leafValue := promMetricValue(r, metricsBody, "consul_leaf_certs_cert_expiry", fmt.Sprintf(`datacenter="%s"`, clusterName), `namespace="default"`, fmt.Sprintf(`service="%s"`, serviceName))
		renewalFailureValue := promMetricValue(r, metricsBody, "consul_leaf_certs_cert_renewal_failure", fmt.Sprintf(`datacenter="%s"`, clusterName), `namespace="default"`, fmt.Sprintf(`service="%s"`, serviceName))
		lastRootValue = rootValue
		lastSigningValue = signingValue
		lastAgentTLSValue = agentTLSValue
		lastLeafValue = leafValue
		lastRenewalFailure = renewalFailureValue
		logRetryStepf(r, "retry step: metric values root=%f signing=%f agent_tls=%f leaf=%f renewal_failure=%f", rootValue, signingValue, agentTLSValue, leafValue, renewalFailureValue)

		expectedSigningNotAfter := *activeRoot.NotAfter
		if len(leafCerts) >= 2 {
			expectedSigningNotAfter = leafCerts[len(leafCerts)-1].NotAfter
		}

		requireGaugeMatchesCertExpiry(r, "consul.mesh.active-root-ca.expiry", rootValue, *activeRoot.NotAfter, certMetricTolerance)
		requireGaugeMatchesCertExpiry(r, "consul.mesh.active-signing-ca.expiry", signingValue, expectedSigningNotAfter, certMetricTolerance)
		requireGaugeMatchesCertExpiry(r, "consul.agent.tls.cert.expiry", agentTLSValue, serverCert.NotAfter, certMetricTolerance)
		requireGaugeMatchesCertExpiry(r, "consul.leaf-certs.cert_expiry", leafValue, leaf.ValidBefore, certMetricTolerance)
		if len(leafCerts) >= 2 {
			require.Greater(r, rootValue, signingValue)
		}
		require.GreaterOrEqual(r, rootValue, signingValue)
		require.Greater(r, signingValue, leafValue)
		require.Equal(r, float64(0), renewalFailureValue)
		logRetryStepf(r, "retry step: all certificate telemetry assertions passed")
	})
	t.Logf("retry summary: active_root_id=%s active_root_not_after=%s connect_root_id=%s connect_root_not_after=%s", lastActiveRootID, lastActiveRootNotAfter, lastConnectRootID, lastConnectRootNotAfter)
	t.Logf("retry summary: leaf_chain_count=%d leaf_not_after=%s server_tls_not_after=%s", lastLeafCertCount, lastLeafNotAfter, lastServerNotAfter)
	t.Logf("retry summary: metric values root=%f signing=%f agent_tls=%f leaf=%f renewal_failure=%f", lastRootValue, lastSigningValue, lastAgentTLSValue, lastLeafValue, lastRenewalFailure)
	t.Logf("certificate telemetry e2e validation completed successfully")
}

func logRetryStepf(r *retry.R, format string, args ...any) {
	r.Logf(format, args...)
	fmt.Printf("[retry] "+format+"\n", args...)
}

func scrapePrometheusMetrics(httpClient *http.Client, nodeAddress string, token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, "http://"+nodeAddress+":8500/v1/agent/metrics?format=prometheus", nil)
	if err != nil {
		return "", err
	}
	if token != "" {
		req.Header.Set("X-Consul-Token", token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metrics request failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func promMetricValue(t require.TestingT, body string, metric string, fragments ...string) float64 {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	require.NotEmpty(t, body, "Prometheus metrics body is empty")

	var candidates []string
	for _, line := range strings.Split(body, "\n") {
		if len(line) < 1 || line[0] == '#' || !strings.Contains(line, metric) {
			continue
		}
		candidates = append(candidates, line)

		matches := true
		for _, fragment := range fragments {
			if !strings.Contains(line, fragment) {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}

		fields := strings.Fields(line)
		require.NotEmpty(t, fields, "metric line should contain fields")

		value, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		require.NoError(t, err, "metric line should end with a numeric value")
		return value
	}

	require.Failf(t, "metric not found", "Could not find metric %q with fragments %q. Candidate lines: %q", metric, fragments, candidates)
	return 0
}

func requireGaugeMatchesCertExpiry(t require.TestingT, name string, value float64, notAfter time.Time, tolerance time.Duration) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	expected := time.Until(notAfter).Seconds()
	diff := expected - value
	if diff < 0 {
		diff = -diff
	}
	require.LessOrEqualf(
		t,
		diff,
		tolerance.Seconds(),
		"gauge %q should be within %s of cert expiry, expected about %v seconds, got %v seconds",
		name,
		tolerance,
		expected,
		value,
	)
}

func findActiveRoot(t require.TestingT, roots []*api.CARoot, activeRootID string) *api.CARoot {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	for _, root := range roots {
		if root.ID == activeRootID {
			return root
		}
	}

	require.Failf(t, "active root not found", "Could not find active root %q", activeRootID)
	return nil
}

func parsePEMCertificates(t require.TestingT, certPEM string) []*x509.Certificate {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	var certs []*x509.Certificate
	rest := []byte(certPEM)
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		rest = remaining
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		certs = append(certs, cert)
	}

	require.NotEmpty(t, certs, "expected at least one certificate in PEM bundle")
	return certs
}

func copyAndParseServerCert(t require.TestingT, sp interface {
	CopyFileFromContainer(context.Context, string, string, string) error
}, node *topology.Node) *x509.Certificate {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	tempDir, err := os.MkdirTemp("", "cert-telemetry-server-cert-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	destPath := filepath.Join(tempDir, "server.pem")
	sourcePath := fmt.Sprintf("/consul/config/certs/%s.pem", node.TLSCertPrefix)
	err = sp.CopyFileFromContainer(context.Background(), node.DockerName(), sourcePath, destPath)
	require.NoError(t, err)

	certPEM, err := os.ReadFile(destPath)
	require.NoError(t, err)

	certs := parsePEMCertificates(t, string(certPEM))
	return certs[0]
}
