// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"crypto/x509"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
)

var metricsPrefixCounter atomic.Uint64

// getUniqueMetricsPrefix generates a unique ID for each test to use as a metrics prefix.
// This is needed because go-metrics is effectively a global variable.
func getUniqueMetricsPrefix() string {
	return fmt.Sprint("metrics_", metricsPrefixCounter.Add(1))
}

func skipIfShortTesting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
}

func recordPromMetrics(t require.TestingT, a *TestAgent, respRec *httptest.ResponseRecorder) {
	if tt, ok := t.(*testing.T); ok {
		tt.Helper()
	}

	req, err := http.NewRequest("GET", "/v1/agent/metrics?format=prometheus", nil)
	require.NoError(t, err, "Failed to generate new http request.")

	a.srv.h.ServeHTTP(respRec, req)
	require.Equalf(t, 200, respRec.Code, "expected 200, got %d, body: %s", respRec.Code, respRec.Body.String())
}

func assertMetricExists(t *testing.T, respRec *httptest.ResponseRecorder, metric string) {
	if respRec.Body.String() == "" {
		t.Fatalf("Response body is empty.")
	}

	if !strings.Contains(respRec.Body.String(), metric) {
		t.Fatalf("Could not find the metric \"%s\" in the /v1/agent/metrics response", metric)
	}
}

// assertMetricExistsWithLabels looks in the prometheus metrics response for the metric name and all the labels. eg:
// new_rpc_metrics_rpc_server_call{errored="false",method="Status.Ping",request_type="unknown",rpc_type="net/rpc"}
func assertMetricExistsWithLabels(t *testing.T, respRec *httptest.ResponseRecorder, metric string, labelNames []string) {
	if respRec.Body.String() == "" {
		t.Fatalf("Response body is empty.")
	}

	if !strings.Contains(respRec.Body.String(), metric) {
		t.Fatalf("Could not find the metric \"%s\" in the /v1/agent/metrics response", metric)
	}

	foundAllLabels := false
	metrics := respRec.Body.String()
	for _, line := range strings.Split(metrics, "\n") {
		// skip help lines
		if len(line) < 1 || line[0] == '#' {
			continue
		}

		if strings.Contains(line, metric) {
			hasAllLabels := true
			for _, labelName := range labelNames {
				if !strings.Contains(line, labelName) {
					hasAllLabels = false
					break
				}
			}

			if hasAllLabels {
				foundAllLabels = true

				// done!
				break
			}
		}
	}

	if !foundAllLabels {
		t.Fatalf("Could not verify that all named labels \"%s\" exist for the metric \"%s\" in the /v1/agent/metrics response", strings.Join(labelNames, ", "), metric)
	}
}

func assertMetricLineContainsFragments(t require.TestingT, respRec *httptest.ResponseRecorder, metric string, fragments ...string) {
	require.NotEmpty(t, respRec.Body.String(), "Response body is empty.")

	var candidates []string
	for _, line := range strings.Split(respRec.Body.String(), "\n") {
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
		if matches {
			return
		}
	}

	require.Failf(t, "metric line not found", "Could not find metric %q with fragments %q in the /v1/agent/metrics response. Candidate lines: %q", metric, fragments, candidates)
}

