package agent

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/testutil/retry"
)

func TestValidateUserEventParams(t *testing.T) {
	t.Parallel()
	p := &UserEvent{}
	err := validateUserEventParams(p)
	if err == nil || err.Error() != "User event missing name" {
		t.Fatalf("err: %v", err)
	}
	p.Name = "foo"

	p.NodeFilter = "("
	err = validateUserEventParams(p)
	if err == nil || !strings.Contains(err.Error(), "Invalid node filter") {
		t.Fatalf("err: %v", err)
	}

	p.NodeFilter = ""
	p.ServiceFilter = "("
	err = validateUserEventParams(p)
	if err == nil || !strings.Contains(err.Error(), "Invalid service filter") {
		t.Fatalf("err: %v", err)
	}

	p.ServiceFilter = "foo"
	p.TagFilter = "("
	err = validateUserEventParams(p)
	if err == nil || !strings.Contains(err.Error(), "Invalid tag filter") {
		t.Fatalf("err: %v", err)
	}

	p.ServiceFilter = ""
	p.TagFilter = "foo"
	err = validateUserEventParams(p)
	if err == nil || !strings.Contains(err.Error(), "tag filter without service") {
		t.Fatalf("err: %v", err)
	}
}

func TestShouldProcessUserEvent(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"test", "foo", "bar", "master"},
		Port:    5000,
	}
	a.state.AddService(srv1, "")

	p := &UserEvent{}
	if !a.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Bad node name
	p = &UserEvent{
		NodeFilter: "foobar",
	}
	if a.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Good node name
	p = &UserEvent{
		NodeFilter: "^Node",
	}
	if !a.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Bad service name
	p = &UserEvent{
		ServiceFilter: "foobar",
	}
	if a.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Good service name
	p = &UserEvent{
		ServiceFilter: ".*sql",
	}
	if !a.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Bad tag name
	p = &UserEvent{
		ServiceFilter: ".*sql",
		TagFilter:     "slave",
	}
	if a.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Good service name
	p = &UserEvent{
		ServiceFilter: ".*sql",
		TagFilter:     "master",
	}
	if !a.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}
}

func TestIngestUserEvent(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	for i := 0; i < 512; i++ {
		msg := &UserEvent{LTime: uint64(i), Name: "test"}
		a.ingestUserEvent(msg)
		if a.LastUserEvent() != msg {
			t.Fatalf("bad: %#v", msg)
		}
		events := a.UserEvents()

		expectLen := 256
		if i < 256 {
			expectLen = i + 1
		}
		if len(events) != expectLen {
			t.Fatalf("bad: %d %d %d", i, expectLen, len(events))
		}

		counter := i
		for j := len(events) - 1; j >= 0; j-- {
			if events[j].LTime != uint64(counter) {
				t.Fatalf("bad: %#v", events)
			}
			counter--
		}
	}
}

func TestFireReceiveEvent(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"test", "foo", "bar", "master"},
		Port:    5000,
	}
	a.state.AddService(srv1, "")

	p1 := &UserEvent{Name: "deploy", ServiceFilter: "web"}
	err := a.UserEvent("dc1", "root", p1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	p2 := &UserEvent{Name: "deploy"}
	err = a.UserEvent("dc1", "root", p2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	retry.Run(t, func(r *retry.R) {
		if got, want := len(a.UserEvents()), 1; got != want {
			r.Fatalf("got %d events want %d", got, want)
		}
	})

	last := a.LastUserEvent()
	if last.ID != p2.ID {
		t.Fatalf("bad: %#v", last)
	}
}

func TestUserEventToken(t *testing.T) {
	t.Parallel()
	cfg := TestACLConfig()
	cfg.ACLDefaultPolicy = "deny" // Set the default policies to deny
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

	// Create an ACL token
	args := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTypeClient,
			Rules: testEventPolicy,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var token string
	if err := a.RPC("ACL.Apply", &args, &token); err != nil {
		t.Fatalf("err: %v", err)
	}

	type tcase struct {
		name   string
		expect bool
	}
	cases := []tcase{
		{"foo", false},
		{"bar", false},
		{"baz", true},
		{"zip", false},
	}
	for _, c := range cases {
		event := &UserEvent{Name: c.name}
		err := a.UserEvent("dc1", token, event)
		allowed := false
		if err == nil || err.Error() != permissionDenied {
			allowed = true
		}
		if allowed != c.expect {
			t.Fatalf("bad: %#v result: %v", c, allowed)
		}
	}
}

const testEventPolicy = `
event "foo" {
	policy = "deny"
}
event "bar" {
	policy = "read"
}
event "baz" {
	policy = "write"
}
`
