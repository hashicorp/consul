// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
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
	a.State.AddServiceWithChecks(srv1, nil, "", false)

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
		TagFilter:     "replica",
	}
	if a.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}

	// Good service name
	p = &UserEvent{
		ServiceFilter: ".*sql",
		TagFilter:     "primary",
	}
	if !a.shouldProcessUserEvent(p) {
		t.Fatalf("bad")
	}
}

func TestIngestUserEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
	a.State.AddServiceWithChecks(srv1, nil, "", false)

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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig()+`
		acl_default_policy = "deny"
	`)
	defer a.Shutdown()

	token := createToken(t, a, testEventPolicy)

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

type RPC interface {
	RPC(ctx context.Context, method string, args interface{}, reply interface{}) error
}

func createToken(t *testing.T, rpc RPC, policyRules string) string {
	t.Helper()

	reqPolicy := structs.ACLPolicySetRequest{
		Datacenter: "dc1",
		Policy: structs.ACLPolicy{
			Name:  "the-policy",
			Rules: policyRules,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	err := rpc.RPC(context.Background(), "ACL.PolicySet", &reqPolicy, &structs.ACLPolicy{})
	require.NoError(t, err)

	token, err := uuid.GenerateUUID()
	require.NoError(t, err)

	reqToken := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			SecretID: token,
			Policies: []structs.ACLTokenPolicyLink{{Name: "the-policy"}},
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	err = rpc.RPC(context.Background(), "ACL.TokenSet", &reqToken, &structs.ACLToken{})
	require.NoError(t, err)
	return token
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
