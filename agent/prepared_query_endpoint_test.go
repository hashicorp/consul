// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/consul/testrpc"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/types"
)

// MockPreparedQuery is a fake endpoint that we inject into the Consul server
// in order to observe the RPC calls made by these HTTP endpoints. This lets
// us make sure that the request is being formed properly without having to
// set up a realistic environment for prepared queries, which is a huge task and
// already done in detail inside the prepared query endpoint's unit tests. If we
// can prove this formats proper requests into that then we should be good to
// go. We will do a single set of end-to-end tests in here to make sure that the
// server is wired up to the right endpoint when not "injected".
type MockPreparedQuery struct {
	applyFn   func(*structs.PreparedQueryRequest, *string) error
	getFn     func(*structs.PreparedQuerySpecificRequest, *structs.IndexedPreparedQueries) error
	listFn    func(*structs.DCSpecificRequest, *structs.IndexedPreparedQueries) error
	executeFn func(*structs.PreparedQueryExecuteRequest, *structs.PreparedQueryExecuteResponse) error
	explainFn func(*structs.PreparedQueryExecuteRequest, *structs.PreparedQueryExplainResponse) error
}

func (m *MockPreparedQuery) Apply(args *structs.PreparedQueryRequest,
	reply *string) (err error) {
	if m.applyFn != nil {
		return m.applyFn(args, reply)
	}
	return fmt.Errorf("should not have called Apply")
}

func (m *MockPreparedQuery) Get(args *structs.PreparedQuerySpecificRequest,
	reply *structs.IndexedPreparedQueries) error {
	if m.getFn != nil {
		return m.getFn(args, reply)
	}
	return fmt.Errorf("should not have called Get")
}

func (m *MockPreparedQuery) List(args *structs.DCSpecificRequest,
	reply *structs.IndexedPreparedQueries) error {
	if m.listFn != nil {
		return m.listFn(args, reply)
	}
	return fmt.Errorf("should not have called List")
}

func (m *MockPreparedQuery) Execute(args *structs.PreparedQueryExecuteRequest,
	reply *structs.PreparedQueryExecuteResponse) error {
	if m.executeFn != nil {
		return m.executeFn(args, reply)
	}
	return fmt.Errorf("should not have called Execute")
}

func (m *MockPreparedQuery) Explain(args *structs.PreparedQueryExecuteRequest,
	reply *structs.PreparedQueryExplainResponse) error {
	if m.explainFn != nil {
		return m.explainFn(args, reply)
	}
	return fmt.Errorf("should not have called Explain")
}

func TestPreparedQuery_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	m := MockPreparedQuery{
		applyFn: func(args *structs.PreparedQueryRequest, reply *string) error {
			expected := &structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryCreate,
				Query: &structs.PreparedQuery{
					Name:    "my-query",
					Session: "my-session",
					Service: structs.ServiceQuery{
						Service: "my-service",
						Failover: structs.QueryFailoverOptions{
							NearestN:    4,
							Datacenters: []string{"dc1", "dc2"},
						},
						IgnoreCheckIDs: []types.CheckID{"broken_check"},
						OnlyPassing:    true,
						Tags:           []string{"foo", "bar"},
						NodeMeta:       map[string]string{"somekey": "somevalue"},
						ServiceMeta:    map[string]string{"env": "prod"},
					},
					DNS: structs.QueryDNSOptions{
						TTL: "10s",
					},
				},
				WriteRequest: structs.WriteRequest{
					Token: "my-token",
				},
			}
			if !reflect.DeepEqual(args, expected) {
				t.Fatalf("bad: %v", args)
			}

			*reply = "my-id"
			return nil
		},
	}
	if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"Name":    "my-query",
		"Session": "my-session",
		"Service": map[string]interface{}{
			"Service": "my-service",
			"Failover": map[string]interface{}{
				"NearestN":    4,
				"Datacenters": []string{"dc1", "dc2"},
			},
			"IgnoreCheckIDs": []string{"broken_check"},
			"OnlyPassing":    true,
			"Tags":           []string{"foo", "bar"},
			"NodeMeta":       map[string]string{"somekey": "somevalue"},
			"ServiceMeta":    map[string]string{"env": "prod"},
		},
		"DNS": map[string]interface{}{
			"TTL": "10s",
		},
	}
	if err := enc.Encode(raw); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("POST", "/v1/query", body)
	req.Header.Add("X-Consul-Token", "my-token")
	resp := httptest.NewRecorder()
	obj, err := a.srv.PreparedQueryGeneral(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("bad code: %d", resp.Code)
	}
	r, ok := obj.(preparedQueryCreateResponse)
	if !ok {
		t.Fatalf("unexpected: %T", obj)
	}
	if r.ID != "my-id" {
		t.Fatalf("bad ID: %s", r.ID)
	}
}

