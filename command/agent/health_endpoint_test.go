package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/testutil/wait"
	"github.com/hashicorp/serf/coordinate"
)

func TestHealthChecksInState(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		retry.Fatal(t, func() error {
			nodes, err := getHealthChecks(srv, "/v1/health/state/warning?dc=dc1")
			if err != nil {
				return err
			}
			if nodes == nil || len(nodes) != 0 {
				return fmt.Errorf("got nodes %#v want an empty list", nodes)
			}
			return nil
		})
	})

	httpTest(t, func(srv *HTTPServer) {
		retry.Fatal(t, func() error {
			nodes, err := getHealthChecks(srv, "/v1/health/state/passing?dc=dc1")
			if err != nil {
				return err
			}
			if got, want := len(nodes), 1; got != want {
				return fmt.Errorf("got %d nodes want %d", got, want)
			}
			return nil
		})
	})
}

func TestHealthChecksInState_NodeMetaFilter(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.1",
			NodeMeta:   map[string]string{"somekey": "somevalue"},
			Check: &structs.HealthCheck{
				Node:   "bar",
				Name:   "node check",
				Status: api.HealthCritical,
			},
		}
		var out struct{}
		if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}

		retry.Fatal(t, func() error {
			nodes, err := getHealthChecks(srv, "/v1/health/state/critical?node-meta=somekey:somevalue")
			if err != nil {
				return err
			}
			if got, want := len(nodes), 1; got != want {
				return fmt.Errorf("got %d nodes want %d", got, want)
			}
			return nil
		})
	})
}

