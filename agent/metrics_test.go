package agent

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/tlsutil"

	"github.com/stretchr/testify/require"
)

func skipIfShortTesting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
}

func recordPromMetrics(t *testing.T, a *TestAgent, respRec *httptest.ResponseRecorder) {
	t.Helper()
	req, err := http.NewRequest("GET", "/v1/agent/metrics?format=prometheus", nil)
	require.NoError(t, err, "Failed to generate new http request.")

	_, err = a.srv.AgentMetrics(respRec, req)
	require.NoError(t, err, "Failed to serve agent metrics")

}

func assertMetricExistsWithValue(t *testing.T, respRec *httptest.ResponseRecorder, metric string, value string) {
	if respRec.Body.String() == "" {
		t.Fatalf("Response body is empty.")
	}

	// eg "consul_autopilot_healthy NaN"
	target := metric + " " + value

	if !strings.Contains(respRec.Body.String(), target) {
		t.Fatalf("Could not find the metric \"%s\" with value \"%s\" in the /v1/agent/metrics response", metric, value)
	}
}

func assertMetricNotExists(t *testing.T, respRec *httptest.ResponseRecorder, metric string) {
	if respRec.Body.String() == "" {
		t.Fatalf("Response body is empty.")
	}

	if strings.Contains(respRec.Body.String(), metric) {
		t.Fatalf("Didn't expect to find the metric \"%s\" in the /v1/agent/metrics response", metric)
	}
}

// TestHTTPHandlers_AgentMetrics_ConsulAutopilot_Prometheus adds testing around
// the published autopilot metrics on https://www.consul.io/docs/agent/telemetry#autopilot
func TestHTTPHandlers_AgentMetrics_ConsulAutopilot_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("Check consul_autopilot_* are not emitted metrics on clients", func(t *testing.T) {
		hcl := `
		telemetry = {
			prometheus_retention_time = "5s"
			disable_hostname = true
			metrics_prefix = "agent_1"
		}
		bootstrap = false
		server = false
	`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		assertMetricNotExists(t, respRec, "agent_1_autopilot_healthy")
		assertMetricNotExists(t, respRec, "agent_1_autopilot_failure_tolerance")
	})

	t.Run("Check consul_autopilot_healthy metric value on startup", func(t *testing.T) {
		// don't bootstrap agent so as not to
		// become a leader
		hcl := `
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_2"
		}
		bootstrap = false
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		assertMetricExistsWithValue(t, respRec, "agent_2_autopilot_healthy", "NaN")
		assertMetricExistsWithValue(t, respRec, "agent_2_autopilot_failure_tolerance", "NaN")
	})
}

func TestHTTPHandlers_AgentMetrics_TLSCertExpiry_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	dir := testutil.TempDir(t, "ca")
	caPEM, caPK, err := tlsutil.GenerateCA(tlsutil.CAOpts{Days: 20, Domain: "consul"})
	require.NoError(t, err)

	caPath := filepath.Join(dir, "ca.pem")
	err = ioutil.WriteFile(caPath, []byte(caPEM), 0600)
	require.NoError(t, err)

	signer, err := tlsutil.ParseSigner(caPK)
	require.NoError(t, err)

	pem, key, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          caPEM,
		Name:        "server.dc1.consul",
		Days:        20,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})
	require.NoError(t, err)

	certPath := filepath.Join(dir, "cert.pem")
	err = ioutil.WriteFile(certPath, []byte(pem), 0600)
	require.NoError(t, err)

	keyPath := filepath.Join(dir, "cert.key")
	err = ioutil.WriteFile(keyPath, []byte(key), 0600)
	require.NoError(t, err)

	hcl := fmt.Sprintf(`
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_3"
		}
		ca_file = "%s"
		cert_file = "%s"
		key_file = "%s"
	`, caPath, certPath, keyPath)

	a := StartTestAgent(t, TestAgent{HCL: hcl})
	defer a.Shutdown()

	respRec := httptest.NewRecorder()
	recordPromMetrics(t, a, respRec)

	require.Contains(t, respRec.Body.String(), "agent_3_agent_tls_cert_expiry 1.7")
}