func metricLineValue(t require.TestingT, respRec *httptest.ResponseRecorder, metric string, fragments ...string) float64 {
	require.NotEmpty(t, respRec.Body.String(), "Response body is empty.")

	var candidates []string
	for _, line := range strings.Split(respRec.Body.String(), "\n") {
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

	require.Failf(t, "metric value not found", "Could not find metric %q with fragments %q in the /v1/agent/metrics response. Candidate lines: %q", metric, fragments, candidates)
	return 0
}

func metricAnyLineValue(t require.TestingT, respRec *httptest.ResponseRecorder, metric string) float64 {
	require.NotEmpty(t, respRec.Body.String(), "Response body is empty.")

	for _, line := range strings.Split(respRec.Body.String(), "\n") {
		if len(line) < 1 || line[0] == '#' || !strings.Contains(line, metric) {
			continue
		}

		fields := strings.Fields(line)
		require.NotEmpty(t, fields, "metric line should contain fields")

		value, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		require.NoError(t, err, "metric line should end with a numeric value")
		return value
	}

	require.Failf(t, "metric value not found", "Could not find metric %q in the /v1/agent/metrics response", metric)
	return 0
}

func assertMetricValueBetween(t require.TestingT, metric string, value float64, min float64, max float64) {
	require.GreaterOrEqualf(t, value, min, "metric %q should be >= %v, got %v", metric, min, value)
	require.LessOrEqualf(t, value, max, "metric %q should be <= %v, got %v", metric, max, value)
}

func assertLabelWithValueForMetricExistsNTime(t *testing.T, respRec *httptest.ResponseRecorder, metric string, label string, labelValue string, occurrences int) {
	if respRec.Body.String() == "" {
		t.Fatalf("Response body is empty.")
	}

	if !strings.Contains(respRec.Body.String(), metric) {
		t.Fatalf("Could not find the metric \"%s\" in the /v1/agent/metrics response", metric)
	}

	metrics := respRec.Body.String()
	// don't look at _sum or _count or other aggregates
	metricTarget := metric + "{"
	// eg method="Status.Ping"
	labelWithValueTarget := label + "=" + "\"" + labelValue + "\""

	matchesFound := 0
	for _, line := range strings.Split(metrics, "\n") {
		// skip help lines
		if len(line) < 1 || line[0] == '#' {
			continue
		}

		if strings.Contains(line, metricTarget) {
			if strings.Contains(line, labelWithValueTarget) {
				matchesFound++
			}
		}
	}

	if matchesFound < occurrences {
		t.Fatalf("Only found metric \"%s\" %d times. Wanted %d times.", metric, matchesFound, occurrences)
	}
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

// TestAgent_OneTwelveRPCMetrics test for the 1.12 style RPC metrics. These are the labeled metrics coming from
// agent.rpc.middleware.interceptors package.
func TestAgent_OneTwelveRPCMetrics(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("Check that 1.12 rpc metrics are not emitted by default.", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		telemetry = {
			prometheus_retention_time = "5s"
			disable_hostname = true
			metrics_prefix = "%s"
		}
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		var out struct{}
		err := a.RPC(context.Background(), "Status.Ping", struct{}{}, &out)
		require.NoError(t, err)

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		assertMetricNotExists(t, respRec, metricsPrefix+"_rpc_server_call")
	})

	t.Run("Check that 1.12 rpc metrics are emitted when specified by operator.", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		allowRPCMetricRule := metricsPrefix + "." + strings.Join(middleware.OneTwelveRPCSummary[0].Name, ".")
		hcl := fmt.Sprintf(`
		telemetry = {
			prometheus_retention_time = "5s"
			disable_hostname = true
			metrics_prefix = "%s"
			prefix_filter = ["+%s"]
		}
		`, metricsPrefix, allowRPCMetricRule)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		var out struct{}
		err := a.RPC(context.Background(), "Status.Ping", struct{}{}, &out)
		require.NoError(t, err)
		err = a.RPC(context.Background(), "Status.Ping", struct{}{}, &out)
		require.NoError(t, err)
		err = a.RPC(context.Background(), "Status.Ping", struct{}{}, &out)
		require.NoError(t, err)

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		// make sure the labels exist for this metric
		assertMetricExistsWithLabels(t, respRec, metricsPrefix+"_rpc_server_call", []string{"errored", "method", "request_type", "rpc_type", "leader"})
		// make sure we see 3 Status.Ping metrics corresponding to the calls we made above
		assertLabelWithValueForMetricExistsNTime(t, respRec, metricsPrefix+"_rpc_server_call", "method", "Status.Ping", 3)
	})
}