func TestPreparedQuery_List(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			listFn: func(args *structs.DCSpecificRequest, reply *structs.IndexedPreparedQueries) error {
				// Return an empty response.
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQueryGeneral(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueries)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if r == nil || len(r) != 0 {
			t.Fatalf("bad: %v", r)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			listFn: func(args *structs.DCSpecificRequest, reply *structs.IndexedPreparedQueries) error {
				expected := &structs.DCSpecificRequest{
					Datacenter: "dc1",
					QueryOptions: structs.QueryOptions{
						Token:             "my-token",
						RequireConsistent: true,
					},
				}
				if !reflect.DeepEqual(args, expected) {
					t.Fatalf("bad: %v", args)
				}

				query := &structs.PreparedQuery{
					ID: "my-id",
				}
				reply.Queries = append(reply.Queries, query)
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query?consistent=true", body)
		req.Header.Add("X-Consul-Token", "my-token")
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQueryGeneral(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueries)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if len(r) != 1 || r[0].ID != "my-id" {
			t.Fatalf("bad: %v", r)
		}
	})
}

func TestPreparedQuery_Execute(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				// Just return an empty response.
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id/execute", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueryExecuteResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if r.Nodes == nil || len(r.Nodes) != 0 {
			t.Fatalf("bad: %v", r)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				expected := &structs.PreparedQueryExecuteRequest{
					Datacenter:    "dc1",
					QueryIDOrName: "my-id",
					Limit:         5,
					Source: structs.QuerySource{
						Datacenter: "dc1",
						Node:       "my-node",
					},
					Agent: structs.QuerySource{
						Datacenter: a.Config.Datacenter,
						Node:       a.Config.NodeName,
					},
					QueryOptions: structs.QueryOptions{
						Token:             "my-token",
						RequireConsistent: true,
					},
				}
				if !reflect.DeepEqual(args, expected) {
					t.Fatalf("bad: %v", args)
				}

				// Just set something so we can tell this is returned.
				reply.Failovers = 99
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id/execute?consistent=true&near=my-node&limit=5", body)
		req.Header.Add("X-Consul-Token", "my-token")
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueryExecuteResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if r.Failovers != 99 {
			t.Fatalf("bad: %v", r)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				expected := &structs.PreparedQueryExecuteRequest{
					Datacenter:    "dc1",
					QueryIDOrName: "my-id",
					Limit:         5,
					Source: structs.QuerySource{
						Datacenter: "dc1",
						Node:       "_ip",
						Ip:         "127.0.0.1",
					},
					Agent: structs.QuerySource{
						Datacenter: a.Config.Datacenter,
						Node:       a.Config.NodeName,
					},
					QueryOptions: structs.QueryOptions{
						Token:             "my-token",
						RequireConsistent: true,
					},
				}
				if !reflect.DeepEqual(args, expected) {
					t.Fatalf("bad: %v", args)
				}

				// Just set something so we can tell this is returned.
				reply.Failovers = 99
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id/execute?consistent=true&near=_ip&limit=5", body)
		req.Header.Add("X-Consul-Token", "my-token")
		req.Header.Add("X-Forwarded-For", "127.0.0.1")
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueryExecuteResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if r.Failovers != 99 {
			t.Fatalf("bad: %v", r)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				expected := &structs.PreparedQueryExecuteRequest{
					Datacenter:    "dc1",
					QueryIDOrName: "my-id",
					Limit:         5,
					Source: structs.QuerySource{
						Datacenter: "dc1",
						Node:       "_ip",
						Ip:         "198.18.0.1",
					},
					Agent: structs.QuerySource{
						Datacenter: a.Config.Datacenter,
						Node:       a.Config.NodeName,
					},
					QueryOptions: structs.QueryOptions{
						Token:             "my-token",
						RequireConsistent: true,
					},
				}
				if !reflect.DeepEqual(args, expected) {
					t.Fatalf("bad: %v", args)
				}

				// Just set something so we can tell this is returned.
				reply.Failovers = 99
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id/execute?consistent=true&near=_ip&limit=5", body)
		req.Header.Add("X-Consul-Token", "my-token")
		req.Header.Add("X-Forwarded-For", "198.18.0.1")
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueryExecuteResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if r.Failovers != 99 {
			t.Fatalf("bad: %v", r)
		}

		req, _ = http.NewRequest("GET", "/v1/query/my-id/execute?consistent=true&near=_ip&limit=5", body)
		req.Header.Add("X-Consul-Token", "my-token")
		req.Header.Add("X-Forwarded-For", "198.18.0.1, 198.19.0.1")
		resp = httptest.NewRecorder()
		obj, err = a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok = obj.(structs.PreparedQueryExecuteResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if r.Failovers != 99 {
			t.Fatalf("bad: %v", r)
		}
	})

	// Ensure the proper params are set when no special args are passed
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				if args.Source.Node != "" {
					t.Fatalf("expect node to be empty, got %q", args.Source.Node)
				}
				expect := structs.QuerySource{
					Datacenter: a.Config.Datacenter,
					Node:       a.Config.NodeName,
				}
				if !reflect.DeepEqual(args.Agent, expect) {
					t.Fatalf("expect: %#v\nactual: %#v", expect, args.Agent)
				}
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		req, _ := http.NewRequest("GET", "/v1/query/my-id/execute", nil)
		resp := httptest.NewRecorder()
		if _, err := a.srv.PreparedQuerySpecific(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
	})

	// Ensure WAN translation occurs for a response outside of the local DC.
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, `
			datacenter = "dc1"
			translate_wan_addrs = true
		`)
		defer a.Shutdown()

		m := MockPreparedQuery{
			executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				nodesResponse := make(structs.CheckServiceNodes, 1)
				nodesResponse[0].Node = &structs.Node{
					Node: "foo", Address: "127.0.0.1",
					TaggedAddresses: map[string]string{
						"wan": "127.0.0.2",
					},
				}
				nodesResponse[0].Service = &structs.NodeService{
					Service: "foo",
					Address: "10.0.1.1",
					Port:    8080,
					TaggedAddresses: map[string]structs.ServiceAddress{
						"wan": {
							Address: "198.18.0.1",
							Port:    80,
						},
					},
				}
				reply.Nodes = nodesResponse
				reply.Datacenter = "dc2"
				return nil
			},
		}
		require.NoError(t, a.registerEndpoint("PreparedQuery", &m))

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id/execute?dc=dc2", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.Code)
		r, ok := obj.(structs.PreparedQueryExecuteResponse)
		require.True(t, ok, "unexpected: %T", obj)
		require.NotNil(t, r.Nodes)
		require.Len(t, r.Nodes, 1)

		node := r.Nodes[0]
		require.NotNil(t, node.Node)
		require.Equal(t, "127.0.0.2", node.Node.Address)
		require.NotNil(t, node.Service)
		require.Equal(t, "198.18.0.1", node.Service.Address)
		require.Equal(t, 80, node.Service.Port)
	})

	// Ensure WAN translation doesn't occur for the local DC.
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, `
			datacenter = "dc1"
			translate_wan_addrs = true
		`)
		defer a.Shutdown()

		m := MockPreparedQuery{
			executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				nodesResponse := make(structs.CheckServiceNodes, 1)
				nodesResponse[0].Node = &structs.Node{
					Node: "foo", Address: "127.0.0.1",
					TaggedAddresses: map[string]string{
						"wan": "127.0.0.2",
					},
				}
				reply.Nodes = nodesResponse
				reply.Datacenter = "dc1"
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id/execute?dc=dc2", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueryExecuteResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if r.Nodes == nil || len(r.Nodes) != 1 {
			t.Fatalf("bad: %v", r)
		}

		node := r.Nodes[0]
		if node.Node.Address != "127.0.0.1" {
			t.Fatalf("bad: %v", node.Node)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/not-there/execute", body)
		resp := httptest.NewRecorder()
		_, err := a.srv.PreparedQuerySpecific(resp, req)
		if httpErr, ok := err.(HTTPError); ok {
			if httpErr.StatusCode != 404 {
				t.Fatalf("expected status 404 but got %d", httpErr.StatusCode)
			}
		} else {
			t.Fatalf("expected HTTP error but got %v", err)
		}
	})
}

