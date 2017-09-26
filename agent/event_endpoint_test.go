package agent

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testutil/retry"
)

func TestEventFire(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig()+`
		acl_default_policy = "deny"
	`)
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
		url := fmt.Sprintf("/v1/event/fire/%s?token=%s", c.event, token)
		req, _ := http.NewRequest("PUT", url, nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.EventFire(resp, req); err != nil {
			t.Fatalf("err: %s", err)
		}

		// Check the result
		body := resp.Body.String()
		if c.allowed {
			if acl.IsErrPermissionDenied(errors.New(body)) {
				t.Fatalf("bad: %s", body)
			}
			if resp.Code != 200 {
				t.Fatalf("bad: %d", resp.Code)
			}
		} else {
			if !acl.IsErrPermissionDenied(errors.New(body)) {
				t.Fatalf("bad: %s", body)
			}
			if resp.Code != 403 {
				t.Fatalf("bad: %d", resp.Code)
			}
		}
	}
}

func TestEventList(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	t.Parallel()
	a := NewTestAgent(t.Name(), TestACLConfig())
	defer a.Shutdown()

	// Fire an event.
	p := &UserEvent{Name: "foo"}
	if err := a.UserEvent("dc1", "root", p); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("no token", func(t *testing.T) {
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
			if len(list) != 0 {
				r.Fatalf("bad: %#v", list)
			}
		})
	})

	t.Run("root token", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/event/list?token=root", nil)
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
		})
	})
}

func TestEventList_Blocking(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
			t.Fatalf("err: %v", err)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Fire some events in a non-sequential order
	expected := &UserEvent{Name: "foo"}

	for _, e := range []*UserEvent{
		&UserEvent{Name: "foo"},
		&UserEvent{Name: "bar"},
		&UserEvent{Name: "foo"},
		expected,
		&UserEvent{Name: "bar"},
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