func TestHTTPHandlers_AgentMetrics_LeaderShipMetrics(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("check that metric isLeader is set properly on server", func(t *testing.T) {
		metricsPrefix1 := getUniqueMetricsPrefix()
		metricsPrefix2 := getUniqueMetricsPrefix()
		metricsPrefix3 := getUniqueMetricsPrefix()

		hcl1 := fmt.Sprintf(`
		server = true
		telemetry = {
			prometheus_retention_time = "25s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		`, metricsPrefix1)

		hcl2 := fmt.Sprintf(`
		server = true
		telemetry = {
			prometheus_retention_time = "25s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		`, metricsPrefix2)

		hcl3 := fmt.Sprintf(`
		server = true
		telemetry = {
			prometheus_retention_time = "25s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		`, metricsPrefix3)

		overrides := `
		  bootstrap = false
		  bootstrap_expect = 3
		`

		s1 := StartTestAgent(t, TestAgent{Name: "s1", HCL: hcl1, Overrides: overrides})
		defer s1.Shutdown()

		s2 := StartTestAgent(t, TestAgent{Name: "s2", HCL: hcl2, Overrides: overrides})
		defer s2.Shutdown()

		s3 := StartTestAgent(t, TestAgent{Name: "s3", HCL: hcl3, Overrides: overrides})
		defer s3.Shutdown()

		// agent hasn't become a leader
		retry.RunWith(retry.ThirtySeconds(), t, func(r *retry.R) {
			respRec := httptest.NewRecorder()
			recordPromMetrics(r, s1, respRec)
			found := strings.Contains(respRec.Body.String(), metricsPrefix1+"_server_isLeader 0")
			require.True(r, found, "non-leader server should have isLeader 0")
		})

		_, err := s2.JoinLAN([]string{s1.Config.SerfBindAddrLAN.String()}, nil)
		require.NoError(t, err)
		_, err = s3.JoinLAN([]string{s1.Config.SerfBindAddrLAN.String()}, nil)
		require.NoError(t, err)

		testrpc.WaitForLeader(t, s1.RPC, "dc1")
		testrpc.WaitForLeader(t, s2.RPC, "dc1")
		testrpc.WaitForLeader(t, s3.RPC, "dc1")

		// Verify agent's isLeader metrics is 1
		retry.RunWith(retry.ThirtySeconds(), t, func(r *retry.R) {
			respRec1 := httptest.NewRecorder()
			recordPromMetrics(r, s1, respRec1)
			found1 := strings.Contains(respRec1.Body.String(), metricsPrefix1+"_server_isLeader 1")

			respRec2 := httptest.NewRecorder()
			recordPromMetrics(r, s2, respRec2)
			found2 := strings.Contains(respRec2.Body.String(), metricsPrefix2+"_server_isLeader 1")

			respRec3 := httptest.NewRecorder()
			recordPromMetrics(r, s3, respRec3)
			found3 := strings.Contains(respRec3.Body.String(), metricsPrefix3+"_server_isLeader 1")

			require.True(r, found1 || found2 || found3, "leader server should have isLeader 1")
		})
	})
}

// TestHTTPHandlers_AgentMetrics_ConsulAutopilot_Prometheus adds testing around
// the published autopilot metrics on https://developer.hashicorp.com/docs/reference/agent/telemetry#autopilot
func TestHTTPHandlers_AgentMetrics_ConsulAutopilot_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("Check consul_autopilot_* are not emitted metrics on clients", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		telemetry = {
			prometheus_retention_time = "5s"
			disable_hostname = true
			metrics_prefix = "%s"
		}
		bootstrap = false
		server = false
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		assertMetricNotExists(t, respRec, metricsPrefix+"_autopilot_healthy")
		assertMetricNotExists(t, respRec, metricsPrefix+"_autopilot_failure_tolerance")
	})

	t.Run("Check consul_autopilot_healthy metric value on startup", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		// don't bootstrap agent so as not to
		// become a leader
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		bootstrap = false
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		assertMetricExistsWithValue(t, respRec, metricsPrefix+"_autopilot_healthy", "1")
		assertMetricExistsWithValue(t, respRec, metricsPrefix+"_autopilot_failure_tolerance", "0")
	})
}