func TestPreparedQuery_ExecuteCached(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	failovers := int32(99)

	m := MockPreparedQuery{
		executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
			// Just set something so we can tell this is returned.
			reply.Failovers = int(atomic.LoadInt32(&failovers))
			return nil
		},
	}
	if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	doRequest := func(expectFailovers int, expectCache string, revalidate bool) {
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id/execute?cached", body)

		if revalidate {
			req.Header.Set("Cache-Control", "must-revalidate")
		}

		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)

		require.NoError(t, err)
		require.Equal(t, 200, resp.Code)

		r, ok := obj.(structs.PreparedQueryExecuteResponse)
		require.True(t, ok)
		require.Equal(t, expectFailovers, r.Failovers)

		require.Equal(t, expectCache, resp.Header().Get("X-Cache"))
	}

	// Should be a miss at first
	doRequest(99, "MISS", false)

	// Change the actual response
	atomic.StoreInt32(&failovers, 66)

	// Request again, should be a cache hit and have the cached (not current)
	// value.
	doRequest(99, "HIT", false)

	// Request with max age that should invalidate cache. note that this will be
	// sent as max-age=0 as that uses seconds but that should cause immediate
	// invalidation rather than being ignored as an unset value.
	doRequest(66, "MISS", true)
}

func TestPreparedQuery_Explain(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			explainFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExplainResponse) error {
				expected := &structs.PreparedQueryExecuteRequest{
					Datacenter:    "dc1",
					QueryIDOrName: "my-id",
					Limit:         5,
					Source: structs.QuerySource{
						Datacenter: "dc1",
						Node:       "my-node",
					},
					Agent: structs.QuerySource{
						Datacenter: a.Config.Datacenter,
						Node:       a.Config.NodeName,
					},
					QueryOptions: structs.QueryOptions{
						Token:             "my-token",
						RequireConsistent: true,
					},
				}
				if !reflect.DeepEqual(args, expected) {
					t.Fatalf("bad: %v", args)
				}

				// Just set something so we can tell this is returned.
				reply.Query.Name = "hello"
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id/explain?consistent=true&near=my-node&limit=5", body)
		req.Header.Add("X-Consul-Token", "my-token")
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueryExplainResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if r.Query.Name != "hello" {
			t.Fatalf("bad: %v", r)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/not-there/explain", body)
		resp := httptest.NewRecorder()
		_, err := a.srv.PreparedQuerySpecific(resp, req)
		if httpErr, ok := err.(HTTPError); ok {
			if httpErr.StatusCode != 404 {
				t.Fatalf("expected status 404 but got %d", httpErr.StatusCode)
			}
		} else {
			t.Fatalf("expected HTTP error but got %v", err)
		}
	})

	// Ensure that Connect is passed through
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			executeFn: func(args *structs.PreparedQueryExecuteRequest, reply *structs.PreparedQueryExecuteResponse) error {
				require.True(t, args.Connect)
				return nil
			},
		}
		require.NoError(t, a.registerEndpoint("PreparedQuery", &m))

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id/execute?connect=true", body)
		resp := httptest.NewRecorder()
		_, err := a.srv.PreparedQuerySpecific(resp, req)
		require.NoError(t, err)
		require.Equal(t, 200, resp.Code)
	})
}

