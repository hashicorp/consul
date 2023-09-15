// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/rpc/middleware"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
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

func assertMetricsWithLabelIsNonZero(t *testing.T, respRec *httptest.ResponseRecorder, label, labelValue string) {
	if respRec.Body.String() == "" {
		t.Fatalf("Response body is empty.")
	}

	metrics := respRec.Body.String()
	labelWithValueTarget := label + "=" + "\"" + labelValue + "\""

	for _, line := range strings.Split(metrics, "\n") {
		if len(line) < 1 || line[0] == '#' {
			continue
		}

		if strings.Contains(line, labelWithValueTarget) {
			s := strings.SplitN(line, " ", 2)
			if s[1] == "0" {
				t.Fatalf("Metric with label provided \"%s:%s\" has the value 0", label, labelValue)
			}
		}
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
		metricsPrefix := "new_rpc_metrics"
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
		metricsPrefix := "new_rpc_metrics_2"
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
		// make sure rpc calls with elapsed time below 1ms are reported as decimal
		assertMetricsWithLabelIsNonZero(t, respRec, "method", "Status.Ping")
	})
}

func TestHTTPHandlers_AgentMetrics_LeaderShipMetrics(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("check that metric isLeader is set properly on server", func(t *testing.T) {
		hcl := `
		telemetry = {
			prometheus_retention_time = "5s",
			metrics_prefix = "agent_is_leader"
		}
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		retryWithBackoff := func(expectedStr string) error {
			waiter := &retry.Waiter{
				MaxWait: 1 * time.Minute,
			}
			ctx := context.Background()
			for {
				if waiter.Failures() > 7 {
					return fmt.Errorf("reach max failure: %d", waiter.Failures())
				}
				respRec := httptest.NewRecorder()
				recordPromMetrics(t, a, respRec)

				out := respRec.Body.String()
				if strings.Contains(out, expectedStr) {
					return nil
				}
				waiter.Wait(ctx)
			}
		}
		// agent hasn't become a leader
		err := retryWithBackoff("isLeader 0")
		require.NoError(t, err, "non-leader server should have isLeader 0")

		testrpc.WaitForLeader(t, a.RPC, "dc1")

		// Verify agent's isLeader metrics is 1
		err = retryWithBackoff("isLeader 1")
		require.NoError(t, err, "leader should have isLeader 1")
	})
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

		assertMetricExistsWithValue(t, respRec, "agent_2_autopilot_healthy", "1")
		assertMetricExistsWithValue(t, respRec, "agent_2_autopilot_failure_tolerance", "0")
	})
}

func TestHTTPHandlers_AgentMetrics_TLSCertExpiry_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

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

func TestHTTPHandlers_AgentMetrics_CACertExpiry_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("non-leader emits NaN", func(t *testing.T) {
		hcl := `
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_4"
		}
		connect {
			enabled = true
		}
		bootstrap = false
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		require.Contains(t, respRec.Body.String(), "agent_4_mesh_active_root_ca_expiry NaN")
		require.Contains(t, respRec.Body.String(), "agent_4_mesh_active_signing_ca_expiry NaN")
	})

	t.Run("leader emits a value", func(t *testing.T) {
		hcl := `
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_5"
		}
		connect {
			enabled = true
		}
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		out := respRec.Body.String()
		require.Contains(t, out, "agent_5_mesh_active_root_ca_expiry 3.15")
		require.Contains(t, out, "agent_5_mesh_active_signing_ca_expiry 3.15")
	})

}

func TestHTTPHandlers_AgentMetrics_WAL_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("client agent emits nothing", func(t *testing.T) {
		hcl := `
		server = false
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_4"
		}
		raft_logstore {
			backend = "wal"
		}
		bootstrap = false
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		require.NotContains(t, respRec.Body.String(), "agent_4_raft_wal")
	})

	t.Run("server with WAL enabled emits WAL metrics", func(t *testing.T) {
		hcl := `
		server = true
		bootstrap = true
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_5"
		}
		connect {
			enabled = true
		}
		raft_logstore {
			backend = "wal"
		}
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		out := respRec.Body.String()
		require.Contains(t, out, "agent_5_raft_wal_head_truncations")
		require.Contains(t, out, "agent_5_raft_wal_last_segment_age_seconds")
		require.Contains(t, out, "agent_5_raft_wal_log_appends")
		require.Contains(t, out, "agent_5_raft_wal_log_entries_read")
		require.Contains(t, out, "agent_5_raft_wal_log_entries_written")
		require.Contains(t, out, "agent_5_raft_wal_log_entry_bytes_read")
		require.Contains(t, out, "agent_5_raft_wal_log_entry_bytes_written")
		require.Contains(t, out, "agent_5_raft_wal_segment_rotations")
		require.Contains(t, out, "agent_5_raft_wal_stable_gets")
		require.Contains(t, out, "agent_5_raft_wal_stable_sets")
		require.Contains(t, out, "agent_5_raft_wal_tail_truncations")
	})

	t.Run("server without WAL enabled emits no WAL metrics", func(t *testing.T) {
		hcl := `
		server = true
		bootstrap = true
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_6"
		}
		connect {
			enabled = true
		}
		raft_logstore {
			backend = "boltdb"
		}
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		require.NotContains(t, respRec.Body.String(), "agent_6_raft_wal")
	})

}

func TestHTTPHandlers_AgentMetrics_LogVerifier_Prometheus(t *testing.T) {
	skipIfShortTesting(t)
	// This test cannot use t.Parallel() since we modify global state, ie the global metrics instance

	t.Run("client agent emits nothing", func(t *testing.T) {
		hcl := `
		server = false
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_4"
		}
		raft_logstore {
			verification {
				enabled = true
				interval = "1s"
			}
		}
		bootstrap = false
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		require.NotContains(t, respRec.Body.String(), "agent_4_raft_logstore_verifier")
	})

	t.Run("server with verifier enabled emits all metrics", func(t *testing.T) {
		hcl := `
		server = true
		bootstrap = true
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_5"
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
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		out := respRec.Body.String()
		require.Contains(t, out, "agent_5_raft_logstore_verifier_checkpoints_written")
		require.Contains(t, out, "agent_5_raft_logstore_verifier_dropped_reports")
		require.Contains(t, out, "agent_5_raft_logstore_verifier_ranges_verified")
		require.Contains(t, out, "agent_5_raft_logstore_verifier_read_checksum_failures")
		require.Contains(t, out, "agent_5_raft_logstore_verifier_write_checksum_failures")
	})

	t.Run("server with verifier disabled emits no extra metrics", func(t *testing.T) {
		hcl := `
		server = true
		bootstrap = true
		telemetry = {
			prometheus_retention_time = "5s",
			disable_hostname = true
			metrics_prefix = "agent_6"
		}
		connect {
			enabled = true
		}
		raft_logstore {
			verification {
				enabled = false
			}
		}
		`

		a := StartTestAgent(t, TestAgent{HCL: hcl})
		defer a.Shutdown()
		testrpc.WaitForLeader(t, a.RPC, "dc1")

		respRec := httptest.NewRecorder()
		recordPromMetrics(t, a, respRec)

		require.NotContains(t, respRec.Body.String(), "agent_6_raft_logstore_verifier")
	})

}
