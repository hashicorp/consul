package api

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/pascaldekloe/goe/verify"
)

func TestAPI_HealthNode(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	health := c.Health()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	name := info["Config"]["NodeName"].(string)
	retry.Run(t, func(r *retry.R) {
		checks, meta, err := health.Node(name, nil)
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		if len(checks) == 0 {
			r.Fatalf("bad: %v", checks)
		}
	})
}

func TestAPI_HealthChecks_AggregatedStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		checks HealthChecks
		exp    string
	}{
		{
			"empty",
			nil,
			HealthPassing,
		},
		{
			"passing",
			HealthChecks{
				&HealthCheck{
					Status: HealthPassing,
				},
			},
			HealthPassing,
		},
		{
			"warning",
			HealthChecks{
				&HealthCheck{
					Status: HealthWarning,
				},
			},
			HealthWarning,
		},
		{
			"critical",
			HealthChecks{
				&HealthCheck{
					Status: HealthCritical,
				},
			},
			HealthCritical,
		},
		{
			"node_maintenance",
			HealthChecks{
				&HealthCheck{
					CheckID: NodeMaint,
				},
			},
			HealthMaint,
		},
		{
			"service_maintenance",
			HealthChecks{
				&HealthCheck{
					CheckID: ServiceMaintPrefix + "service",
				},
			},
			HealthMaint,
		},
		{
			"unknown",
			HealthChecks{
				&HealthCheck{
					Status: "nope-nope-noper",
				},
			},
			"",
		},
		{
			"maintenance_over_critical",
			HealthChecks{
				&HealthCheck{
					CheckID: NodeMaint,
				},
				&HealthCheck{
					Status: HealthCritical,
				},
			},
			HealthMaint,
		},
		{
			"critical_over_warning",
			HealthChecks{
				&HealthCheck{
					Status: HealthCritical,
				},
				&HealthCheck{
					Status: HealthWarning,
				},
			},
			HealthCritical,
		},
		{
			"warning_over_passing",
			HealthChecks{
				&HealthCheck{
					Status: HealthWarning,
				},
				&HealthCheck{
					Status: HealthPassing,
				},
			},
			HealthWarning,
		},
		{
			"lots",
			HealthChecks{
				&HealthCheck{
					Status: HealthPassing,
				},
				&HealthCheck{
					Status: HealthPassing,
				},
				&HealthCheck{
					Status: HealthPassing,
				},
				&HealthCheck{
					Status: HealthWarning,
				},
			},
			HealthWarning,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			act := tc.checks.AggregatedStatus()
			if tc.exp != act {
				t.Errorf("\nexp: %#v\nact: %#v", tc.exp, act)
			}
		})
	}
}

func TestAPI_HealthChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeName = "node123"
	})
	defer s.Stop()

	agent := c.Agent()
	health := c.Health()

	// Make a service with a check
	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar"},
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}
	defer agent.ServiceDeregister("foo")

	retry.Run(t, func(r *retry.R) {
		checks := HealthChecks{
			&HealthCheck{
				Node:        "node123",
				CheckID:     "service:foo",
				Name:        "Service 'foo' check",
				Status:      "critical",
				ServiceID:   "foo",
				ServiceName: "foo",
				ServiceTags: []string{"bar"},
			},
		}

		out, meta, err := health.Checks("foo", nil)
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		if got, want := out, checks; !verify.Values(t, "checks", got, want) {
			r.Fatal("health.Checks failed")
		}
	})
}

func TestAPI_HealthChecks_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	agent := c.Agent()
	health := c.Health()

	// Make a service with a check
	reg := &AgentServiceRegistration{
		Name: "foo",
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}
	defer agent.ServiceDeregister("foo")

	retry.Run(t, func(r *retry.R) {
		checks, meta, err := health.Checks("foo", &QueryOptions{NodeMeta: meta})
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		if len(checks) == 0 {
			r.Fatalf("Bad: %v", checks)
		}
	})
}

func TestAPI_HealthService(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	health := c.Health()
	retry.Run(t, func(r *retry.R) {
		// consul service should always exist...
		checks, meta, err := health.Service("consul", "", true, nil)
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		if len(checks) == 0 {
			r.Fatalf("Bad: %v", checks)
		}
		if _, ok := checks[0].Node.TaggedAddresses["wan"]; !ok {
			r.Fatalf("Bad: %v", checks[0].Node)
		}
		if checks[0].Node.Datacenter != "dc1" {
			r.Fatalf("Bad datacenter: %v", checks[0].Node)
		}
	})
}

func TestAPI_HealthService_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	health := c.Health()
	retry.Run(t, func(r *retry.R) {
		// consul service should always exist...
		checks, meta, err := health.Service("consul", "", true, &QueryOptions{NodeMeta: meta})
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		if len(checks) == 0 {
			r.Fatalf("Bad: %v", checks)
		}
		if _, ok := checks[0].Node.TaggedAddresses["wan"]; !ok {
			r.Fatalf("Bad: %v", checks[0].Node)
		}
		if checks[0].Node.Datacenter != "dc1" {
			r.Fatalf("Bad datacenter: %v", checks[0].Node)
		}
	})
}

func TestAPI_HealthState(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	health := c.Health()
	retry.Run(t, func(r *retry.R) {
		checks, meta, err := health.State("any", nil)
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		if len(checks) == 0 {
			r.Fatalf("Bad: %v", checks)
		}
	})
}

func TestAPI_HealthState_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	health := c.Health()
	retry.Run(t, func(r *retry.R) {
		checks, meta, err := health.State("any", &QueryOptions{NodeMeta: meta})
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		if len(checks) == 0 {
			r.Fatalf("Bad: %v", checks)
		}
	})
}
