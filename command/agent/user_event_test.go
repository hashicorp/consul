package agent

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
)

func TestValidateUserEventParams(t *testing.T) {
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
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"test", "foo", "bar", "master"},
		Port:    5000,
	}
	agent.state.AddService(srv1)

	p := &UserEvent{}
	if !agent.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Bad node name
	p = &UserEvent{
		NodeFilter: "foobar",
	}
	if agent.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Good node name
	p = &UserEvent{
		NodeFilter: "^Node",
	}
	if !agent.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Bad service name
	p = &UserEvent{
		ServiceFilter: "foobar",
	}
	if agent.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Good service name
	p = &UserEvent{
		ServiceFilter: ".*sql",
	}
	if !agent.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Bad tag name
	p = &UserEvent{
		ServiceFilter: ".*sql",
		TagFilter:     "slave",
	}
	if agent.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Good service name
	p = &UserEvent{
		ServiceFilter: ".*sql",
		TagFilter:     "master",
	}
	if !agent.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}
}

func TestIngestUserEvent(t *testing.T) {
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	for i := 0; i < 512; i++ {
		msg := &UserEvent{LTime: uint64(i), Name: "test"}
		agent.ingestUserEvent(msg)
		if agent.LastUserEvent() != msg {
			t.Fatalf("bad: %#v", msg)
		}
		events := agent.UserEvents()

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
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"test", "foo", "bar", "master"},
		Port:    5000,
	}
	agent.state.AddService(srv1)

	p1 := &UserEvent{Name: "deploy", ServiceFilter: "web"}
	err := agent.UserEvent("", p1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	p2 := &UserEvent{Name: "deploy"}
	err = agent.UserEvent("", p2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testutil.WaitForResult(
		func() (bool, error) {
			return len(agent.UserEvents()) == 1, nil
		},
		func(err error) {
			t.Fatalf("bad len")
		})

	last := agent.LastUserEvent()
	if last.ID != p2.ID {
		t.Fatalf("bad: %#v", last)
	}
}
