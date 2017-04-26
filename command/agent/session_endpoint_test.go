package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/types"
)

func TestSessionCreate(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		// Create a health check
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       srv.agent.config.NodeName,
			Address:    "127.0.0.1",
			Check: &structs.HealthCheck{
				CheckID:   "consul",
				Node:      srv.agent.config.NodeName,
				Name:      "consul",
				ServiceID: "consul",
				Status:    api.HealthPassing,
			},
		}
		var out struct{}
		if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Associate session with node and 2 health checks
		body := toJSON(map[string]interface{}{
			"Name":      "my-cool-session",
			"Node":      srv.agent.config.NodeName,
			"Checks":    []types.CheckID{consul.SerfCheckID, "consul"},
			"LockDelay": "20s",
		})

		_, err := sessionCreate(srv, body)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestSessionCreateDelete(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		// Create a health check
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       srv.agent.config.NodeName,
			Address:    "127.0.0.1",
			Check: &structs.HealthCheck{
				CheckID:   "consul",
				Node:      srv.agent.config.NodeName,
				Name:      "consul",
				ServiceID: "consul",
				Status:    api.HealthPassing,
			},
		}
		var out struct{}
		if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Associate session with node and 2 health checks, and make it delete on session destroy
		body := toJSON(map[string]interface{}{
			"Name":      "my-cool-session",
			"Node":      srv.agent.config.NodeName,
			"Checks":    []types.CheckID{consul.SerfCheckID, "consul"},
			"LockDelay": "20s",
			"Behavior":  structs.SessionKeysDelete,
		})

		_, err := sessionCreate(srv, body)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestFixupLockDelay(t *testing.T) {
	inp := map[string]interface{}{
		"lockdelay": float64(15),
	}
	if err := FixupLockDelay(inp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if inp["lockdelay"] != 15*time.Second {
		t.Fatalf("bad: %v", inp)
	}

	inp = map[string]interface{}{
		"lockDelay": float64(15 * time.Second),
	}
	if err := FixupLockDelay(inp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if inp["lockDelay"] != 15*time.Second {
		t.Fatalf("bad: %v", inp)
	}

	inp = map[string]interface{}{
		"LockDelay": "15s",
	}
	if err := FixupLockDelay(inp); err != nil {
		t.Fatalf("err: %v", err)
	}
	if inp["LockDelay"] != 15*time.Second {
		t.Fatalf("bad: %v", inp)
	}
}

func TestSessionDestroy(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		id := makeTestSession(t, srv)
		v, err := sessionDestroy(srv, id)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := v, true; got != want {
			t.Fatalf("got %v want %v", got, want)
		}
	})
}

func TestSessionDefaultTTL(t *testing.T) {
	testSessionTTL(t, 10*time.Second, nil)
}

func TestSessionCustomTTL(t *testing.T) {
	ttl := 250 * time.Millisecond
	testSessionTTL(t, ttl, customTTL(ttl))
}

var customTTL = func(ttl time.Duration) func(c *Config) {
	return func(c *Config) {
		c.SessionTTLMinRaw = ttl.String()
		c.SessionTTLMin = ttl
	}
}

func testSessionTTL(t *testing.T, ttl time.Duration, cb func(c *Config)) {
	httpTestWithConfig(t, func(srv *HTTPServer) {
		id := makeTestSessionTTL(t, srv, ttl.String())

		v, err := sessionInfo(srv, id)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(v), 1; got != want {
			t.Fatalf("got %d sessions want %d", got, want)
		}
		if got, want := v[0].TTL, ttl.String(); got != want {
			t.Fatalf("got %v session TTL want %v", got, want)
		}

		// wait for session to time out
		time.Sleep(ttl*structs.SessionTTLMultiplier + ttl)

		v, err = sessionInfo(srv, id)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(v), 0; got != want {
			t.Fatalf("got %d sessions want %d", got, want)
		}
	}, cb)
}

