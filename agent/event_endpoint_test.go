// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestEventFire(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	body := bytes.NewBuffer([]byte("test"))
	url := "/v1/event/fire/test?node=Node&service=foo&tag=bar"
	req, _ := http.NewRequest("PUT", url, body)
	resp := httptest.NewRecorder()
	obj, err := a.srv.EventFire(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	event, ok := obj.(*UserEvent)
	if !ok {
		t.Fatalf("bad: %#v", obj)
	}

	if event.ID == "" {
		t.Fatalf("bad: %#v", event)
	}
	if event.Name != "test" {
		t.Fatalf("bad: %#v", event)
	}
	if string(event.Payload) != "test" {
		t.Fatalf("bad: %#v", event)
	}
	if event.NodeFilter != "Node" {
		t.Fatalf("bad: %#v", event)
	}
	if event.ServiceFilter != "foo" {
		t.Fatalf("bad: %#v", event)
	}
	if event.TagFilter != "bar" {
		t.Fatalf("bad: %#v", event)
	}
}

func TestEventFire_token(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig()+`
		acl_default_policy = "deny"
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	token := createToken(t, a, testEventPolicy)

	type tcase struct {
		event   string
		allowed bool
	}
	tcases := []tcase{
		{"foo", false},
		{"bar", false},
		{"baz", true},
	}
	for _, c := range tcases {
		// Try to fire the event over the HTTP interface
		url := fmt.Sprintf("/v1/event/fire/%s", c.event)
		req, _ := http.NewRequest("PUT", url, nil)
		req.Header.Add("X-Consul-Token", token)
		resp := httptest.NewRecorder()
		_, err := a.srv.EventFire(resp, req)

		// Check the result
		if c.allowed {
			body := resp.Body.String()
			if acl.IsErrPermissionDenied(errors.New(body)) {
				t.Fatalf("bad: %s", body)
			}
			if resp.Code != 200 {
				t.Fatalf("bad: %d", resp.Code)
			}
		} else {
			if !acl.IsErrPermissionDenied(err) {
				t.Fatalf("bad: %s", err.Error())
			}
			if err, ok := err.(HTTPError); ok {
				if err.StatusCode != 403 {
					t.Fatalf("Expected 403 but got %d", err.StatusCode)
				}
			} else {
				t.Fatalf("Expected HTTP Error %v", err)
			}
		}
	}
}

func TestEventList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	p := &UserEvent{Name: "test"}
	if err := a.UserEvent("dc1", "root", p); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/event/list", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.EventList(resp, req)
		if err != nil {
			r.Fatal(err)
		}

		list, ok := obj.([]*UserEvent)
		if !ok {
			r.Fatalf("bad: %#v", obj)
		}
		if len(list) != 1 || list[0].Name != "test" {
			r.Fatalf("bad: %#v", list)
		}
		header := resp.Header().Get("X-Consul-Index")
		if header == "" || header == "0" {
			r.Fatalf("bad: %#v", header)
		}
	})
}

func TestEventList_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	p := &UserEvent{Name: "test"}
	if err := a.UserEvent("dc1", "root", p); err != nil {
		t.Fatalf("err: %v", err)
	}

	p = &UserEvent{Name: "foo"}
	if err := a.UserEvent("dc1", "root", p); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/event/list?name=foo", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.EventList(resp, req)
		if err != nil {
			r.Fatal(err)
		}

		list, ok := obj.([]*UserEvent)
		if !ok {
			r.Fatalf("bad: %#v", obj)
		}
		if len(list) != 1 || list[0].Name != "foo" {
			r.Fatalf("bad: %#v", list)
		}
		header := resp.Header().Get("X-Consul-Index")
		if header == "" || header == "0" {
			r.Fatalf("bad: %#v", header)
		}
	})
}

func TestEventList_ACLFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, TestACLConfig())
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Fire some events.
	events := []*UserEvent{
		{Name: "foo"},
		{Name: "bar"},
	}
	for _, e := range events {
		err := a.UserEvent("dc1", "root", e)
		require.NoError(t, err)
	}

	t.Run("no token", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req := httptest.NewRequest("GET", "/v1/event/list", nil)
			resp := httptest.NewRecorder()

			obj, err := a.srv.EventList(resp, req)
			require.NoError(r, err)

			list, ok := obj.([]*UserEvent)
			require.True(r, ok)
			require.Empty(r, list)
			require.Empty(r, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
		})
	})

	t.Run("token with access to one event type", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			token := testCreateToken(t, a, `
				event "foo" {
					policy = "read"
				}
			`)

			req := httptest.NewRequest("GET", "/v1/event/list", nil)
			req.Header.Add("X-Consul-Token", token)
			resp := httptest.NewRecorder()

			obj, err := a.srv.EventList(resp, req)
			require.NoError(r, err)

			list, ok := obj.([]*UserEvent)
			require.True(r, ok)
			require.Len(r, list, 1)
			require.Equal(r, "foo", list[0].Name)
			require.NotEmpty(r, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
		})
	})

	t.Run("root token", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req := httptest.NewRequest("GET", "/v1/event/list", nil)
			req.Header.Add("X-Consul-Token", "root")
			resp := httptest.NewRecorder()

			obj, err := a.srv.EventList(resp, req)
			require.NoError(r, err)

			list, ok := obj.([]*UserEvent)
			require.True(r, ok)
			require.Len(r, list, 2)

			var names []string
			for _, e := range list {
				names = append(names, e.Name)
			}
			require.ElementsMatch(r, []string{"foo", "bar"}, names)

			require.Empty(r, resp.Header().Get("X-Consul-Results-Filtered-By-ACLs"))
		})
	})
}

func TestEventList_Blocking(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	p := &UserEvent{Name: "test"}
	if err := a.UserEvent("dc1", "root", p); err != nil {
		t.Fatalf("err: %v", err)
	}

	var index string
	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/event/list", nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.EventList(resp, req); err != nil {
			r.Fatal(err)
		}
		header := resp.Header().Get("X-Consul-Index")
		if header == "" || header == "0" {
			r.Fatalf("bad: %#v", header)
		}
		index = header
	})

	go func() {
		time.Sleep(50 * time.Millisecond)
		p := &UserEvent{Name: "second"}
		if err := a.UserEvent("dc1", "root", p); err != nil {
			t.Errorf("err: %v", err)
		}
	}()

	retry.Run(t, func(r *retry.R) {
		url := "/v1/event/list?index=" + index
		req, _ := http.NewRequest("GET", url, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.EventList(resp, req)
		if err != nil {
			r.Fatal(err)
		}

		list, ok := obj.([]*UserEvent)
		if !ok {
			r.Fatalf("bad: %#v", obj)
		}
		if len(list) != 2 || list[1].Name != "second" {
			r.Fatalf("bad: %#v", list)
		}
	})
}

func TestEventList_EventBufOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Fire some events in a non-sequential order
	expected := &UserEvent{Name: "foo"}

	for _, e := range []*UserEvent{
		{Name: "foo"},
		{Name: "bar"},
		{Name: "foo"},
		expected,
		{Name: "bar"},
	} {
		if err := a.UserEvent("dc1", "root", e); err != nil {
			t.Fatalf("err: %v", err)
		}
	}
	// Test that the event order is preserved when name
	// filtering on a list of > 1 matching event.
	retry.Run(t, func(r *retry.R) {
		url := "/v1/event/list?name=foo"
		req, _ := http.NewRequest("GET", url, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.EventList(resp, req)
		if err != nil {
			r.Fatal(err)
		}
		list, ok := obj.([]*UserEvent)
		if !ok {
			r.Fatalf("bad: %#v", obj)
		}
		if len(list) != 3 || list[2].ID != expected.ID {
			r.Fatalf("bad: %#v", list)
		}
	})
}

func TestUUIDToUint64(t *testing.T) {
	t.Parallel()
	inp := "cb9a81ad-fff6-52ac-92a7-5f70687805ec"

	// Output value was computed using python
	if uuidToUint64(inp) != 6430540886266763072 {
		t.Fatalf("bad")
	}
}
