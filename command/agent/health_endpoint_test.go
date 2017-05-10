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
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/serf/coordinate"
)

func TestHealthChecksInState(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		req, _ := http.NewRequest("GET", "/v1/health/state/warning?dc=dc1", nil)
		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			obj, err := srv.HealthChecksInState(resp, req)
			if err != nil {
				r.Fatal(err)
			}
			if err := checkIndex(resp); err != nil {
				r.Fatal(err)
			}

			// Should be a non-nil empty list
			nodes := obj.(structs.HealthChecks)
			if nodes == nil || len(nodes) != 0 {
				r.Fatalf("bad: %v", obj)
			}
		})
	})

	httpTest(t, func(srv *HTTPServer) {
		req, _ := http.NewRequest("GET", "/v1/health/state/passing?dc=dc1", nil)
		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			obj, err := srv.HealthChecksInState(resp, req)
			if err != nil {
				r.Fatal(err)
			}
			if err := checkIndex(resp); err != nil {
				r.Fatal(err)
			}

			// Should be 1 health check for the server
			nodes := obj.(structs.HealthChecks)
			if len(nodes) != 1 {
				r.Fatalf("bad: %v", obj)
			}
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

		req, _ := http.NewRequest("GET", "/v1/health/state/critical?node-meta=somekey:somevalue", nil)
		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			obj, err := srv.HealthChecksInState(resp, req)
			if err != nil {
				r.Fatal(err)
			}
			if err := checkIndex(resp); err != nil {
				r.Fatal(err)
			}

			// Should be 1 health check for the server
			nodes := obj.(structs.HealthChecks)
			if len(nodes) != 1 {
				r.Fatalf("bad: %v", obj)
			}
		})
	})
}

func TestHealthChecksInState_DistanceSort(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testrpc.WaitForLeader(t, srv.agent.RPC, "dc1")

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

	req, _ := http.NewRequest("GET", "/v1/health/state/critical?dc=dc1&near=foo", nil)
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
	retry.Run(t, func(r *retry.R) {
		resp = httptest.NewRecorder()
		obj, err = srv.HealthChecksInState(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		assertIndex(t, resp)
		nodes = obj.(structs.HealthChecks)
		if len(nodes) != 2 {
			r.Fatalf("bad: %v", nodes)
		}
		if nodes[0].Node != "foo" {
			r.Fatalf("bad: %v", nodes)
		}
		if nodes[1].Node != "bar" {
			r.Fatalf("bad: %v", nodes)
		}
	})
}

func TestHealthNodeChecks(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testrpc.WaitForLeader(t, srv.agent.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/node/nope?dc=dc1", nil)
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

	req, _ = http.NewRequest("GET", fmt.Sprintf("/v1/health/node/%s?dc=dc1", srv.agent.config.NodeName), nil)
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

	testrpc.WaitForLeader(t, srv.agent.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1", nil)
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

	req, _ = http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1", nil)
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

	testrpc.WaitForLeader(t, srv.agent.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1&node-meta=somekey:somevalue", nil)
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

	req, _ = http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1&node-meta=somekey:somevalue", nil)
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

	testrpc.WaitForLeader(t, srv.agent.RPC, "dc1")

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

	req, _ := http.NewRequest("GET", "/v1/health/checks/test?dc=dc1&near=foo", nil)
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
	retry.Run(t, func(r *retry.R) {
		resp = httptest.NewRecorder()
		obj, err = srv.HealthServiceChecks(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		assertIndex(t, resp)
		nodes = obj.(structs.HealthChecks)
		if len(nodes) != 2 {
			r.Fatalf("bad: %v", obj)
		}
		if nodes[0].Node != "foo" {
			r.Fatalf("bad: %v", nodes)
		}
		if nodes[1].Node != "bar" {
			r.Fatalf("bad: %v", nodes)
		}
	})
}

func TestHealthServiceNodes(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testrpc.WaitForLeader(t, srv.agent.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1", nil)
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

	req, _ = http.NewRequest("GET", "/v1/health/service/nope?dc=dc1", nil)
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

	req, _ = http.NewRequest("GET", "/v1/health/service/test?dc=dc1", nil)
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

	testrpc.WaitForLeader(t, srv.agent.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1&node-meta=somekey:somevalue", nil)
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

	req, _ = http.NewRequest("GET", "/v1/health/service/test?dc=dc1&node-meta=somekey:somevalue", nil)
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

	testrpc.WaitForLeader(t, srv.agent.RPC, "dc1")

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

	req, _ := http.NewRequest("GET", "/v1/health/service/test?dc=dc1&near=foo", nil)
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
	retry.Run(t, func(r *retry.R) {
		resp = httptest.NewRecorder()
		obj, err = srv.HealthServiceNodes(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		assertIndex(t, resp)
		nodes = obj.(structs.CheckServiceNodes)
		if len(nodes) != 2 {
			r.Fatalf("bad: %v", obj)
		}
		if nodes[0].Node.Node != "foo" {
			r.Fatalf("bad: %v", nodes)
		}
		if nodes[1].Node.Node != "bar" {
			r.Fatalf("bad: %v", nodes)
		}
	})
}

func TestHealthServiceNodes_PassingFilter(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testrpc.WaitForLeader(t, srv.agent.RPC, "dc1")

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

	req, _ := http.NewRequest("GET", "/v1/health/service/consul?passing", nil)
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
	dir1, srv1 := makeHTTPServerWithConfig(t,
		func(c *Config) {
			c.Datacenter = "dc1"
			c.TranslateWanAddrs = true
			c.ACLDatacenter = ""
		})
	defer os.RemoveAll(dir1)
	defer srv1.Shutdown()
	defer srv1.agent.Shutdown()
	testrpc.WaitForLeader(t, srv1.agent.RPC, "dc1")

	dir2, srv2 := makeHTTPServerWithConfig(t,
		func(c *Config) {
			c.Datacenter = "dc2"
			c.TranslateWanAddrs = true
			c.ACLDatacenter = ""
		})
	defer os.RemoveAll(dir2)
	defer srv2.Shutdown()
	defer srv2.agent.Shutdown()
	testrpc.WaitForLeader(t, srv2.agent.RPC, "dc2")

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", srv1.agent.config.Ports.SerfWan)
	if _, err := srv2.agent.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	retry.Run(t, func(r *retry.R) {
		if got, want := len(srv1.agent.WANMembers()), 2; got < want {
			r.Fatalf("got %d WAN members want at least %d", got, want)
		}
	})

	// Register a node with DC2.
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc2",
			Node:       "foo",
			Address:    "127.0.0.1",
			TaggedAddresses: map[string]string{
				"wan": "127.0.0.2",
			},
			Service: &structs.NodeService{
				Service: "http_wan_translation_test",
			},
		}

		var out struct{}
		if err := srv2.agent.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Query for a service in DC2 from DC1.
	req, _ := http.NewRequest("GET", "/v1/health/service/http_wan_translation_test?dc=dc2", nil)
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
