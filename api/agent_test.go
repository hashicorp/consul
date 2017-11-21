package api

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/serf/serf"
)

func TestAPI_AgentSelf(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	name := info["Config"]["NodeName"].(string)
	if name == "" {
		t.Fatalf("bad: %v", info)
	}
}

func TestAPI_AgentMetrics(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	timer := &retry.Timer{Timeout: 10 * time.Second, Wait: 500 * time.Millisecond}
	retry.RunWith(timer, t, func(r *retry.R) {
		metrics, err := agent.Metrics()
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		for _, g := range metrics.Gauges {
			if g.Name == "consul.runtime.alloc_bytes" {
				return
			}
		}
		r.Fatalf("missing runtime metrics")
	})
}

func TestAPI_AgentReload(t *testing.T) {
	t.Parallel()

	// Create our initial empty config file, to be overwritten later
	cfgDir := testutil.TempDir(t, "consul-config")
	defer os.RemoveAll(cfgDir)

	cfgFilePath := filepath.Join(cfgDir, "reload.json")
	configFile, err := os.Create(cfgFilePath)
	if err != nil {
		t.Fatalf("Unable to create file %v, got error:%v", cfgFilePath, err)
	}

	c, s := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Args = []string{"-config-file", configFile.Name()}
	})
	defer s.Stop()

	agent := c.Agent()

	// Update the config file with a service definition
	config := `{"service":{"name":"redis", "port":1234}}`
	err = ioutil.WriteFile(configFile.Name(), []byte(config), 0644)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if err = agent.Reload(); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	service, ok := services["redis"]
	if !ok {
		t.Fatalf("bad: %v", ok)
	}
	if service.Port != 1234 {
		t.Fatalf("bad: %v", service.Port)
	}
}

func TestAPI_AgentMembersOpts(t *testing.T) {
	t.Parallel()
	c, s1 := makeClient(t)
	_, s2 := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		c.Datacenter = "dc2"
	})
	defer s1.Stop()
	defer s2.Stop()

	agent := c.Agent()

	s2.JoinWAN(t, s1.WANAddr)

	members, err := agent.MembersOpts(MembersOpts{WAN: true})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("bad: %v", members)
	}
}

func TestAPI_AgentMembers(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	members, err := agent.Members(false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("bad: %v", members)
	}
}