func TestHTTPHandlers_AgentMetrics_TLSCertExpiry_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	testCertExpirationMonitorInterval = 2 * time.Second
	t.Cleanup(func() {
		testCertExpirationMonitorInterval = 0
	})

	dir := testutil.TempDir(t, "ca")
	caPEM, caPK, err := tlsutil.GenerateCA(tlsutil.CAOpts{Days: 20, Domain: "consul"})
	require.NoError(t, err)

	caPath := filepath.Join(dir, "ca.pem")
	err = os.WriteFile(caPath, []byte(caPEM), 0600)
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
	err = os.WriteFile(certPath, []byte(pem), 0600)
	require.NoError(t, err)

	keyPath := filepath.Join(dir, "cert.key")
	err = os.WriteFile(keyPath, []byte(key), 0600)
	require.NoError(t, err)

	metricsPrefix := getUniqueMetricsPrefix()
	hcl := fmt.Sprintf(`
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
			certificate = {
				enabled = true
			}
		}
		verify_incoming = true
		verify_outgoing = true
		ca_file = "%s"
		cert_file = "%s"
		key_file = "%s"
	`, metricsPrefix, caPath, certPath, keyPath)

	a := StartTestAgent(t, TestAgent{HCL: hcl})
	defer a.Shutdown()

	certTTL := 20 * 24 * time.Hour
	certWindow := 15 * time.Minute

	var firstValue float64
	retry.Run(t, func(r *retry.R) {
		respRec := httptest.NewRecorder()
		recordPromMetrics(r, a, respRec)

		firstValue = metricLineValue(
			r,
			respRec,
			metricsPrefix+"_agent_tls_cert_expiry",
			`datacenter="dc1"`,
			`partition="default"`,
			`node="`,
			`role="server"`,
		)
		assertMetricValueBetween(
			r,
			metricsPrefix+"_agent_tls_cert_expiry",
			firstValue,
			float64((certTTL - certWindow).Seconds()),
			float64(certTTL.Seconds()),
		)
	})

	time.Sleep(3 * time.Second)

	retry.Run(t, func(r *retry.R) {
		respRec := httptest.NewRecorder()
		recordPromMetrics(r, a, respRec)

		secondValue := metricLineValue(
			r,
			respRec,
			metricsPrefix+"_agent_tls_cert_expiry",
			`datacenter="dc1"`,
			`partition="default"`,
			`node="`,
			`role="server"`,
		)
		require.Less(r, secondValue, firstValue)
		require.GreaterOrEqual(r, firstValue-secondValue, 1.0)
	})
}

