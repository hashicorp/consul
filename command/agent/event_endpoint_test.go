package agent

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil"
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

func TestEventList(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		p := &UserEvent{Name: "test"}
		if err := srv.agent.UserEvent("", p); err != nil {
			t.Fatalf("err: %v", err)
		}

		testutil.WaitForResult(func() (bool, error) {
			req, err := http.NewRequest("GET", "/v1/event/list", nil)
			if err != nil {
				return false, err
			}
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
			if err != nil {
				return false, err
			}

			list, ok := obj.([]*UserEvent)
			if !ok {
				return false, fmt.Errorf("bad: %#v", obj)
			}
			if len(list) != 1 || list[0].Name != "test" {
				return false, fmt.Errorf("bad: %#v", list)
			}
			header := resp.Header().Get("X-Consul-Index")
			if header == "" || header == "0" {
				return false, fmt.Errorf("bad: %#v", header)
			}
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	})
}

func TestEventList_Filter(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		p := &UserEvent{Name: "test"}
		if err := srv.agent.UserEvent("", p); err != nil {
			t.Fatalf("err: %v", err)
		}

		p = &UserEvent{Name: "foo"}
		if err := srv.agent.UserEvent("", p); err != nil {
			t.Fatalf("err: %v", err)
		}

		testutil.WaitForResult(func() (bool, error) {
			req, err := http.NewRequest("GET", "/v1/event/list?name=foo", nil)
			if err != nil {
				return false, err
			}
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
			if err != nil {
				return false, err
			}

			list, ok := obj.([]*UserEvent)
			if !ok {
				return false, fmt.Errorf("bad: %#v", obj)
			}
			if len(list) != 1 || list[0].Name != "foo" {
				return false, fmt.Errorf("bad: %#v", list)
			}
			header := resp.Header().Get("X-Consul-Index")
			if header == "" || header == "0" {
				return false, fmt.Errorf("bad: %#v", header)
			}
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})
	})
}

func TestEventList_Blocking(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		p := &UserEvent{Name: "test"}
		if err := srv.agent.UserEvent("", p); err != nil {
			t.Fatalf("err: %v", err)
		}

		var index string
		testutil.WaitForResult(func() (bool, error) {
			req, err := http.NewRequest("GET", "/v1/event/list", nil)
			if err != nil {
				return false, err
			}
			resp := httptest.NewRecorder()
			_, err = srv.EventList(resp, req)
			if err != nil {
				return false, err
			}
			header := resp.Header().Get("X-Consul-Index")
			if header == "" || header == "0" {
				return false, fmt.Errorf("bad: %#v", header)
			}
			index = header
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
		})

		go func() {
			time.Sleep(50 * time.Millisecond)
			p := &UserEvent{Name: "second"}
			if err := srv.agent.UserEvent("", p); err != nil {
				t.Fatalf("err: %v", err)
			}
		}()

		testutil.WaitForResult(func() (bool, error) {
			url := "/v1/event/list?index=" + index
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return false, err
			}
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
			if err != nil {
				return false, err
			}

			list, ok := obj.([]*UserEvent)
			if !ok {
				return false, fmt.Errorf("bad: %#v", obj)
			}
			if len(list) != 2 || list[1].Name != "second" {
				return false, fmt.Errorf("bad: %#v", list)
			}
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
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
			if err := srv.agent.UserEvent("", e); err != nil {
				t.Fatalf("err: %v", err)
			}
		}

		// Test that the event order is preserved when name
		// filtering on a list of > 1 matching event.
		testutil.WaitForResult(func() (bool, error) {
			url := "/v1/event/list?name=foo"
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return false, err
			}
			resp := httptest.NewRecorder()
			obj, err := srv.EventList(resp, req)
			if err != nil {
				return false, err
			}
			list, ok := obj.([]*UserEvent)
			if !ok {
				return false, fmt.Errorf("bad: %#v", obj)
			}
			if len(list) != 3 || list[2].ID != expected.ID {
				return false, fmt.Errorf("bad: %#v", list)
			}
			return true, nil
		}, func(err error) {
			t.Fatalf("err: %v", err)
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
