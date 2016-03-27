package agent

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
)

func setupGloablInmemSink() *metrics.InmemSink {
	inm := metrics.NewInmemSink(10*time.Second, time.Minute)
	metricsConf := metrics.DefaultConfig("")
	metricsConf.EnableHostname = false
	metricsConf.EnableRuntimeMetrics = false
	metrics.NewGlobal(metricsConf, inm)

	return inm
}

func TestInstrumentation(t *testing.T) {
	inm := setupGloablInmemSink()
	AgentIntrumentationInterval = 1 * time.Second
	defer func() { AgentIntrumentationInterval = DefaultAgentIntrumentationInterval }()

	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testServices := map[string]map[string]string{
		"a_multi_instance_service": map[string]string{
			"a_passing_instance":       "exit 0",
			"another_passing_instance": "exit 0",
			"warning_instance":         "exit 1",
			"failing_instance":         "exit 2",
		},
		"a_service_without_checks": map[string]string{
			"sole_instance": "",
		},
	}

	for serviceName, instances := range testServices {
		for serviceID, script := range instances {
			checks := CheckTypes{}

			if script != "" {
				checks = append(checks, &CheckType{Script: script, Interval: 2})
			}

			err := agent.AddService(
				&structs.NodeService{
					ID:      serviceID,
					Service: serviceName,
				},
				checks,
				false,
				"",
			)

			if err != nil {
				t.Fatal(err)
			}

		}
	}

	verifyGauges(
		t,
		inm,
		map[string]float32{
			"services.a_multi_instance_service.instances_by_state.passing":  2.0,
			"services.a_multi_instance_service.instances_by_state.warning":  1.0,
			"services.a_multi_instance_service.instances_by_state.critical": 1.0,
			"services.a_service_without_checks.instances_by_state.passing":  1.0,
			"services.a_service_without_checks.instances_by_state.warning":  0.0,
			"services.a_service_without_checks.instances_by_state.critical": 0.0,
		},
	)

	agent.EnableNodeMaintenance("", "")

	verifyGauges(
		t,
		inm,
		map[string]float32{
			"services.a_multi_instance_service.instances_by_state.passing":  0.0,
			"services.a_multi_instance_service.instances_by_state.warning":  0.0,
			"services.a_multi_instance_service.instances_by_state.critical": 4.0,
			"services.a_service_without_checks.instances_by_state.passing":  0.0,
			"services.a_service_without_checks.instances_by_state.warning":  0.0,
			"services.a_service_without_checks.instances_by_state.critical": 1.0,
		},
	)

}

func verifyGauges(t *testing.T, inm *metrics.InmemSink, expectedMetrics map[string]float32) {
	testutil.WaitForResult(func() (bool, error) {
		stats := inm.Data()
		latestInterval := stats[len(stats)-1]

		for metric, expectedValue := range expectedMetrics {
			val, ok := latestInterval.Gauges[metric]

			if !ok {
				return false, fmt.Errorf("expected metric %s to be set", metric)
			}

			if val != expectedValue {
				return false, fmt.Errorf("expected %s to be %f got %f", metric, expectedValue, val)
			}

		}

		return true, nil

	}, func(e error) {
		t.Fatal(e)
	})
}
