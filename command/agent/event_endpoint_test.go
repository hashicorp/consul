package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil/retry"
)

func TestEventFire(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		body := bytes.NewBuffer([]byte("test"))
		url := "/v1/event/fire/test?node=Node&service=foo&tag=bar"
		req, _ := http.NewRequest("PUT", url, body)
		resp := httptest.NewRecorder()
		obj, err := srv.EventFire(resp, req)
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
	})
}

func TestEventFire_token(t *testing.T) {
	httpTestWithConfig(t, func(srv *HTTPServer) {
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
		if err := srv.agent.RPC("ACL.Apply", &args, &token); err != nil {
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
			if _, err := srv.EventFire(resp, req); err != nil {
				t.Fatalf("err: %s", err)
			}

			// Check the result
			body := resp.Body.String()
			if c.allowed {
				if strings.Contains(body, permissionDenied) {
					t.Fatalf("bad: %s", body)
				}
				if resp.Code != 200 {
					t.Fatalf("bad: %d", resp.Code)
				}
			} else {
				if !strings.Contains(body, permissionDenied) {
					t.Fatalf("bad: %s", body)
				}
				if resp.Code != 403 {
					t.Fatalf("bad: %d", resp.Code)
				}
			}
		}
	}, func(c *Config) {
		c.ACLDefaultPolicy = "deny"
	})
}

func TestEventList(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		p := &UserEvent{Name: "test"}
		if err := srv.agent.UserEvent("dc1", "root", p); err != nil {
			t.Fatalf("err: %v", err)
		}

		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/event/list", nil)
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
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
	})
}

func TestEventList_Filter(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		p := &UserEvent{Name: "test"}
		if err := srv.agent.UserEvent("dc1", "root", p); err != nil {
			t.Fatalf("err: %v", err)
		}

		p = &UserEvent{Name: "foo"}
		if err := srv.agent.UserEvent("dc1", "root", p); err != nil {
			t.Fatalf("err: %v", err)
		}

		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/event/list?name=foo", nil)
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
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
	})
}

func TestEventList_ACLFilter(t *testing.T) {
	dir, srv := makeHTTPServerWithACLs(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Fire an event.
	p := &UserEvent{Name: "foo"}
	if err := srv.agent.UserEvent("dc1", "root", p); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try no token.
	{
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/event/list", nil)
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
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
	}

	// Try the root token.
	{
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/event/list?token=root", nil)
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
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
	}
}

func TestEventList_Blocking(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		p := &UserEvent{Name: "test"}
		if err := srv.agent.UserEvent("dc1", "root", p); err != nil {
			t.Fatalf("err: %v", err)
		}

		var index string
		retry.Run(t, func(r *retry.R) {
			req, _ := http.NewRequest("GET", "/v1/event/list", nil)
			resp := httptest.NewRecorder()
			if _, err := srv.EventList(resp, req); err != nil {
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
			if err := srv.agent.UserEvent("dc1", "root", p); err != nil {
				t.Fatalf("err: %v", err)
			}
		}()

		retry.Run(t, func(r *retry.R) {
			url := "/v1/event/list?index=" + index
			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
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
	})
}

func TestEventList_EventBufOrder(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		// Fire some events in a non-sequential order
		expected := &UserEvent{Name: "foo"}

		for _, e := range []*UserEvent{
			&UserEvent{Name: "foo"},
			&UserEvent{Name: "bar"},
			&UserEvent{Name: "foo"},
			expected,
			&UserEvent{Name: "bar"},
		} {
			if err := srv.agent.UserEvent("dc1", "root", e); err != nil {
				t.Fatalf("err: %v", err)
			}
		}
		// Test that the event order is preserved when name
		// filtering on a list of > 1 matching event.
		retry.Run(t, func(r *retry.R) {
			url := "/v1/event/list?name=foo"
			req, _ := http.NewRequest("GET", url, nil)
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
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
	})
}

func TestUUIDToUint64(t *testing.T) {
	inp := "cb9a81ad-fff6-52ac-92a7-5f70687805ec"

	// Output value was computed using python
	if uuidToUint64(inp) != 6430540886266763072 {
		t.Fatalf("bad")
	}
}
