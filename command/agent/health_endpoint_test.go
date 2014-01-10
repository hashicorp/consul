package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestHealthChecksInState(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest("GET", "/v1/health/state/passing?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.HealthChecksInState(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be 1 health check for the server
	nodes := obj.(structs.HealthChecks)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestHealthNodeChecks(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest("GET",
		fmt.Sprintf("/v1/health/node/%s?dc=dc1", srv.agent.config.NodeName), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.HealthNodeChecks(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

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

	obj, err := srv.HealthServiceChecks(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

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

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.HealthServiceNodes(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should be 1 health check for consul
	nodes := obj.(structs.CheckServiceNodes)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}
