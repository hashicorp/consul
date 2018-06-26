package agent

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/assert"
)

func TestHealthChecksInState(t *testing.T) {
	t.Parallel()
	t.Run("warning", func(t *testing.T) {
		a := NewTestAgent(t.Name(), "")
		defer a.Shutdown()

		req, _ := http.NewRequest("GET", "/v1/health/state/warning?dc=dc1", nil)
		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			obj, err := a.srv.HealthChecksInState(resp, req)
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

	t.Run("passing", func(t *testing.T) {
		a := NewTestAgent(t.Name(), "")
		defer a.Shutdown()

		req, _ := http.NewRequest("GET", "/v1/health/state/passing?dc=dc1", nil)
		retry.Run(t, func(r *retry.R) {
			resp := httptest.NewRecorder()
			obj, err := a.srv.HealthChecksInState(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/health/state/critical?node-meta=somekey:somevalue", nil)
	retry.Run(t, func(r *retry.R) {
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthChecksInState(resp, req)
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
}

func TestHealthChecksInState_DistanceSort(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Node, args.Check.Node = "foo", "foo"
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/health/state/critical?dc=dc1&near=foo", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthChecksInState(resp, req)
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
	if err := a.RPC("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Retry until foo moves to the front of the line.
	retry.Run(t, func(r *retry.R) {
		resp = httptest.NewRecorder()
		obj, err = a.srv.HealthChecksInState(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/health/node/nope?dc=dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthNodeChecks(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be a non-nil empty list
	nodes := obj.(structs.HealthChecks)
	if nodes == nil || len(nodes) != 0 {
		t.Fatalf("bad: %v", obj)
	}

	req, _ = http.NewRequest("GET", fmt.Sprintf("/v1/health/node/%s?dc=dc1", a.Config.NodeName), nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.HealthNodeChecks(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceChecks(resp, req)
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
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Node:      a.Config.NodeName,
			Name:      "consul check",
			ServiceID: "consul",
		},
	}

	var out struct{}
	if err = a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ = http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.HealthServiceChecks(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1&node-meta=somekey:somevalue", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceChecks(resp, req)
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
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		NodeMeta:   map[string]string{"somekey": "somevalue"},
		Check: &structs.HealthCheck{
			Node:      a.Config.NodeName,
			Name:      "consul check",
			ServiceID: "consul",
		},
	}

	var out struct{}
	if err = a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ = http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1&node-meta=somekey:somevalue", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.HealthServiceChecks(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Node, args.Check.Node = "foo", "foo"
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/health/checks/test?dc=dc1&near=foo", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceChecks(resp, req)
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
	if err := a.RPC("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Retry until foo has moved to the front of the line.
	retry.Run(t, func(r *retry.R) {
		resp = httptest.NewRecorder()
		obj, err = a.srv.HealthServiceChecks(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceNodes(resp, req)
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
	obj, err = a.srv.HealthServiceNodes(resp, req)
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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ = http.NewRequest("GET", "/v1/health/service/test?dc=dc1", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.HealthServiceNodes(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	req, _ := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1&node-meta=somekey:somevalue", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceNodes(resp, req)
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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ = http.NewRequest("GET", "/v1/health/service/test?dc=dc1&node-meta=somekey:somevalue", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.HealthServiceNodes(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Node, args.Check.Node = "foo", "foo"
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/health/service/test?dc=dc1&near=foo", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceNodes(resp, req)
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
	if err := a.RPC("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	// Retry until foo has moved to the front of the line.
	retry.Run(t, func(r *retry.R) {
		resp = httptest.NewRecorder()
		obj, err = a.srv.HealthServiceNodes(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Create a failing service check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Node:      a.Config.NodeName,
			Name:      "consul check",
			ServiceID: "consul",
			Status:    api.HealthCritical,
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Run("bc_no_query_value", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/health/service/consul?passing", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthServiceNodes(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		assertIndex(t, resp)

		// Should be 0 health check for consul
		nodes := obj.(structs.CheckServiceNodes)
		if len(nodes) != 0 {
			t.Fatalf("bad: %v", obj)
		}
	})

	t.Run("passing_true", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/health/service/consul?passing=true", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthServiceNodes(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		assertIndex(t, resp)

		// Should be 0 health check for consul
		nodes := obj.(structs.CheckServiceNodes)
		if len(nodes) != 0 {
			t.Fatalf("bad: %v", obj)
		}
	})

	t.Run("passing_false", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/health/service/consul?passing=false", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthServiceNodes(resp, req)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		assertIndex(t, resp)

		// Should be 1 consul, it's unhealthy, but we specifically asked for
		// everything.
		nodes := obj.(structs.CheckServiceNodes)
		if len(nodes) != 1 {
			t.Fatalf("bad: %v", obj)
		}
	})

	t.Run("passing_bad", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/health/service/consul?passing=nope-nope-nope", nil)
		resp := httptest.NewRecorder()
		a.srv.HealthServiceNodes(resp, req)

		if code := resp.Code; code != 400 {
			t.Errorf("bad response code %d, expected %d", code, 400)
		}

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte("Invalid value for ?passing")) {
			t.Errorf("bad %s", body)
		}
	})
}

func TestHealthServiceNodes_WanTranslation(t *testing.T) {
	t.Parallel()
	a1 := NewTestAgent(t.Name(), `
		datacenter = "dc1"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a1.Shutdown()

	a2 := NewTestAgent(t.Name(), `
		datacenter = "dc2"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a2.Shutdown()

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	if _, err := a2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	retry.Run(t, func(r *retry.R) {
		if got, want := len(a1.WANMembers()), 2; got < want {
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
		if err := a2.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Query for a service in DC2 from DC1.
	req, _ := http.NewRequest("GET", "/v1/health/service/http_wan_translation_test?dc=dc2", nil)
	resp1 := httptest.NewRecorder()
	obj1, err1 := a1.srv.HealthServiceNodes(resp1, req)
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
	obj2, err2 := a2.srv.HealthServiceNodes(resp2, req)
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

func TestHealthConnectServiceNodes(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Register
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(a.RPC("Catalog.Register", args, &out))

	// Request
	req, _ := http.NewRequest("GET", fmt.Sprintf(
		"/v1/health/connect/%s?dc=dc1", args.Service.ProxyDestination), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthConnectServiceNodes(resp, req)
	assert.Nil(err)
	assertIndex(t, resp)

	// Should be a non-nil empty list for checks
	nodes := obj.(structs.CheckServiceNodes)
	assert.Len(nodes, 1)
	assert.Len(nodes[0].Checks, 0)
}

func TestHealthConnectServiceNodes_PassingFilter(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	// Register
	args := structs.TestRegisterRequestProxy(t)
	args.Check = &structs.HealthCheck{
		Node:      args.Node,
		Name:      "check",
		ServiceID: args.Service.Service,
		Status:    api.HealthCritical,
	}
	var out struct{}
	assert.Nil(t, a.RPC("Catalog.Register", args, &out))

	t.Run("bc_no_query_value", func(t *testing.T) {
		assert := assert.New(t)
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/connect/%s?passing", args.Service.ProxyDestination), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthConnectServiceNodes(resp, req)
		assert.Nil(err)
		assertIndex(t, resp)

		// Should be 0 health check for consul
		nodes := obj.(structs.CheckServiceNodes)
		assert.Len(nodes, 0)
	})

	t.Run("passing_true", func(t *testing.T) {
		assert := assert.New(t)
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/connect/%s?passing=true", args.Service.ProxyDestination), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthConnectServiceNodes(resp, req)
		assert.Nil(err)
		assertIndex(t, resp)

		// Should be 0 health check for consul
		nodes := obj.(structs.CheckServiceNodes)
		assert.Len(nodes, 0)
	})

	t.Run("passing_false", func(t *testing.T) {
		assert := assert.New(t)
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/connect/%s?passing=false", args.Service.ProxyDestination), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthConnectServiceNodes(resp, req)
		assert.Nil(err)
		assertIndex(t, resp)

		// Should be 1
		nodes := obj.(structs.CheckServiceNodes)
		assert.Len(nodes, 1)
	})

	t.Run("passing_bad", func(t *testing.T) {
		assert := assert.New(t)
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/connect/%s?passing=nope-nope", args.Service.ProxyDestination), nil)
		resp := httptest.NewRecorder()
		a.srv.HealthConnectServiceNodes(resp, req)
		assert.Equal(400, resp.Code)

		body, err := ioutil.ReadAll(resp.Body)
		assert.Nil(err)
		assert.True(bytes.Contains(body, []byte("Invalid value for ?passing")))
	})
}

func TestFilterNonPassing(t *testing.T) {
	t.Parallel()
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