func TestHTTPHandlers_AgentMetrics_CACertExpiry_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	consul.TestCertExpirationMonitorInterval = 2 * time.Second
	t.Cleanup(func() {
		consul.TestCertExpirationMonitorInterval = 0
	})

	t.Run("non-leader emits NaN", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		connect {
			enabled = true
		}
		bootstrap = false
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		retry.Run(t, func(r *retry.R) {
			respRec := httptest.NewRecorder()
			recordPromMetrics(r, a, respRec)

			rootValue := metricAnyLineValue(r, respRec, metricsPrefix+"_mesh_active_root_ca_expiry")
			signingValue := metricAnyLineValue(r, respRec, metricsPrefix+"_mesh_active_signing_ca_expiry")
			require.True(r, math.IsNaN(rootValue))
			require.True(r, math.IsNaN(signingValue))
		})
	})

	t.Run("leader emits a value", func(t *testing.T) {
		const (
			leafTTL       = time.Hour
			signingTTL    = 4 * time.Hour
			rootTTL       = 5 * time.Hour
			rootWindow    = 15 * time.Minute
			signingWindow = 15 * time.Minute
		)

		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		connect {
			enabled = true
			ca_config {
				cluster_id = "%s"
				leaf_cert_ttl = "%s"
				intermediate_cert_ttl = "%s"
				root_cert_ttl = "%s"
			}
		}
		`, metricsPrefix, connect.TestClusterID, leafTTL.String(), signingTTL.String(), rootTTL.String())

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		var rootFirst, signingFirst float64
		retry.Run(t, func(r *retry.R) {
			respRec := httptest.NewRecorder()
			recordPromMetrics(r, a, respRec)

			rootFirst = metricLineValue(
				r,
				respRec,
				metricsPrefix+"_mesh_active_root_ca_expiry",
				`datacenter="dc1"`,
			)
			signingFirst = metricLineValue(
				r,
				respRec,
				metricsPrefix+"_mesh_active_signing_ca_expiry",
				`datacenter="dc1"`,
			)

			assertMetricValueBetween(
				r,
				metricsPrefix+"_mesh_active_root_ca_expiry",
				rootFirst,
				float64((rootTTL - rootWindow).Seconds()),
				float64(rootTTL.Seconds()),
			)
			inSigningRange := signingFirst >= float64((signingTTL-signingWindow).Seconds()) && signingFirst <= float64(signingTTL.Seconds())
			inRootRange := signingFirst >= float64((rootTTL-rootWindow).Seconds()) && signingFirst <= float64(rootTTL.Seconds())
			require.Truef(
				r,
				inSigningRange || inRootRange,
				"expected signing metric in either intermediate range [%v,%v] or root range [%v,%v], got %v",
				float64((signingTTL - signingWindow).Seconds()),
				float64(signingTTL.Seconds()),
				float64((rootTTL - rootWindow).Seconds()),
				float64(rootTTL.Seconds()),
				signingFirst,
			)

			// In a primary DC, some CA providers sign leaf certs directly with root,
			// so signing expiry can equal root expiry. Others use intermediate certs.
			if inSigningRange {
				require.Greater(r, rootFirst, signingFirst)
			} else {
				require.GreaterOrEqual(r, rootFirst, signingFirst)
			}
		})

		time.Sleep(3 * time.Second)

		retry.Run(t, func(r *retry.R) {
			respRec := httptest.NewRecorder()
			recordPromMetrics(r, a, respRec)

			rootSecond := metricLineValue(
				r,
				respRec,
				metricsPrefix+"_mesh_active_root_ca_expiry",
				`datacenter="dc1"`,
			)
			signingSecond := metricLineValue(
				r,
				respRec,
				metricsPrefix+"_mesh_active_signing_ca_expiry",
				`datacenter="dc1"`,
			)

			require.Less(r, rootSecond, rootFirst)
			require.Less(r, signingSecond, signingFirst)
			require.GreaterOrEqual(r, rootFirst-rootSecond, 1.0)
			require.GreaterOrEqual(r, signingFirst-signingSecond, 1.0)
		})
	})

}

func TestHTTPHandlers_AgentMetrics_LeafCertExpiry_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	testLeafCertMetricEmissionInterval = 2 * time.Second
	t.Cleanup(func() {
		testLeafCertMetricEmissionInterval = 0
	})

	t.Run("leaf metrics are emitted after issuing a leaf cert", func(t *testing.T) {
		const (
			leafTTL             = time.Hour
			signingTTL          = 4 * time.Hour
			rootTTL             = 5 * time.Hour
			leafWindow          = 5 * time.Minute
			leafCertDriftBuffer = time.Minute
		)

		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
			certificate = {
				enabled = true
			}
		}
		connect {
			enabled = true
			ca_config {
				cluster_id = "%s"
				leaf_cert_ttl = "%s"
				intermediate_cert_ttl = "%s"
				root_cert_ttl = "%s"
			}
		}
		`, metricsPrefix, connect.TestClusterID, leafTTL.String(), signingTTL.String(), rootTTL.String())

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")
		testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

		client := a.Client()
		err := client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			Name: "web",
			Port: 8080,
		})
		require.NoError(t, err)

		_, _, err = client.Agent().ConnectCALeaf("web", nil)
		require.NoError(t, err)

		var firstValue float64
		retry.Run(t, func(r *retry.R) {
			respRec := httptest.NewRecorder()
			recordPromMetrics(r, a, respRec)

			firstValue = metricLineValue(
				r,
				respRec,
				metricsPrefix+"_leaf_certs_cert_expiry",
				`datacenter="dc1"`,
				`partition="default"`,
				`namespace="default"`,
				`service="web"`,
				`kind=""`,
			)
			assertMetricValueBetween(
				r,
				metricsPrefix+"_leaf_certs_cert_expiry",
				firstValue,
				float64((leafTTL - leafCertDriftBuffer - leafWindow).Seconds()),
				float64((leafTTL - leafCertDriftBuffer).Seconds()),
			)
			require.Equal(r, 0.0, metricLineValue(
				r,
				respRec,
				metricsPrefix+"_leaf_certs_cert_renewal_failure",
				`datacenter="dc1"`,
				`partition="default"`,
				`namespace="default"`,
				`service="web"`,
				`kind=""`,
			))
		})

		time.Sleep(3 * time.Second)

		retry.Run(t, func(r *retry.R) {
			respRec := httptest.NewRecorder()
			recordPromMetrics(r, a, respRec)

			secondValue := metricLineValue(
				r,
				respRec,
				metricsPrefix+"_leaf_certs_cert_expiry",
				`datacenter="dc1"`,
				`partition="default"`,
				`namespace="default"`,
				`service="web"`,
				`kind=""`,
			)
			require.Less(r, secondValue, firstValue)
			require.GreaterOrEqual(r, firstValue-secondValue, 1.0)
		})
	})

	t.Run("leaf metrics are emitted for multiple services independently", func(t *testing.T) {
		const (
			leafTTL             = time.Hour
			signingTTL          = 4 * time.Hour
			rootTTL             = 5 * time.Hour
			leafWindow          = 5 * time.Minute
			leafCertDriftBuffer = time.Minute
		)

		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
			certificate = {
				enabled = true
			}
		}
		connect {
			enabled = true
			ca_config {
				cluster_id = "%s"
				leaf_cert_ttl = "%s"
				intermediate_cert_ttl = "%s"
				root_cert_ttl = "%s"
			}
		}
		`, metricsPrefix, connect.TestClusterID, leafTTL.String(), signingTTL.String(), rootTTL.String())

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")
		testrpc.WaitForActiveCARoot(t, a.RPC, "dc1", nil)

		client := a.Client()
		for _, svc := range []struct {
			name string
			port int
		}{
			{name: "api", port: 9090},
			{name: "payments", port: 9091},
		} {
			err := client.Agent().ServiceRegister(&api.AgentServiceRegistration{
				Name: svc.name,
				Port: svc.port,
			})
			require.NoError(t, err)

			_, _, err = client.Agent().ConnectCALeaf(svc.name, nil)
			require.NoError(t, err)
		}

		retry.Run(t, func(r *retry.R) {
			respRec := httptest.NewRecorder()
			recordPromMetrics(r, a, respRec)

			for _, serviceName := range []string{"api", "payments"} {
				value := metricLineValue(
					r,
					respRec,
					metricsPrefix+"_leaf_certs_cert_expiry",
					`datacenter="dc1"`,
					`partition="default"`,
					`namespace="default"`,
					fmt.Sprintf(`service="%s"`, serviceName),
					`kind=""`,
				)
				assertMetricValueBetween(
					r,
					metricsPrefix+"_leaf_certs_cert_expiry",
					value,
					float64((leafTTL - leafCertDriftBuffer - leafWindow).Seconds()),
					float64((leafTTL - leafCertDriftBuffer).Seconds()),
				)
				require.Equal(r, 0.0, metricLineValue(
					r,
					respRec,
					metricsPrefix+"_leaf_certs_cert_renewal_failure",
					`datacenter="dc1"`,
					`partition="default"`,
					`namespace="default"`,
					fmt.Sprintf(`service="%s"`, serviceName),
					`kind=""`,
				))
			}
		})
	})
}

func TestHTTPHandlers_AgentMetrics_WAL_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("client agent emits nothing", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		server = false
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		raft_logstore {
			backend = "wal"
		}
		bootstrap = false
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		require.NotContains(t, respRec.Body.String(), metricsPrefix+"_raft_wal")
	})

	t.Run("server with WAL enabled emits WAL metrics", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		server = true
		bootstrap = true
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		connect {
			enabled = true
		}
		raft_logstore {
			backend = "wal"
		}
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		retry.Run(t, func(r *retry.R) {
			respRec := httptest.NewRecorder()
			recordPromMetrics(r, a, respRec)

			out := respRec.Body.String()
			require.Contains(r, out, metricsPrefix+"_raft_wal_head_truncations")
			require.Contains(r, out, metricsPrefix+"_raft_wal_last_segment_age_seconds")
			require.Contains(r, out, metricsPrefix+"_raft_wal_log_appends")
			require.Contains(r, out, metricsPrefix+"_raft_wal_log_entries_read")
			require.Contains(r, out, metricsPrefix+"_raft_wal_log_entries_written")
			require.Contains(r, out, metricsPrefix+"_raft_wal_log_entry_bytes_read")
			require.Contains(r, out, metricsPrefix+"_raft_wal_log_entry_bytes_written")
			require.Contains(r, out, metricsPrefix+"_raft_wal_segment_rotations")
			require.Contains(r, out, metricsPrefix+"_raft_wal_stable_gets")
			require.Contains(r, out, metricsPrefix+"_raft_wal_stable_sets")
			require.Contains(r, out, metricsPrefix+"_raft_wal_tail_truncations")
		})

	})

	t.Run("server without WAL enabled emits no WAL metrics", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		server = true
		bootstrap = true
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		connect {
			enabled = true
		}
		raft_logstore {
			backend = "boltdb"
		}
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		require.NotContains(t, respRec.Body.String(), metricsPrefix+"_raft_wal")
	})

}

func TestHTTPHandlers_AgentMetrics_LogVerifier_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("client agent emits nothing", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		server = false
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		raft_logstore {
			verification {
				enabled = true
				interval = "1s"
			}
		}
		bootstrap = false
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		require.NotContains(t, respRec.Body.String(), metricsPrefix+"_raft_logstore_verifier")
	})

	t.Run("server with verifier enabled emits all metrics", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		server = true
		bootstrap = true
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		connect {
			enabled = true
		}
		raft_logstore {
			verification {
				enabled = true
				interval = "1s"
			}
		}
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		retry.Run(t, func(r *retry.R) {
			respRec := httptest.NewRecorder()
			recordPromMetrics(r, a, respRec)

			out := respRec.Body.String()
			require.Contains(r, out, metricsPrefix+"_raft_logstore_verifier_checkpoints_written")
			require.Contains(r, out, metricsPrefix+"_raft_logstore_verifier_dropped_reports")
			require.Contains(r, out, metricsPrefix+"_raft_logstore_verifier_ranges_verified")
			require.Contains(r, out, metricsPrefix+"_raft_logstore_verifier_read_checksum_failures")
			require.Contains(r, out, metricsPrefix+"_raft_logstore_verifier_write_checksum_failures")
		})
	})

	t.Run("server with verifier disabled emits no extra metrics", func(t *testing.T) {
		metricsPrefix := getUniqueMetricsPrefix()
		hcl := fmt.Sprintf(`
		server = true
		bootstrap = true
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "%s"
		}
		connect {
			enabled = true
		}
		raft_logstore {
			verification {
				enabled = false
			}
		}
		`, metricsPrefix)

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		require.NotContains(t, respRec.Body.String(), metricsPrefix+"_raft_logstore_verifier")
	})

}
