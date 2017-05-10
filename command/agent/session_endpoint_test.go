package agent

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		body := bytes.NewBuffer(nil)
		enc := json.NewEncoder(body)
		raw := map[string]interface{}{
			"Name":      "my-cool-session",
			"Node":      srv.agent.config.NodeName,
			"Checks":    []types.CheckID{consul.SerfCheckID, "consul"},
			"LockDelay": "20s",
		}
		enc.Encode(raw)

		req, _ := http.NewRequest("PUT", "/v1/session/create", body)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, ok := obj.(sessionCreateResponse); !ok {
			t.Fatalf("should work")
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
		body := bytes.NewBuffer(nil)
		enc := json.NewEncoder(body)
		raw := map[string]interface{}{
			"Name":      "my-cool-session",
			"Node":      srv.agent.config.NodeName,
			"Checks":    []types.CheckID{consul.SerfCheckID, "consul"},
			"LockDelay": "20s",
			"Behavior":  structs.SessionKeysDelete,
		}
		enc.Encode(raw)

		req, _ := http.NewRequest("PUT", "/v1/session/create", body)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionCreate(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if _, ok := obj.(sessionCreateResponse); !ok {
			t.Fatalf("should work")
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

func makeTestSession(t *testing.T, srv *HTTPServer) string {
	req, _ := http.NewRequest("PUT", "/v1/session/create", nil)
	resp := httptest.NewRecorder()
	obj, err := srv.SessionCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	sessResp := obj.(sessionCreateResponse)
	return sessResp.ID
}

func makeTestSessionDelete(t *testing.T, srv *HTTPServer) string {
	// Create Session with delete behavior
	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"Behavior": "delete",
	}
	enc.Encode(raw)

	req, _ := http.NewRequest("PUT", "/v1/session/create", body)
	resp := httptest.NewRecorder()
	obj, err := srv.SessionCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	sessResp := obj.(sessionCreateResponse)
	return sessResp.ID
}

func makeTestSessionTTL(t *testing.T, srv *HTTPServer, ttl string) string {
	// Create Session with TTL
	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"TTL": ttl,
	}
	enc.Encode(raw)

	req, _ := http.NewRequest("PUT", "/v1/session/create", body)
	resp := httptest.NewRecorder()
	obj, err := srv.SessionCreate(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	sessResp := obj.(sessionCreateResponse)
	return sessResp.ID
}

func TestSessionDestroy(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		id := makeTestSession(t, srv)

		req, _ := http.NewRequest("PUT", "/v1/session/destroy/"+id, nil)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionDestroy(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp := obj.(bool); !resp {
			t.Fatalf("should work")
		}
	})
}

func TestSessionCustomTTL(t *testing.T) {
	ttl := 250 * time.Millisecond
	testSessionTTL(t, ttl, customTTL(ttl))
}

func customTTL(d time.Duration) func(c *Config) {
	return func(c *Config) {
		c.SessionTTLMinRaw = d.String()
		c.SessionTTLMin = d
	}
}

func testSessionTTL(t *testing.T, ttl time.Duration, cb func(c *Config)) {
	httpTestWithConfig(t, func(srv *HTTPServer) {
		TTL := ttl.String()

		id := makeTestSessionTTL(t, srv, TTL)

		req, _ := http.NewRequest("GET", "/v1/session/info/"+id, nil)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if len(respObj) != 1 {
			t.Fatalf("bad: %v", respObj)
		}
		if respObj[0].TTL != TTL {
			t.Fatalf("Incorrect TTL: %s", respObj[0].TTL)
		}

		time.Sleep(ttl*structs.SessionTTLMultiplier + ttl)

		req, _ = http.NewRequest("GET", "/v1/session/info/"+id, nil)
		resp = httptest.NewRecorder()
		obj, err = srv.SessionGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok = obj.(structs.Sessions)
		if len(respObj) != 0 {
			t.Fatalf("session '%s' should have been destroyed", id)
		}
	}, cb)
}

func TestSessionTTLRenew(t *testing.T) {
	ttl := 250 * time.Millisecond
	TTL := ttl.String()
	httpTestWithConfig(t, func(srv *HTTPServer) {
		id := makeTestSessionTTL(t, srv, TTL)

		req, _ := http.NewRequest("GET", "/v1/session/info/"+id, nil)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if len(respObj) != 1 {
			t.Fatalf("bad: %v", respObj)
		}
		if respObj[0].TTL != TTL {
			t.Fatalf("Incorrect TTL: %s", respObj[0].TTL)
		}

		// Sleep to consume some time before renew
		time.Sleep(ttl * (structs.SessionTTLMultiplier / 2))

		req, _ = http.NewRequest("PUT", "/v1/session/renew/"+id, nil)
		resp = httptest.NewRecorder()
		obj, err = srv.SessionRenew(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok = obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if len(respObj) != 1 {
			t.Fatalf("bad: %v", respObj)
		}

		// Sleep for ttl * TTL Multiplier
		time.Sleep(ttl * structs.SessionTTLMultiplier)

		req, _ = http.NewRequest("GET", "/v1/session/info/"+id, nil)
		resp = httptest.NewRecorder()
		obj, err = srv.SessionGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok = obj.(structs.Sessions)
		if !ok {
			t.Fatalf("session '%s' should have renewed", id)
		}
		if len(respObj) != 1 {
			t.Fatalf("session '%s' should have renewed", id)
		}

		// now wait for timeout and expect session to get destroyed
		time.Sleep(ttl * structs.SessionTTLMultiplier)

		req, _ = http.NewRequest("GET", "/v1/session/info/"+id, nil)
		resp = httptest.NewRecorder()
		obj, err = srv.SessionGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok = obj.(structs.Sessions)
		if !ok {
			t.Fatalf("session '%s' should have destroyed", id)
		}
		if len(respObj) != 0 {
			t.Fatalf("session '%s' should have destroyed", id)
		}
	}, customTTL(ttl))
}

func TestSessionGet(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		req, _ := http.NewRequest("GET", "/v1/session/info/adf4238a-882b-9ddc-4a9d-5b6758e4159e", nil)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if respObj == nil || len(respObj) != 0 {
			t.Fatalf("bad: %v", respObj)
		}
	})

	httpTest(t, func(srv *HTTPServer) {
		id := makeTestSession(t, srv)

		req, _ := http.NewRequest("GET", "/v1/session/info/"+id, nil)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionGet(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if len(respObj) != 1 {
			t.Fatalf("bad: %v", respObj)
		}
	})
}

func TestSessionList(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		req, _ := http.NewRequest("GET", "/v1/session/list", nil)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionList(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if respObj == nil || len(respObj) != 0 {
			t.Fatalf("bad: %v", respObj)
		}
	})

	httpTest(t, func(srv *HTTPServer) {
		var ids []string
		for i := 0; i < 10; i++ {
			ids = append(ids, makeTestSession(t, srv))
		}

		req, _ := http.NewRequest("GET", "/v1/session/list", nil)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionList(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if len(respObj) != 10 {
			t.Fatalf("bad: %v", respObj)
		}
	})
}

func TestSessionsForNode(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		req, _ := http.NewRequest("GET", "/v1/session/node/"+srv.agent.config.NodeName, nil)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionsForNode(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if respObj == nil || len(respObj) != 0 {
			t.Fatalf("bad: %v", respObj)
		}
	})

	httpTest(t, func(srv *HTTPServer) {
		var ids []string
		for i := 0; i < 10; i++ {
			ids = append(ids, makeTestSession(t, srv))
		}

		req, _ := http.NewRequest("GET", "/v1/session/node/"+srv.agent.config.NodeName, nil)
		resp := httptest.NewRecorder()
		obj, err := srv.SessionsForNode(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		respObj, ok := obj.(structs.Sessions)
		if !ok {
			t.Fatalf("should work")
		}
		if len(respObj) != 10 {
			t.Fatalf("bad: %v", respObj)
		}
	})
}

func TestSessionDeleteDestroy(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		id := makeTestSessionDelete(t, srv)

		// now create a new key for the session and acquire it
		buf := bytes.NewBuffer([]byte("test"))
		req, _ := http.NewRequest("PUT", "/v1/kv/ephemeral?acquire="+id, buf)
		resp := httptest.NewRecorder()
		obj, err := srv.KVSEndpoint(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		if res := obj.(bool); !res {
			t.Fatalf("should work")
		}

		// now destroy the session, this should delete the key created above
		req, _ = http.NewRequest("PUT", "/v1/session/destroy/"+id, nil)
		resp = httptest.NewRecorder()
		obj, err = srv.SessionDestroy(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp := obj.(bool); !resp {
			t.Fatalf("should work")
		}

		// Verify that the key is gone
		req, _ = http.NewRequest("GET", "/v1/kv/ephemeral", nil)
		resp = httptest.NewRecorder()
		obj, _ = srv.KVSEndpoint(resp, req)
		res, found := obj.(structs.DirEntries)
		if found || len(res) != 0 {
			t.Fatalf("bad: %v found, should be nothing", res)
		}
	})
}