func TestHealthChecksInState_DistanceSort(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	wait.ForLeader(t, srv.agent.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Node:   "bar",
			Name:   "node check",
			Status: api.HealthCritical,
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Node, args.Check.Node = "foo", "foo"
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err := http.NewRequest("GET", "/v1/health/state/critical?dc=dc1&near=foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthChecksInState(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)
	nodes := obj.(structs.HealthChecks)
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[0].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}

	// Send an update for the node and wait for it to get applied.
	arg := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      coordinate.NewCoordinate(coordinate.DefaultConfig()),
	}
	if err := srv.agent.RPC("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Retry until foo moves to the front of the line.
	// todo(fs): fix me
	retry.Fatal(t, func() error {
		resp = httptest.NewRecorder()
		obj, err = srv.HealthChecksInState(resp, req)
		if err != nil {
			return fmt.Errorf("err: %v", err)
		}
		assertIndex(t, resp)
		nodes = obj.(structs.HealthChecks)
		if len(nodes) != 2 {
			return fmt.Errorf("bad: %v", nodes)
		}
		if nodes[0].Node != "foo" {
			return fmt.Errorf("bad: %v", nodes)
		}
		if nodes[1].Node != "bar" {
			return fmt.Errorf("bad: %v", nodes)
		}
		return nil
	})
}

func TestHealthNodeChecks(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	wait.ForLeader(t, srv.agent.RPC, "dc1")

	req, err := http.NewRequest("GET", "/v1/health/node/nope?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthNodeChecks(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be a non-nil empty list
	nodes := obj.(structs.HealthChecks)
	if nodes == nil || len(nodes) != 0 {
		t.Fatalf("bad: %v", obj)
	}

	req, err = http.NewRequest("GET", fmt.Sprintf("/v1/health/node/%s?dc=dc1", srv.agent.config.NodeName), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = httptest.NewRecorder()
	obj, err = srv.HealthNodeChecks(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be 1 health check for the server
	nodes = obj.(structs.HealthChecks)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestHealthServiceChecks(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	wait.ForLeader(t, srv.agent.RPC, "dc1")

	req, err := http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthServiceChecks(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be a non-nil empty list
	nodes := obj.(structs.HealthChecks)
	if nodes == nil || len(nodes) != 0 {
		t.Fatalf("bad: %v", obj)
	}

	// Create a service check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       srv.agent.config.NodeName,
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Node:      srv.agent.config.NodeName,
			Name:      "consul check",
			ServiceID: "consul",
		},
	}

	var out struct{}
	if err = srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err = http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = httptest.NewRecorder()
	obj, err = srv.HealthServiceChecks(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be 1 health check for consul
	nodes = obj.(structs.HealthChecks)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestHealthServiceChecks_NodeMetaFilter(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	wait.ForLeader(t, srv.agent.RPC, "dc1")

	req, err := http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1&node-meta=somekey:somevalue", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthServiceChecks(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be a non-nil empty list
	nodes := obj.(structs.HealthChecks)
	if nodes == nil || len(nodes) != 0 {
		t.Fatalf("bad: %v", obj)
	}

	// Create a service check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       srv.agent.config.NodeName,
		Address:    "127.0.0.1",
		NodeMeta:   map[string]string{"somekey": "somevalue"},
		Check: &structs.HealthCheck{
			Node:      srv.agent.config.NodeName,
			Name:      "consul check",
			ServiceID: "consul",
		},
	}

	var out struct{}
	if err = srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err = http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1&node-meta=somekey:somevalue", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = httptest.NewRecorder()
	obj, err = srv.HealthServiceChecks(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be 1 health check for consul
	nodes = obj.(structs.HealthChecks)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestHealthServiceChecks_DistanceSort(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	wait.ForLeader(t, srv.agent.RPC, "dc1")

	// Create a service check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "test",
			Service: "test",
		},
		Check: &structs.HealthCheck{
			Node:      "bar",
			Name:      "test check",
			ServiceID: "test",
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Node, args.Check.Node = "foo", "foo"
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err := http.NewRequest("GET", "/v1/health/checks/test?dc=dc1&near=foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthServiceChecks(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)
	nodes := obj.(structs.HealthChecks)
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", obj)
	}
	if nodes[0].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}

	// Send an update for the node and wait for it to get applied.
	arg := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      coordinate.NewCoordinate(coordinate.DefaultConfig()),
	}
	if err := srv.agent.RPC("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Retry until foo has moved to the front of the line.
	// todo(fs): fix me
	retry.Fatal(t, func() error {
		resp = httptest.NewRecorder()
		obj, err = srv.HealthServiceChecks(resp, req)
		if err != nil {
			return fmt.Errorf("err: %v", err)
		}
		assertIndex(t, resp)
		nodes = obj.(structs.HealthChecks)
		if len(nodes) != 2 {
			return fmt.Errorf("bad: %v", obj)
		}
		if nodes[0].Node != "foo" {
			return fmt.Errorf("bad: %v", nodes)
		}
		if nodes[1].Node != "bar" {
			return fmt.Errorf("bad: %v", nodes)
		}
		return nil
	})
}

func TestHealthServiceNodes(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	wait.ForLeader(t, srv.agent.RPC, "dc1")

	req, err := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	// Should be 1 health check for consul
	nodes := obj.(structs.CheckServiceNodes)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}

	req, err = http.NewRequest("GET", "/v1/health/service/nope?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = httptest.NewRecorder()
	obj, err = srv.HealthServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	// Should be a non-nil empty list
	nodes = obj.(structs.CheckServiceNodes)
	if nodes == nil || len(nodes) != 0 {
		t.Fatalf("bad: %v", obj)
	}

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "test",
			Service: "test",
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err = http.NewRequest("GET", "/v1/health/service/test?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = httptest.NewRecorder()
	obj, err = srv.HealthServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	// Should be a non-nil empty list for checks
	nodes = obj.(structs.CheckServiceNodes)
	if len(nodes) != 1 || nodes[0].Checks == nil || len(nodes[0].Checks) != 0 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestHealthServiceNodes_NodeMetaFilter(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	wait.ForLeader(t, srv.agent.RPC, "dc1")

	req, err := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1&node-meta=somekey:somevalue", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	// Should be a non-nil empty list
	nodes := obj.(structs.CheckServiceNodes)
	if nodes == nil || len(nodes) != 0 {
		t.Fatalf("bad: %v", obj)
	}

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta:   map[string]string{"somekey": "somevalue"},
		Service: &structs.NodeService{
			ID:      "test",
			Service: "test",
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err = http.NewRequest("GET", "/v1/health/service/test?dc=dc1&node-meta=somekey:somevalue", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = httptest.NewRecorder()
	obj, err = srv.HealthServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	// Should be a non-nil empty list for checks
	nodes = obj.(structs.CheckServiceNodes)
	if len(nodes) != 1 || nodes[0].Checks == nil || len(nodes[0].Checks) != 0 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestHealthServiceNodes_DistanceSort(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	wait.ForLeader(t, srv.agent.RPC, "dc1")

	// Create a service check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "test",
			Service: "test",
		},
		Check: &structs.HealthCheck{
			Node:      "bar",
			Name:      "test check",
			ServiceID: "test",
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Node, args.Check.Node = "foo", "foo"
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err := http.NewRequest("GET", "/v1/health/service/test?dc=dc1&near=foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)
	nodes := obj.(structs.CheckServiceNodes)
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", obj)
	}
	if nodes[0].Node.Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Node.Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}

	// Send an update for the node and wait for it to get applied.
	arg := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      coordinate.NewCoordinate(coordinate.DefaultConfig()),
	}
	if err := srv.agent.RPC("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Retry until foo has moved to the front of the line.
	// todo(fs): fix me
	retry.Fatal(t, func() error {
		resp = httptest.NewRecorder()
		obj, err = srv.HealthServiceNodes(resp, req)
		if err != nil {
			return fmt.Errorf("err: %v", err)
		}
		assertIndex(t, resp)
		nodes = obj.(structs.CheckServiceNodes)
		if len(nodes) != 2 {
			return fmt.Errorf("bad: %v", obj)
		}
		if nodes[0].Node.Node != "foo" {
			return fmt.Errorf("bad: %v", nodes)
		}
		if nodes[1].Node.Node != "bar" {
			return fmt.Errorf("bad: %v", nodes)
		}
		return nil
	})
}

func TestHealthServiceNodes_PassingFilter(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	wait.ForLeader(t, srv.agent.RPC, "dc1")

	// Create a failing service check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       srv.agent.config.NodeName,
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Node:      srv.agent.config.NodeName,
			Name:      "consul check",
			ServiceID: "consul",
			Status:    api.HealthCritical,
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err := http.NewRequest("GET", "/v1/health/service/consul?passing", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	// Should be 0 health check for consul
	nodes := obj.(structs.CheckServiceNodes)
	if len(nodes) != 0 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestHealthServiceNodes_WanTranslation(t *testing.T) {
	testWanTranslation(t, func(t *testing.T, srv1, srv2 *HTTPServer) {
		// Query for a service in DC2 from DC1.
		req, err := http.NewRequest("GET", "/v1/health/service/http_wan_translation_test?dc=dc2", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		resp1 := httptest.NewRecorder()
		obj1, err1 := srv1.HealthServiceNodes(resp1, req)
		if err1 != nil {
			t.Fatalf("err: %v", err1)
		}
		assertIndex(t, resp1)

		// Expect that DC1 gives us a WAN address (since the node is in DC2).
		nodes1 := obj1.(structs.CheckServiceNodes)
		if len(nodes1) != 1 {
			t.Fatalf("bad: %v", obj1)
		}
		node1 := nodes1[0].Node
		if node1.Address != "127.0.0.2" {
			t.Fatalf("bad: %v", node1)
		}

		// Query DC2 from DC2.
		resp2 := httptest.NewRecorder()
		obj2, err2 := srv2.HealthServiceNodes(resp2, req)
		if err2 != nil {
			t.Fatalf("err: %v", err2)
		}
		assertIndex(t, resp2)

		// Expect that DC2 gives us a private address (since the node is in DC2).
		nodes2 := obj2.(structs.CheckServiceNodes)
		if len(nodes2) != 1 {
			t.Fatalf("bad: %v", obj2)
		}
		node2 := nodes2[0].Node
		if node2.Address != "127.0.0.1" {
			t.Fatalf("bad: %v", node2)
		}
	})
}

func TestFilterNonPassing(t *testing.T) {
	nodes := structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Status: api.HealthCritical,
				},
				&structs.HealthCheck{
					Status: api.HealthCritical,
				},
			},
		},
		structs.CheckServiceNode{
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Status: api.HealthCritical,
				},
				&structs.HealthCheck{
					Status: api.HealthCritical,
				},
			},
		},
		structs.CheckServiceNode{
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Status: api.HealthPassing,
				},
			},
		},
	}
	out := filterNonPassing(nodes)
	if len(out) != 1 && reflect.DeepEqual(out[0], nodes[2]) {
		t.Fatalf("bad: %v", out)
	}
}

func getHealthChecks(srv *HTTPServer, urlstr string) (structs.HealthChecks, error) {
	req, err := http.NewRequest("GET", urlstr, nil)
	if err != nil {
		return nil, err
	}
	resp := httptest.NewRecorder()
	obj, err := srv.HealthChecksInState(resp, req)
	if err != nil {
		return nil, fmt.Errorf("HealthChecksInState failed: %s", err)
	}
	list, ok := obj.(structs.HealthChecks)
	if list != nil && !ok {
		return nil, fmt.Errorf("result is not structs.HealthChecks")
	}
	header := resp.Header().Get("X-Consul-Index")
	if header == "" || header == "0" {
		return nil, fmt.Errorf(`X-Consul-Index header %q must not be "" or "0"`, header)
	}
	return list, nil
}
