package agent

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func checkForShortTesting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
}

func recordPromMetrics(t *testing.T, a *TestAgent, respRec *httptest.ResponseRecorder) {
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
	checkForShortTesting(t)
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