func TestPreparedQuery_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		m := MockPreparedQuery{
			getFn: func(args *structs.PreparedQuerySpecificRequest, reply *structs.IndexedPreparedQueries) error {
				expected := &structs.PreparedQuerySpecificRequest{
					Datacenter: "dc1",
					QueryID:    "my-id",
					QueryOptions: structs.QueryOptions{
						Token:             "my-token",
						RequireConsistent: true,
					},
				}
				if !reflect.DeepEqual(args, expected) {
					t.Fatalf("bad: %v", args)
				}

				query := &structs.PreparedQuery{
					ID: "my-id",
				}
				reply.Queries = append(reply.Queries, query)
				return nil
			},
		}
		if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
			t.Fatalf("err: %v", err)
		}

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/my-id?consistent=true", body)
		req.Header.Add("X-Consul-Token", "my-token")
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueries)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if len(r) != 1 || r[0].ID != "my-id" {
			t.Fatalf("bad: %v", r)
		}
	})

	t.Run("", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/f004177f-2c28-83b7-4229-eacc25fe55d1", body)
		resp := httptest.NewRecorder()
		_, err := a.srv.PreparedQuerySpecific(resp, req)
		if httpErr, ok := err.(HTTPError); ok {
			if httpErr.StatusCode != 404 {
				t.Fatalf("expected status 404 but got %d", httpErr.StatusCode)
			}
		} else {
			t.Fatalf("expected HTTP error but got %v", err)
		}
	})
}

