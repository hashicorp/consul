package agent

import (
	"strings"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
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

func TestUserEventHandler_ShouldProcessUserEvent(t *testing.T) {
	type testCase struct {
		name     string
		event    *UserEvent
		expected bool
	}

	cfg := UserEventHandlerConfig{
		NodeName: "the-node",
		State: &fakeServiceLister{
			serviceID:   "the-service-id",
			serviceTags: []string{"tag1", "tag2"},
		},
	}
	u := newUserEventHandler(cfg, hclog.New(nil))

	fn := func(t *testing.T, tc testCase) {
		require.Equal(t, tc.expected, u.shouldProcessUserEvent(tc.event))
	}

	var testCases = []testCase{
		{
			name:     "empty event",
			event:    &UserEvent{},
			expected: true,
		},
		{
			name:  "node filter does not match node",
			event: &UserEvent{NodeFilter: "foobar"},
		},
		{
			name:     "node filter matches node name",
			event:    &UserEvent{NodeFilter: "the-node"},
			expected: true,
		},
		{
			name:  "service filter does not match any services",
			event: &UserEvent{ServiceFilter: "foobar"},
		},
		{
			name:     "service filter matches a service",
			event:    &UserEvent{ServiceFilter: ".*-service-id"},
			expected: true,
		},
		{
			name: "tag filter does not match",
			event: &UserEvent{
				ServiceFilter: ".*-service-id",
				TagFilter:     "foobar",
			},
		},
		{
			name: "tag filter matches a tag",
			event: &UserEvent{
				ServiceFilter: ".*-service-id",
				TagFilter:     "tag2",
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

type fakeServiceLister struct {
	serviceID   string
	serviceTags []string
}

func (f *fakeServiceLister) Services(_ *structs.EnterpriseMeta) map[structs.ServiceID]*structs.NodeService {
	return map[structs.ServiceID]*structs.NodeService{
		{ID: f.serviceID}: {
			ID:   f.serviceID,
			Tags: f.serviceTags,
		},
	}
}

func TestUserEventHandler_IngestUserEvent(t *testing.T) {
	cfg := UserEventHandlerConfig{
		Notifier: new(NotifyGroup),
	}
	u := newUserEventHandler(cfg, hclog.New(nil))

	for i := 0; i < 512; i++ {
		msg := &UserEvent{LTime: uint64(i), Name: "test"}
		u.ingestUserEvent(msg)
		if u.lastUserEvent() != msg {
			t.Fatalf("bad: %#v", msg)
		}
		events := u.UserEvents()

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

// TODO: move this with Agent.UserEvent
func TestAgent_UserEvent_FireReceiveEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"test", "foo", "bar", "primary"},
		Port:    5000,
	}
	a.State.AddService(srv1, "")

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
		if got, want := len(a.userEventHandler.UserEvents()), 1; got != want {
			r.Fatalf("got %d events want %d", got, want)
		}
	})

	last := a.userEventHandler.lastUserEvent()
	if last.ID != p2.ID {
		t.Fatalf("bad: %#v", last)
	}
}

// TODO: move this with Agent.UserEvent
func TestAgent_UserEvent_Token(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()
	a := NewTestAgent(t, TestACLConfig()+`
		acl_default_policy = "deny"
	`)
	defer a.Shutdown()

	// Create an ACL token
	args := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTokenTypeClient,
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
		allowed := !acl.IsErrPermissionDenied(err)
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
