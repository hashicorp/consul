package agent

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/pascaldekloe/goe/verify"
)

func TestEventFire(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		body := bytes.NewBuffer([]byte("test"))
		url := "/v1/event/fire/test?node=Node&service=foo&tag=bar"
		req, err := http.NewRequest("PUT", url, body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
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
			req, err := http.NewRequest("PUT", url, nil)
			if err != nil {
				t.Fatalf("err: %s", err)
			}
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
		e := &UserEvent{Name: "test"}
		if err := srv.agent.UserEvent("dc1", "root", e); err != nil {
			t.Fatalf("err: %v", err)
		}

		retry.Fatal(t, func() error {
			list, _, err := getEventList(srv, "/v1/event/list")
			if err != nil {
				return err
			}
			want := []*UserEvent{{ID: e.ID, Name: "test", Version: 1, LTime: 2}}
			if !verify.Func(t.Log, "", list, want) {
				return errors.New("results differ")
			}
			return nil
		})
	})
}

func TestEventList_Filter(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		events := []*UserEvent{{Name: "test"}, {Name: "foo"}}
		for _, e := range events {
			if err := srv.agent.UserEvent("dc1", "root", e); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		retry.Fatal(t, func() error {
			list, _, err := getEventList(srv, "/v1/event/list?name=foo")
			if err != nil {
				return err
			}
			want := []*UserEvent{{ID: events[1].ID, Name: "foo", Version: 1, LTime: 3}}
			if !verify.Func(t.Log, "", list, want) {
				return errors.New("results differ")
			}
			return nil
		})
	})
}

func TestEventList_ACLFilter(t *testing.T) {
	dir, srv := makeHTTPServerWithACLs(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Fire an event.
	e := &UserEvent{Name: "foo"}
	if err := srv.agent.UserEvent("dc1", "root", e); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Try no token.
	retry.Fatal(t, func() error {
		list, _, err := getEventList(srv, "/v1/event/list")
		if err != nil {
			return err
		}
		if got, want := len(list), 0; got != want {
			return fmt.Errorf("got %d events want %d", got, want)
		}
		return nil
	})

	// Try the root token.
	retry.Fatal(t, func() error {
		list, _, err := getEventList(srv, "/v1/event/list?token=root")
		if err != nil {
			return err
		}
		want := []*UserEvent{{ID: e.ID, Name: "foo", Version: 1, LTime: 2}}
		if !verify.Func(t.Log, "", list, want) {
			return errors.New("results differ")
		}
		return nil
	})
}

func TestEventList_Blocking(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		events := []*UserEvent{{Name: "first"}, {Name: "second"}}
		if err := srv.agent.UserEvent("dc1", "root", events[0]); err != nil {
			t.Fatalf("err: %v", err)
		}

		var index string
		retry.Fatal(t, func() error {
			_, hdr, err := getEventList(srv, "/v1/event/list")
			if err != nil {
				return err
			}
			index = hdr
			return nil
		})

		go func() {
			time.Sleep(25 * time.Millisecond)
			if err := srv.agent.UserEvent("dc1", "root", events[1]); err != nil {
				t.Fatalf("err: %v", err)
			}
		}()

		retry.Fatal(t, func() error {
			list, _, err := getEventList(srv, "/v1/event/list?index="+index)
			if err != nil {
				return err
			}
			want := []*UserEvent{
				{ID: events[0].ID, Name: "first", Version: 1, LTime: 2},
				{ID: events[1].ID, Name: "second", Version: 1, LTime: 3},
			}
			if !verify.Func(t.Log, "", list, want) {
				return errors.New("results differ")
			}
			return nil
		})
	})
}

func TestEventList_EventBufOrder(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		events := []*UserEvent{{Name: "foo"}, {Name: "bar"}, {Name: "foo"}, {Name: "foo"}, {Name: "bar"}}
		for _, e := range events {
			if err := srv.agent.UserEvent("dc1", "root", e); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		// Test that the event order is preserved when name
		// filtering on a list of > 1 matching event.
		retry.Fatal(t, func() error {
			list, _, err := getEventList(srv, "/v1/event/list?name=foo")
			if err != nil {
				return err
			}
			want := []*UserEvent{
				{ID: events[0].ID, Name: "foo", Version: 1, LTime: 2},
				{ID: events[2].ID, Name: "foo", Version: 1, LTime: 4},
				{ID: events[3].ID, Name: "foo", Version: 1, LTime: 5},
			}
			if !verify.Func(t.Log, "", list, want) {
				return errors.New("results differ")
			}
			return nil
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

func getEventList(srv *HTTPServer, urlstr string) ([]*UserEvent, string, error) {
	req, err := http.NewRequest("GET", urlstr, nil)
	if err != nil {
		return nil, "", err
	}
	resp := httptest.NewRecorder()
	obj, err := srv.EventList(resp, req)
	if err != nil {
		return nil, "", fmt.Errorf("EventList failed: %s", err)
	}
	list, ok := obj.([]*UserEvent)
	if !ok {
		return nil, "", fmt.Errorf("result is not []*UserEvent")
	}
	header := resp.Header().Get("X-Consul-Index")
	if header == "" || header == "0" {
		return nil, "", fmt.Errorf(`X-Consul-Index header %q must not be "" or "0"`, header)
	}
	return list, header, nil
}
