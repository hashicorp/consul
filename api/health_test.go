package api

import (
	"testing"
	"time"
)

func TestHealth_Node(t *testing.T) {
	c := makeClient(t)
	agent := c.Agent()
	health := c.Health()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	name := info["Config"]["NodeName"].(string)

	checks, meta, err := health.Node(name, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.LastIndex == 0 {
		t.Fatalf("bad: %v", meta)
	}
	if len(checks) == 0 {
		t.Fatalf("Bad: %v", checks)
	}
}

func TestHealth_Checks(t *testing.T) {
	c := makeClient(t)
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

	// Wait for the register...
	time.Sleep(20 * time.Millisecond)

	checks, meta, err := health.Checks("foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.LastIndex == 0 {
		t.Fatalf("bad: %v", meta)
	}
	if len(checks) == 0 {
		t.Fatalf("Bad: %v", checks)
	}
}

func TestHealth_Service(t *testing.T) {
	c := makeClient(t)
	health := c.Health()

	// consul service should always exist...
	checks, meta, err := health.Service("consul", "", true, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.LastIndex == 0 {
		t.Fatalf("bad: %v", meta)
	}
	if len(checks) == 0 {
		t.Fatalf("Bad: %v", checks)
	}
}

func TestHealth_State(t *testing.T) {
	c := makeClient(t)
	health := c.Health()

	checks, meta, err := health.State("any", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if meta.LastIndex == 0 {
		t.Fatalf("bad: %v", meta)
	}
	if len(checks) == 0 {
		t.Fatalf("Bad: %v", checks)
	}
}
