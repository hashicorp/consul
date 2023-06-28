// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/stretchr/testify/require"
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

func TestAPI_HealthNode_Filter(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	// this sets up the catalog entries with things we can filter on
	testNodeServiceCheckRegistrations(t, c, "dc1")

	health := c.Health()

	// filter for just the redis service checks
	checks, _, err := health.Node("foo", &QueryOptions{Filter: "ServiceName == redis"})
	require.NoError(t, err)
	require.Len(t, checks, 2)

	// filter out service checks
	checks, _, err = health.Node("foo", &QueryOptions{Filter: "ServiceID == ``"})
	require.NoError(t, err)
	require.Len(t, checks, 2)
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
	const nodename = "node123"
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeName = nodename
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

	retry.Run(t, func(r *retry.R) {
		checks := HealthChecks{
			&HealthCheck{
				Node:        nodename,
				CheckID:     "service:foo",
				Name:        "Service 'foo' check",
				Status:      "critical",
				ServiceID:   "foo",
				ServiceName: "foo",
				ServiceTags: []string{"bar"},
				Type:        "ttl",
				Partition:   defaultPartition,
				Namespace:   defaultNamespace,
			},
		}

		out, meta, err := health.Checks("foo", nil)
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		checks[0].CreateIndex = out[0].CreateIndex
		checks[0].ModifyIndex = out[0].ModifyIndex
		require.Equal(r, checks, out)
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

	s.WaitForSerfCheck(t)

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

	retry.Run(t, func(r *retry.R) {
		checks, meta, err := health.Checks("foo", &QueryOptions{NodeMeta: meta})
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		if len(checks) != 1 {
			r.Fatalf("expected 1 check, got %d", len(checks))
		}
		if checks[0].Type != "ttl" {
			r.Fatalf("expected type ttl, got %s", checks[0].Type)
		}
	})
}

func TestAPI_HealthChecks_Filter(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	// this sets up the catalog entries with things we can filter on
	testNodeServiceCheckRegistrations(t, c, "dc1")

	health := c.Health()

	checks, _, err := health.Checks("redis", &QueryOptions{Filter: "Node == foo"})
	require.NoError(t, err)
	// 1 service check for each instance
	require.Len(t, checks, 2)

	checks, _, err = health.Checks("redis", &QueryOptions{Filter: "Node == bar"})
	require.NoError(t, err)
	// 1 service check for each instance
	require.Len(t, checks, 1)

	checks, _, err = health.Checks("redis", &QueryOptions{Filter: "Node == foo and v1 in ServiceTags"})
	require.NoError(t, err)
	// 1 service check for the matching instance
	require.Len(t, checks, 1)
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

func TestAPI_HealthService_SingleTag(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeName = "node123"
	})
	defer s.Stop()
	agent := c.Agent()
	health := c.Health()
	reg := &AgentServiceRegistration{
		Name: "foo",
		ID:   "foo1",
		Tags: []string{"bar"},
		Check: &AgentServiceCheck{
			Status: HealthPassing,
			TTL:    "15s",
		},
	}
	require.NoError(t, agent.ServiceRegister(reg))
	retry.Run(t, func(r *retry.R) {
		services, meta, err := health.Service("foo", "bar", true, nil)
		require.NoError(r, err)
		require.NotEqual(r, meta.LastIndex, 0)
		require.Len(r, services, 1)
		require.Equal(r, services[0].Service.ID, "foo1")

		for _, check := range services[0].Checks {
			if check.CheckID == "service:foo1" && check.Type != "ttl" {
				r.Fatalf("expected type ttl, got %s", check.Type)
			}
		}
	})
}
func TestAPI_HealthService_MultipleTags(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeName = "node123"
	})
	defer s.Stop()

	agent := c.Agent()
	health := c.Health()

	// Make two services with a check
	reg := &AgentServiceRegistration{
		Name: "foo",
		ID:   "foo1",
		Tags: []string{"bar"},
		Check: &AgentServiceCheck{
			Status: HealthPassing,
			TTL:    "15s",
		},
	}
	require.NoError(t, agent.ServiceRegister(reg))

	reg2 := &AgentServiceRegistration{
		Name: "foo",
		ID:   "foo2",
		Tags: []string{"bar", "v2"},
		Check: &AgentServiceCheck{
			Status: HealthPassing,
			TTL:    "15s",
		},
	}
	require.NoError(t, agent.ServiceRegister(reg2))

	// Test searching with one tag (two results)
	retry.Run(t, func(r *retry.R) {
		services, meta, err := health.ServiceMultipleTags("foo", []string{"bar"}, true, nil)

		require.NoError(r, err)
		require.NotEqual(r, meta.LastIndex, 0)
		require.Len(r, services, 2)
	})

	// Test searching with two tags (one result)
	retry.Run(t, func(r *retry.R) {
		services, meta, err := health.ServiceMultipleTags("foo", []string{"bar", "v2"}, true, nil)

		require.NoError(r, err)
		require.NotEqual(r, meta.LastIndex, 0)
		require.Len(r, services, 1)
		require.Equal(r, services[0].Service.ID, "foo2")
	})
}

func TestAPI_HealthService_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	meta := map[string]string{"somekey": "somevalue"}
	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.NodeMeta = meta
	})
	defer s.Stop()

	s.WaitForSerfCheck(t)

	health := c.Health()
	retry.Run(t, func(r *retry.R) {
		// consul service should always exist...
		checks, meta, err := health.Service("consul", "", true, &QueryOptions{NodeMeta: meta})
		require.NoError(r, err)
		require.NotEqual(r, meta.LastIndex, 0)
		require.NotEqual(r, len(checks), 0)
		require.Equal(r, checks[0].Node.Datacenter, "dc1")
		require.Contains(r, checks[0].Node.TaggedAddresses, "wan")
	})
}