func TestPreparedQuery_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	m := MockPreparedQuery{
		applyFn: func(args *structs.PreparedQueryRequest, reply *string) error {
			expected := &structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryUpdate,
				Query: &structs.PreparedQuery{
					ID:      "my-id",
					Name:    "my-query",
					Session: "my-session",
					Service: structs.ServiceQuery{
						Service: "my-service",
						Failover: structs.QueryFailoverOptions{
							NearestN:    4,
							Datacenters: []string{"dc1", "dc2"},
						},
						OnlyPassing: true,
						Tags:        []string{"foo", "bar"},
						NodeMeta:    map[string]string{"somekey": "somevalue"},
					},
					DNS: structs.QueryDNSOptions{
						TTL: "10s",
					},
				},
				WriteRequest: structs.WriteRequest{
					Token: "my-token",
				},
			}
			if !reflect.DeepEqual(args, expected) {
				t.Fatalf("bad: %v", args)
			}

			*reply = "don't care"
			return nil
		},
	}
	if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"ID":      "this should get ignored",
		"Name":    "my-query",
		"Session": "my-session",
		"Service": map[string]interface{}{
			"Service": "my-service",
			"Failover": map[string]interface{}{
				"NearestN":    4,
				"Datacenters": []string{"dc1", "dc2"},
			},
			"OnlyPassing": true,
			"Tags":        []string{"foo", "bar"},
			"NodeMeta":    map[string]string{"somekey": "somevalue"},
		},
		"DNS": map[string]interface{}{
			"TTL": "10s",
		},
	}
	if err := enc.Encode(raw); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("PUT", "/v1/query/my-id", body)
	req.Header.Add("X-Consul-Token", "my-token")
	resp := httptest.NewRecorder()
	if _, err := a.srv.PreparedQuerySpecific(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("bad code: %d", resp.Code)
	}
}

func TestPreparedQuery_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	m := MockPreparedQuery{
		applyFn: func(args *structs.PreparedQueryRequest, reply *string) error {
			expected := &structs.PreparedQueryRequest{
				Datacenter: "dc1",
				Op:         structs.PreparedQueryDelete,
				Query: &structs.PreparedQuery{
					ID: "my-id",
				},
				WriteRequest: structs.WriteRequest{
					Token: "my-token",
				},
			}
			if !reflect.DeepEqual(args, expected) {
				t.Fatalf("bad: %v", args)
			}

			*reply = "don't care"
			return nil
		},
	}
	if err := a.registerEndpoint("PreparedQuery", &m); err != nil {
		t.Fatalf("err: %v", err)
	}

	body := bytes.NewBuffer(nil)
	enc := json.NewEncoder(body)
	raw := map[string]interface{}{
		"ID": "this should get ignored",
	}
	if err := enc.Encode(raw); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("DELETE", "/v1/query/my-id", body)
	req.Header.Add("X-Consul-Token", "my-token")
	resp := httptest.NewRecorder()
	if _, err := a.srv.PreparedQuerySpecific(resp, req); err != nil {
		t.Fatalf("err: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("bad code: %d", resp.Code)
	}
}

