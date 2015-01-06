package api

import (
	"testing"
)

func TestAgent_Self(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	name := info["Config"]["NodeName"]
	if name == "" {
		t.Fatalf("bad: %v", info)
	}
}

func TestAgent_Members(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	members, err := agent.Members(false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("bad: %v", members)
	}
}

func TestAgent_Services(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

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
	if _, ok := checks["service:foo"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_SetTTLStatus(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

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

	if err := agent.WarnTTL("service:foo", "test"); err != nil {
		t.Fatalf("err: %v", err)
	}

	checks, err := agent.Checks()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	chk, ok := checks["service:foo"]
	if !ok {
		t.Fatalf("missing check: %v", checks)
	}
	if chk.Status != "warning" {
		t.Fatalf("Bad: %#v", chk)
	}
	if chk.Output != "test" {
		t.Fatalf("Bad: %#v", chk)
	}

	if err := agent.ServiceDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_Checks(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

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
	if _, ok := checks["foo"]; !ok {
		t.Fatalf("missing check: %v", checks)
	}

	if err := agent.CheckDeregister("foo"); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_Join(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Join ourself
	addr := info["Config"]["AdvertiseAddr"].(string)
	err = agent.Join(addr, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_ForceLeave(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	agent := c.Agent()

	// Eject somebody
	err := agent.ForceLeave("foo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
}
