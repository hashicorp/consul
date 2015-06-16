package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
)

func TestHealthChecksInState(t *testing.T) {
	httpTest(t, func(srv *HTTPServer) {
		req, err := http.NewRequest("GET", "/v1/health/state/passing?dc=dc1", nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		testutil.WaitForResult(func() (bool, error) {
			resp := httptest.NewRecorder()
			obj, err := srv.HealthChecksInState(resp, req)
			if err != nil {
				return false, err
			}
			if err := checkIndex(resp); err != nil {
				return false, err
			}

			// Should be 1 health check for the server
			nodes := obj.(structs.HealthChecks)
			if len(nodes) != 1 {
				return false, fmt.Errorf("bad: %v", obj)
			}
			return true, nil
		}, func(err error) { t.Fatalf("err: %v", err) })
	})
}

func TestHealthNodeChecks(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	req, err := http.NewRequest("GET",
		fmt.Sprintf("/v1/health/node/%s?dc=dc1", srv.agent.config.NodeName), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.HealthNodeChecks(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	// Should be 1 health check for the server
	nodes := obj.(structs.HealthChecks)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestHealthServiceChecks(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

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
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

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

	// Should be 1 health check for consul
	nodes := obj.(structs.HealthChecks)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestHealthServiceNodes(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

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
}

func TestHealthServiceNodes_PassingFilter(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Create a failing service check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       srv.agent.config.NodeName,
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Node:      srv.agent.config.NodeName,
			Name:      "consul check",
			ServiceID: "consul",
			Status:    structs.HealthCritical,
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

func TestFilterNonPassing(t *testing.T) {
	nodes := structs.CheckServiceNodes{
		structs.CheckServiceNode{
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Status: structs.HealthCritical,
				},
				&structs.HealthCheck{
					Status: structs.HealthCritical,
				},
			},
		},
		structs.CheckServiceNode{
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Status: structs.HealthCritical,
				},
				&structs.HealthCheck{
					Status: structs.HealthCritical,
				},
			},
		},
		structs.CheckServiceNode{
			Checks: structs.HealthChecks{
				&structs.HealthCheck{
					Status: structs.HealthPassing,
				},
			},
		},
	}
	out := filterNonPassing(nodes)
	if len(out) != 1 && reflect.DeepEqual(out[0], nodes[2]) {
		t.Fatalf("bad: %v", out)
	}
}
