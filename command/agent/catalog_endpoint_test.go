package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestCatalogRegister(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

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
}

func TestCatalogDeregister(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

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

	// Wait for initialization
	time.Sleep(10 * time.Millisecond)

	obj, err := srv.CatalogDatacenters(nil, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	dcs := obj.([]string)
	if len(dcs) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestCatalogNodes(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

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

	idx, obj, err := srv.CatalogNodes(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if idx == 0 {
		t.Fatalf("bad: %v", idx)
	}

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

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

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
		time.Sleep(100 * time.Millisecond)
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

	idx, obj, err := srv.CatalogNodes(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should block for a while
	if time.Now().Sub(start) < 100*time.Millisecond {
		t.Fatalf("too fast")
	}

	if idx <= out.Index {
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

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

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

	idx, obj, err := srv.CatalogServices(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if idx == 0 {
		t.Fatalf("bad: %v", idx)
	}

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

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
			Tag:     "a",
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

	idx, obj, err := srv.CatalogServiceNodes(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if idx == 0 {
		t.Fatalf("bad: %v", idx)
	}

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

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
			Tag:     "a",
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

	idx, obj, err := srv.CatalogNodeServices(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if idx == 0 {
		t.Fatalf("bad: %v", idx)
	}

	services := obj.(*structs.NodeServices)
	if len(services.Services) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}