func TestAPI_HealthService_Filter(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	// this sets up the catalog entries with things we can filter on
	testNodeServiceCheckRegistrations(t, c, "dc1")

	health := c.Health()

	services, _, err := health.Service("redis", "", false, &QueryOptions{Filter: "Service.Meta.version == 2"})
	require.NoError(t, err)
	require.Len(t, services, 1)

	services, _, err = health.Service("web", "", false, &QueryOptions{Filter: "Node.Meta.os == linux"})
	require.NoError(t, err)
	require.Len(t, services, 2)
	require.Equal(t, "baz", services[0].Node.Node)
	require.Equal(t, "baz", services[1].Node.Node)

	services, _, err = health.Service("web", "", false, &QueryOptions{Filter: "Node.Meta.os == linux and Service.Meta.version == 1"})
	require.NoError(t, err)
	require.Len(t, services, 1)
}

func TestAPI_HealthConnect(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	health := c.Health()

	s.WaitForSerfCheck(t)

	// Make a service with a proxy
	reg := &AgentServiceRegistration{
		Name: "foo",
		Port: 8000,
	}
	err := agent.ServiceRegister(reg)
	require.NoError(t, err)

	// Register the proxy
	proxyReg := &AgentServiceRegistration{
		Name: "foo-proxy",
		Port: 8001,
		Kind: ServiceKindConnectProxy,
		Proxy: &AgentServiceConnectProxyConfig{
			DestinationServiceName: "foo",
		},
	}
	err = agent.ServiceRegister(proxyReg)
	require.NoError(t, err)

	retry.Run(t, func(r *retry.R) {
		services, meta, err := health.Connect("foo", "", true, nil)
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		// Should be exactly 1 service - the original shouldn't show up as a connect
		// endpoint, only it's proxy.
		if len(services) != 1 {
			r.Fatalf("Bad: %v", services)
		}
		if services[0].Node.Datacenter != "dc1" {
			r.Fatalf("Bad datacenter: %v", services[0].Node)
		}
		if services[0].Service.Port != proxyReg.Port {
			r.Fatalf("Bad port: %v", services[0])
		}
	})
}

func TestAPI_HealthConnect_Filter(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	// this sets up the catalog entries with things we can filter on
	testNodeServiceCheckRegistrations(t, c, "dc1")

	health := c.Health()

	services, _, err := health.Connect("web", "", false, &QueryOptions{Filter: "Node.Meta.os == linux"})
	require.NoError(t, err)
	require.Len(t, services, 2)
	require.Equal(t, "baz", services[0].Node.Node)
	require.Equal(t, "baz", services[1].Node.Node)

	services, _, err = health.Service("web", "", false, &QueryOptions{Filter: "Node.Meta.os == linux and Service.Meta.version == 1"})
	require.NoError(t, err)
	require.Len(t, services, 1)
}

func TestAPI_HealthIngress(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	health := c.Health()

	s.WaitForSerfCheck(t)

	// Make a service with a proxy
	reg := &AgentServiceRegistration{
		Name: "foo",
		Port: 8000,
	}
	err := agent.ServiceRegister(reg)
	require.NoError(t, err)

	// Register the gateway
	gatewayReg := &AgentServiceRegistration{
		Name: "foo-gateway",
		Port: 8001,
		Kind: ServiceKindIngressGateway,
	}
	err = agent.ServiceRegister(gatewayReg)
	require.NoError(t, err)

	// Associate service and gateway
	gatewayConfig := &IngressGatewayConfigEntry{
		Kind: IngressGateway,
		Name: "foo-gateway",
		Listeners: []IngressListener{
			{
				Port:     2222,
				Protocol: "tcp",
				Services: []IngressService{
					{
						Name: "foo",
					},
				},
			},
		},
	}
	_, wm, err := c.ConfigEntries().Set(gatewayConfig, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)

	retry.Run(t, func(r *retry.R) {
		services, meta, err := health.Ingress("foo", true, nil)
		require.NoError(r, err)

		require.NotZero(r, meta.LastIndex)

		// Should be exactly 1 service - the original shouldn't show up as a connect
		// endpoint, only it's proxy.
		require.Len(r, services, 1)
		require.Equal(r, services[0].Node.Datacenter, "dc1")
		require.Equal(r, services[0].Service.Service, gatewayReg.Name)
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

func TestAPI_HealthState_Filter(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	// this sets up the catalog entries with things we can filter on
	testNodeServiceCheckRegistrations(t, c, "dc1")

	health := c.Health()

	checks, _, err := health.State(HealthAny, &QueryOptions{Filter: "Node == baz"})
	require.NoError(t, err)
	require.Len(t, checks, 6)

	checks, _, err = health.State(HealthAny, &QueryOptions{Filter: "Status == warning or Status == critical"})
	require.NoError(t, err)
	require.Len(t, checks, 2)

	checks, _, err = health.State(HealthCritical, &QueryOptions{Filter: "Node == baz"})
	require.NoError(t, err)
	require.Len(t, checks, 1)

	checks, _, err = health.State(HealthWarning, &QueryOptions{Filter: "Node == baz"})
	require.NoError(t, err)
	require.Len(t, checks, 1)
}
