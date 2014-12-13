package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestCatalogRegister(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	req, err := http.NewRequest("GET", "/v1/catalog/register", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	args := &structs.RegisterRequest{
		Node:    "foo",
		Address: "127.0.0.1",
	}
	req.Body = encodeReq(args)

	obj, err := srv.CatalogRegister(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	res := obj.(bool)
	if res != true {
		t.Fatalf("bad: %v", res)
	}

	// Service should be in sync
	if err := srv.agent.state.syncService("foo"); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, ok := srv.agent.state.serviceStatus["foo"]; !ok {
		t.Fatalf("bad: %#v", srv.agent.state.serviceStatus)
	}
	if !srv.agent.state.serviceStatus["foo"].inSync {
		t.Fatalf("should be in sync")
	}
}

func TestCatalogDeregister(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	req, err := http.NewRequest("GET", "/v1/catalog/deregister", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	args := &structs.DeregisterRequest{
		Node: "foo",
	}
	req.Body = encodeReq(args)

	obj, err := srv.CatalogDeregister(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	res := obj.(bool)
	if res != true {
		t.Fatalf("bad: %v", res)
	}
}

func TestCatalogDatacenters(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForResult(func() (bool, error) {
		obj, err := srv.CatalogDatacenters(nil, nil)
		if err != nil {
			return false, err
		}

		dcs := obj.([]string)
		if len(dcs) != 1 {
			return false, fmt.Errorf("missing dc: %v", dcs)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf("bad: %v", err)
	})
}

func TestCatalogNodes(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err := http.NewRequest("GET", "/v1/catalog/nodes?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.CatalogNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify an index is set
	assertIndex(t, resp)

	nodes := obj.(structs.Nodes)
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestCatalogNodes_Blocking(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}

	var out structs.IndexedNodes
	if err := srv.agent.RPC("Catalog.ListNodes", *args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Do an update in a little while
	start := time.Now()
	go func() {
		time.Sleep(50 * time.Millisecond)
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
		}
		var out struct{}
		if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}()

	// Do a blocking read
	req, err := http.NewRequest("GET",
		fmt.Sprintf("/v1/catalog/nodes?wait=60s&index=%d", out.Index), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.CatalogNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should block for a while
	if time.Now().Sub(start) < 50*time.Millisecond {
		t.Fatalf("too fast")
	}

	if idx := getIndex(t, resp); idx <= out.Index {
		t.Fatalf("bad: %v", idx)
	}

	nodes := obj.(structs.Nodes)
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestCatalogServices(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err := http.NewRequest("GET", "/v1/catalog/services?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.CatalogServices(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	services := obj.(structs.Services)
	if len(services) != 2 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestCatalogServiceNodes(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
			Tags:    []string{"a"},
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err := http.NewRequest("GET", "/v1/catalog/service/api?tag=a", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.CatalogServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	nodes := obj.(structs.ServiceNodes)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestCatalogNodeServices(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
			Tags:    []string{"a"},
		},
	}

	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, err := http.NewRequest("GET", "/v1/catalog/node/foo?dc=dc1", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.CatalogNodeServices(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	services := obj.(*structs.NodeServices)
	if len(services.Services) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}