func TestSessionTTLRenew(t *testing.T) {
	ttl := 250 * time.Millisecond
	httpTestWithConfig(t, func(srv *HTTPServer) {
		id := makeTestSessionTTL(t, srv, ttl.String())

		v, err := sessionInfo(srv, id)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(v), 1; got != want {
			t.Fatalf("got %d sessions want %d", got, want)
		}
		if got, want := v[0].TTL, ttl.String(); got != want {
			t.Fatalf("got %q session ttl want %q", got, want)
		}

		// Sleep to consume some time before renew
		time.Sleep(ttl * (structs.SessionTTLMultiplier / 2))

		if v, err = sessionRenew(srv, id); err != nil {
			t.Fatal(err)
		}
		if got, want := len(v), 1; got != want {
			t.Fatalf("got %d sessions want %d", got, want)
		}

		// Sleep for ttl * TTL Multiplier
		time.Sleep(ttl * structs.SessionTTLMultiplier)

		if v, err = sessionInfo(srv, id); err != nil {
			t.Fatal(err)
		}
		if got, want := len(v), 1; got != want {
			t.Fatalf("got %d sessions want %d", got, want)
		}

		// now wait for timeout and expect session to get destroyed
		time.Sleep(ttl * structs.SessionTTLMultiplier)

		if v, err = sessionInfo(srv, id); err != nil {
			t.Fatal(err)
		}
		if got, want := len(v), 0; got != want {
			t.Fatalf("got %d sessions want %d", got, want)
		}
	}, customTTL(ttl))
}

func TestSessionGetIsEmpty(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		// fetch non-existent session id
		v, err := sessionInfo(srv, "adf4238a-882b-9ddc-4a9d-5b6758e4159e")
		if err != nil {
			t.Fatal(err)
		}
		if v == nil || len(v) > 0 {
			t.Fatalf("got nil or non-empty list want empty list")
		}
	})
}

func TestSessionGet(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		id := makeTestSession(t, srv)
		v, err := sessionInfo(srv, id)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(v), 1; got != want {
			t.Fatalf("got %d sessions want %d", got, want)
		}
	})
}

func TestSessionListIsEmpty(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		v, err := sessionList(srv)
		if err != nil {
			t.Fatal(err)
		}
		if v == nil || len(v) > 0 {
			t.Fatalf("got nil or non-empty list want empty list")
		}
	})
}

func TestSessionList(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		var ids []string
		for i := 0; i < 10; i++ {
			ids = append(ids, makeTestSession(t, srv))
		}
		sessions, err := sessionList(srv)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(sessions), len(ids); got != want {
			t.Fatalf("got %d sessions want %d", got, want)
		}
		// todo(fs): compare ids
	})
}

func TestSessionsForNodeIsEmpty(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		v, err := sessionsForNode(srv, srv.agent.config.NodeName)
		if err != nil {
			t.Fatal(err)
		}
		if v == nil || len(v) > 0 {
			t.Fatalf("got nil or non-empty list want empty list")
		}
	})
}

func TestSessionsForNode(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		var ids []string
		for i := 0; i < 10; i++ {
			ids = append(ids, makeTestSession(t, srv))
		}
		v, err := sessionsForNode(srv, srv.agent.config.NodeName)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(v), len(ids); got != want {
			t.Fatalf("got %d sessions want %d", got, want)
		}
		// todo(fs): compare ids
	})
}

func TestSessionDeleteDestroy(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		id := makeTestSessionDelete(t, srv)

		// now create a new key for the session and acquire it
		ok, err := kvPutEphemeral(srv, id, bytes.NewBuffer([]byte("test")))
		if err != nil {
			t.Fatal(err)
		}
		if got, want := ok, true; got != want {
			t.Fatalf("got %v want %v", got, want)
		}

		// now destroy the session, this should delete the key created above
		b, err := sessionDestroy(srv, id)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := b, true; got != want {
			t.Fatalf("got %v want %v", got, want)
		}

		// Verify that the key is gone
		keys, err := kvGetEphemeral(srv, 404)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := len(keys), 0; got != want {
			t.Fatalf("got %d keys want %d", got, want)
		}
	})
}

