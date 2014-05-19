package agent

import (
	"bytes"
	"encoding/json"
	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"net/http/httptest"
	"testing"
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
			"Node":   srv.agent.config.NodeName,
			"Checks": []string{consul.SerfCheckID, "consul"},
		}
		enc.Encode(raw)

		req, err := http.NewRequest("PUT", "/v1/session/create", body)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

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

func makeTestSession(t *testing.T, srv *HTTPServer) string {
	req, err := http.NewRequest("PUT", "/v1/session/create", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
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

		req, err := http.NewRequest("PUT", "/v1/session/destroy/"+id, nil)
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

func TestSessionGet(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		id := makeTestSession(t, srv)

		req, err := http.NewRequest("GET",
			"/v1/session/info/"+id, nil)
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
		var ids []string
		for i := 0; i < 10; i++ {
			ids = append(ids, makeTestSession(t, srv))
		}

		req, err := http.NewRequest("GET", "/v1/session/list", nil)
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
		var ids []string
		for i := 0; i < 10; i++ {
			ids = append(ids, makeTestSession(t, srv))
		}

		req, err := http.NewRequest("GET",
			"/v1/session/node/"+srv.agent.config.NodeName, nil)
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