func TestPreparedQuery_parseLimit(t *testing.T) {
	t.Parallel()
	body := bytes.NewBuffer(nil)
	req, _ := http.NewRequest("GET", "/v1/query", body)
	limit := 99
	if err := parseLimit(req, &limit); err != nil {
		t.Fatalf("err: %v", err)
	}
	if limit != 0 {
		t.Fatalf("bad limit: %d", limit)
	}

	req, _ = http.NewRequest("GET", "/v1/query?limit=11", body)
	if err := parseLimit(req, &limit); err != nil {
		t.Fatalf("err: %v", err)
	}
	if limit != 11 {
		t.Fatalf("bad limit: %d", limit)
	}

	req, _ = http.NewRequest("GET", "/v1/query?limit=bob", body)
	if err := parseLimit(req, &limit); err == nil {
		t.Fatalf("bad: %v", err)
	}
}

// Since we've done exhaustive testing of the calls into the endpoints above
// this is just a basic end-to-end sanity check to make sure things are wired
// correctly when calling through to the real endpoints.
func TestPreparedQuery_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register a node and a service.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
			Address:    "127.0.0.1",
			Service: &structs.NodeService{
				Service: "my-service",
			},
		}
		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Create a query.
	var id string
	{
		body := bytes.NewBuffer(nil)
		enc := json.NewEncoder(body)
		raw := map[string]interface{}{
			"Name": "my-query",
			"Service": map[string]interface{}{
				"Service": "my-service",
			},
		}
		if err := enc.Encode(raw); err != nil {
			t.Fatalf("err: %v", err)
		}

		req, _ := http.NewRequest("POST", "/v1/query", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQueryGeneral(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(preparedQueryCreateResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		id = r.ID
	}

	// List them all.
	{
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query", body)
		req.Header.Add("X-Consul-Token", "root")
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQueryGeneral(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueries)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if len(r) != 1 {
			t.Fatalf("bad: %v", r)
		}
	}

	// Execute it.
	{
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/"+id+"/execute", body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueryExecuteResponse)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if len(r.Nodes) != 1 {
			t.Fatalf("bad: %v", r)
		}
	}

	// Read it back.
	{
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("GET", "/v1/query/"+id, body)
		resp := httptest.NewRecorder()
		obj, err := a.srv.PreparedQuerySpecific(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
		r, ok := obj.(structs.PreparedQueries)
		if !ok {
			t.Fatalf("unexpected: %T", obj)
		}
		if len(r) != 1 {
			t.Fatalf("bad: %v", r)
		}
	}

	// Make an update to it.
	{
		body := bytes.NewBuffer(nil)
		enc := json.NewEncoder(body)
		raw := map[string]interface{}{
			"Name": "my-query",
			"Service": map[string]interface{}{
				"Service":     "my-service",
				"OnlyPassing": true,
			},
		}
		if err := enc.Encode(raw); err != nil {
			t.Fatalf("err: %v", err)
		}

		req, _ := http.NewRequest("PUT", "/v1/query/"+id, body)
		resp := httptest.NewRecorder()
		if _, err := a.srv.PreparedQuerySpecific(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
	}

	// Delete it.
	{
		body := bytes.NewBuffer(nil)
		req, _ := http.NewRequest("DELETE", "/v1/query/"+id, body)
		resp := httptest.NewRecorder()
		if _, err := a.srv.PreparedQuerySpecific(resp, req); err != nil {
			t.Fatalf("err: %v", err)
		}
		if resp.Code != 200 {
			t.Fatalf("bad code: %d", resp.Code)
		}
	}
}