func toJSON(v interface{}) io.Reader {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(v); err != nil {
		panic(err)
	}
	return buf
}

func makeTestSession(t *testing.T, srv *HTTPServer) string {
	v, err := sessionCreate(srv, nil)
	if err != nil {
		t.Fatal(err)
	}
	return v.ID
}

func makeTestSessionDelete(t *testing.T, srv *HTTPServer) string {
	body := toJSON(map[string]interface{}{"Behavior": "delete"})
	v, err := sessionCreate(srv, body)
	if err != nil {
		t.Fatal(err)
	}
	return v.ID
}

func makeTestSessionTTL(t *testing.T, srv *HTTPServer, ttl string) string {
	body := toJSON(map[string]interface{}{"TTL": ttl})
	v, err := sessionCreate(srv, body)
	if err != nil {
		t.Fatal(err)
	}
	return v.ID
}

func kvGetEphemeral(srv *HTTPServer, code int) (v structs.DirEntries, err error) {
	req, _ := http.NewRequest("GET", "/v1/kv/ephemeral", nil)
	err = call(req, srv.KVSEndpoint, code, &v)
	return
}

func kvPutEphemeral(srv *HTTPServer, id string, body io.Reader) (v bool, err error) {
	req, _ := http.NewRequest("PUT", "/v1/kv/ephemeral?acquire="+id, body)
	err = call(req, srv.KVSEndpoint, 200, &v)
	return
}

func sessionCreate(srv *HTTPServer, body io.Reader) (v sessionCreateResponse, err error) {
	req, _ := http.NewRequest("PUT", "/v1/session/create", body)
	err = call(req, srv.SessionCreate, 200, &v)
	return
}

func sessionDestroy(srv *HTTPServer, id string) (v bool, err error) {
	req, _ := http.NewRequest("PUT", "/v1/session/destroy/"+id, nil)
	err = call(req, srv.SessionDestroy, 200, &v)
	return
}

func sessionList(srv *HTTPServer) (v structs.Sessions, err error) {
	req, _ := http.NewRequest("GET", "/v1/session/list", nil)
	err = call(req, srv.SessionList, 200, &v)
	return
}

func sessionInfo(srv *HTTPServer, id string) (v structs.Sessions, err error) {
	req, _ := http.NewRequest("GET", "/v1/session/info/"+id, nil)
	err = call(req, srv.SessionGet, 200, &v)
	return
}

func sessionsForNode(srv *HTTPServer, node string) (v structs.Sessions, err error) {
	req, _ := http.NewRequest("GET", "/v1/session/node/"+node, nil)
	err = call(req, srv.SessionsForNode, 200, &v)
	return
}

func sessionRenew(srv *HTTPServer, id string) (v structs.Sessions, err error) {
	req, _ := http.NewRequest("PUT", "/v1/session/renew/"+id, nil)
	err = call(req, srv.SessionRenew, 200, &v)
	return
}

type handler func(http.ResponseWriter, *http.Request) (interface{}, error)

func call(req *http.Request, h handler, code int, v interface{}) error {
	if reflect.TypeOf(v).Kind() != reflect.Ptr {
		panic("v must be pointer")
	}

	prefix := fmt.Sprintf("[%s %s]", req.Method, req.URL)
	resp := httptest.NewRecorder()
	obj, err := h(resp, req)
	if err != nil {
		return fmt.Errorf("%s: failed: %s", prefix, err)
	}
	if got, want := resp.Code, code; got != want {
		return fmt.Errorf("%s: got status code %d want %d", prefix, got, want)
	}
	if obj == nil {
		return nil
	}

	// type(obj) == type(*v)
	if got, want := reflect.TypeOf(obj), reflect.TypeOf(v).Elem(); got != want {
		return fmt.Errorf("%s: got type %s want %s", prefix, got, want)
	}
	// *v = obj
	reflect.ValueOf(v).Elem().Set(reflect.ValueOf(obj))
	return nil
}
