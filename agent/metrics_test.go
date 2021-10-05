package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMetrics_ConsulAutopilotHealthy_Prometheus adds testing around
// the published autopilot metrics on https://www.consul.io/docs/agent/telemetry#autopilot
func TestMetrics_ConsulAutopilotHealthy_Prometheus(t *testing.T) {
	checkForShortTesting(t)
	t.Parallel()

	// configure agent to emit Prometheus metrics
	hcl :=`
	telemetry = {
		prometheus_retention_time = "5s",
		disable_hostname = true
	}
	`

	a := StartTestAgent(t, TestAgent{HCL: hcl, NoWaitForStartup: true})
	defer a.Shutdown()


	req, err := http.NewRequest("GET", "/v1/agent/metrics?format=prometheus", nil)
	if err != nil {
		t.Fatalf("Failed to generate new http request. err: %v", err)
	}

	respRec := httptest.NewRecorder()
	_, err = a.srv.AgentMetrics(respRec, req)
	if err != nil {
		t.Fatalf("Failed to serve agent metrics. err: %v", err)
	}

	t.Run("Check consul_autopilot_healthy metric value on startup", func(t *testing.T) {
		target := "consul_autopilot_healthy NaN"
		keyValue := strings.Split(target, " ")
		if !strings.Contains(respRec.Body.String(), target) {
			t.Fatalf("Could not find the metric \"%s\" with value \"%s\" in the /v1/agent/metrics response", keyValue[0], keyValue[1])
		}
	})
}

func checkForShortTesting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
}