func TestAPI_AgentServices(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["service:foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}

	// Checks should default to critical
	if chk.Status != HealthCritical {
		t.Fatalf("Bad: %#v", chk)
	}

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentServices_CheckPassing(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL:    "15s",
			Status: HealthPassing,
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %v", services)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["service:foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}

	if chk.Status != HealthPassing {
		t.Fatalf("Bad: %#v", chk)
	}
	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentServices_CheckBadStatus(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()
	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Check: &AgentServiceCheck{
			TTL:    "15s",
			Status: "fluffy",
		},
	}
	if err := agent.ServiceRegister(reg); err == nil {
		t.Fatalf("bad status accepted")
	}
}

func TestAPI_AgentServiceAddress(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg1 := &AgentServiceRegistration{
		Name:    "foo1",
		Port:    8000,
		Address: "192.168.0.42",
	}
	reg2 := &AgentServiceRegistration{
		Name: "foo2",
		Port: 8000,
	}
	if err := agent.ServiceRegister(reg1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := agent.ServiceRegister(reg2); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := services["foo1"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	if _, ok := services["foo2"]; !ok {
		t.Fatalf("missing service: %v", services)
	}

	if services["foo1"].Address != "192.168.0.42" {
		t.Fatalf("missing Address field in service foo1: %v", services)
	}
	if services["foo2"].Address != "" {
		t.Fatalf("missing Address field in service foo2: %v", services)
	}

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentEnableTagOverride(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg1 := &AgentServiceRegistration{
		Name:              "foo1",
		Port:              8000,
		Address:           "192.168.0.42",
		EnableTagOverride: true,
	}
	reg2 := &AgentServiceRegistration{
		Name: "foo2",
		Port: 8000,
	}
	if err := agent.ServiceRegister(reg1); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := agent.ServiceRegister(reg2); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, ok := services["foo1"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	if services["foo1"].EnableTagOverride != true {
		t.Fatalf("tag override not set on service foo1: %v", services)
	}
	if _, ok := services["foo2"]; !ok {
		t.Fatalf("missing service: %v", services)
	}
	if services["foo2"].EnableTagOverride != false {
		t.Fatalf("tag override set on service foo2: %v", services)
	}
}

func TestAPI_AgentServices_MultipleChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentServiceRegistration{
		Name: "foo",
		Tags: []string{"bar", "baz"},
		Port: 8000,
		Checks: AgentServiceChecks{
			&AgentServiceCheck{
				TTL: "15s",
			},
			&AgentServiceCheck{
				TTL: "30s",
			},
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	services, err := agent.Services()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := services["foo"]; !ok {
		t.Fatalf("missing service: %v", services)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, ok := checks["service:foo:1"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if _, ok := checks["service:foo:2"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}
}

func TestAPI_AgentSetTTLStatus(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentServiceRegistration{
		Name: "foo",
		Check: &AgentServiceCheck{
			TTL: "15s",
		},
	}
	if err := agent.ServiceRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	verify := func(status, output string) {
		checks, err := agent.Checks()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		chk, ok := checks["service:foo"]
		if !ok {
			t.Fatalf("missing check: %v", checks)
		}
		if chk.Status != status {
			t.Fatalf("Bad: %#v", chk)
		}
		if chk.Output != output {
			t.Fatalf("Bad: %#v", chk)
		}
	}

	if err := agent.WarnTTL("service:foo", "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthWarning, "foo")

	if err := agent.PassTTL("service:foo", "bar"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthPassing, "bar")

	if err := agent.FailTTL("service:foo", "baz"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthCritical, "baz")

	if err := agent.UpdateTTL("service:foo", "foo", "warn"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthWarning, "foo")

	if err := agent.UpdateTTL("service:foo", "bar", "pass"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthPassing, "bar")

	if err := agent.UpdateTTL("service:foo", "baz", "fail"); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthCritical, "baz")

	if err := agent.UpdateTTL("service:foo", "foo", HealthWarning); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthWarning, "foo")

	if err := agent.UpdateTTL("service:foo", "bar", HealthPassing); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthPassing, "bar")

	if err := agent.UpdateTTL("service:foo", "baz", HealthCritical); err != nil {
		t.Fatalf("err: %v", err)
	}
	verify(HealthCritical, "baz")

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentChecks(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentCheckRegistration{
		Name: "foo",
	}
	reg.TTL = "15s"
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if chk.Status != HealthCritical {
		t.Fatalf("check not critical: %v", chk)
	}

	if err := agent.CheckDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentScriptCheck(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		c.EnableScriptChecks = true
	})
	defer s.Stop()

	agent := c.Agent()

	t.Run("node script check", func(t *testing.T) {
		reg := &AgentCheckRegistration{
			Name: "foo",
			AgentServiceCheck: AgentServiceCheck{
				Interval: "10s",
				Args:     []string{"sh", "-c", "false"},
			},
		}
		if err := agent.CheckRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}

		checks, err := agent.Checks()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if _, ok := checks["foo"]; !ok {
			t.Fatalf("missing check: %v", checks)
		}
	})

	t.Run("service script check", func(t *testing.T) {
		reg := &AgentServiceRegistration{
			Name: "bar",
			Port: 1234,
			Checks: AgentServiceChecks{
				&AgentServiceCheck{
					Interval: "10s",
					Args:     []string{"sh", "-c", "false"},
				},
			},
		}
		if err := agent.ServiceRegister(reg); err != nil {
			t.Fatalf("err: %v", err)
		}

		services, err := agent.Services()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if _, ok := services["bar"]; !ok {
			t.Fatalf("missing service: %v", services)
		}

		checks, err := agent.Checks()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if _, ok := checks["service:bar"]; !ok {
			t.Fatalf("missing check: %v", checks)
		}
	})
}

func TestAPI_AgentCheckStartPassing(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	reg := &AgentCheckRegistration{
		Name: "foo",
		AgentServiceCheck: AgentServiceCheck{
			Status: HealthPassing,
		},
	}
	reg.TTL = "15s"
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if chk.Status != HealthPassing {
		t.Fatalf("check not passing: %v", chk)
	}

	if err := agent.CheckDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentChecks_serviceBound(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	// First register a service
	serviceReg := &AgentServiceRegistration{
		Name: "redis",
	}
	if err := agent.ServiceRegister(serviceReg); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check bound to the service
	reg := &AgentCheckRegistration{
		Name:      "redischeck",
		ServiceID: "redis",
	}
	reg.TTL = "15s"
	reg.DeregisterCriticalServiceAfter = "nope"
	err := agent.CheckRegister(reg)
	if err == nil || !strings.Contains(err.Error(), "invalid duration") {
		t.Fatalf("err: %v", err)
	}

	reg.DeregisterCriticalServiceAfter = "90m"
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	check, ok := checks["redischeck"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if check.ServiceID != "redis" {
		t.Fatalf("missing service association for check: %v", check)
	}
}

func TestAPI_AgentChecks_Docker(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		c.EnableScriptChecks = true
	})
	defer s.Stop()

	agent := c.Agent()

	// First register a service
	serviceReg := &AgentServiceRegistration{
		Name: "redis",
	}
	if err := agent.ServiceRegister(serviceReg); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check bound to the service
	reg := &AgentCheckRegistration{
		Name:      "redischeck",
		ServiceID: "redis",
		AgentServiceCheck: AgentServiceCheck{
			DockerContainerID: "f972c95ebf0e",
			Script:            "/bin/true",
			Shell:             "/bin/bash",
			Interval:          "10s",
		},
	}
	if err := agent.CheckRegister(reg); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	check, ok := checks["redischeck"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if check.ServiceID != "redis" {
		t.Fatalf("missing service association for check: %v", check)
	}
}

func TestAPI_AgentJoin(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Join ourself
	addr := info["DebugConfig"]["SerfAdvertiseAddrLAN"].(string)
	// strip off 'tcp://'
	addr = addr[len("tcp://"):]
	err = agent.Join(addr, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentLeave(t *testing.T) {
	t.Parallel()
	c1, s1 := makeClient(t)
	defer s1.Stop()

	c2, s2 := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.Server = false
		conf.Bootstrap = false
	})
	defer s2.Stop()

	if err := c2.Agent().Join(s1.LANAddr, false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We sometimes see an EOF response to this one, depending on timing.
	err := c2.Agent().Leave()
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		t.Fatalf("err: %v", err)
	}

	// Make sure the second agent's status is 'Left'
	members, err := c1.Agent().Members(false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	member := members[0]
	if member.Name == s1.Config.NodeName {
		member = members[1]
	}
	if member.Status != int(serf.StatusLeft) {
		t.Fatalf("bad: %v", *member)
	}
}

func TestAPI_AgentForceLeave(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	// Eject somebody
	err := agent.ForceLeave("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPI_AgentMonitor(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	logCh, err := agent.Monitor("info", nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the first log message and validate it
	select {
	case log := <-logCh:
		if !strings.Contains(log, "[INFO]") {
			t.Fatalf("bad: %q", log)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("failed to get a log message")
	}
}

func TestAPI_ServiceMaintenance(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	// First register a service
	serviceReg := &AgentServiceRegistration{
		Name: "redis",
	}
	if err := agent.ServiceRegister(serviceReg); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Enable maintenance mode
	if err := agent.EnableServiceMaintenance("redis", "broken"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure a critical check was added
	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	found := false
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			found = true
			if check.Status != HealthCritical || check.Notes != "broken" {
				t.Fatalf("bad: %#v", checks)
			}
		}
	}
	if !found {
		t.Fatalf("bad: %#v", checks)
	}

	// Disable maintenance mode
	if err := agent.DisableServiceMaintenance("redis"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the critical health check was removed
	checks, err = agent.Checks()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			t.Fatalf("should have removed health check")
		}
	}
}

func TestAPI_NodeMaintenance(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	agent := c.Agent()

	// Enable maintenance mode
	if err := agent.EnableNodeMaintenance("broken"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Check that a critical check was added
	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	found := false
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			found = true
			if check.Status != HealthCritical || check.Notes != "broken" {
				t.Fatalf("bad: %#v", checks)
			}
		}
	}
	if !found {
		t.Fatalf("bad: %#v", checks)
	}

	// Disable maintenance mode
	if err := agent.DisableNodeMaintenance(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was removed
	checks, err = agent.Checks()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	for _, check := range checks {
		if strings.Contains(check.CheckID, "maintenance") {
			t.Fatalf("should have removed health check")
		}
	}
}

func TestAPI_AgentUpdateToken(t *testing.T) {
	t.Parallel()
	c, s := makeACLClient(t)
	defer s.Stop()

	agent := c.Agent()

	if _, err := agent.UpdateACLToken("root", nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, err := agent.UpdateACLAgentToken("root", nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, err := agent.UpdateACLAgentMasterToken("root", nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, err := agent.UpdateACLReplicationToken("root", nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}
