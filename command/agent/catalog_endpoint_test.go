package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/serf/coordinate"
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

func TestCatalogNodes_WanTranslation(t *testing.T) {
	httpCtx1, httpCtx2 := setupWanHTTPServers(t)
	defer shutdownHTTPServer(httpCtx1)
	defer shutdownHTTPServer(httpCtx2)
	srv1 := httpCtx1.srv
	srv2 := httpCtx2.srv

	// Register a node with DC2
	{
		args := &structs.RegisterRequest{
			Datacenter: "dc2",
			Node:       "wan_translation_test",
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

	req, err := http.NewRequest("GET", "/v1/catalog/nodes?dc=dc2", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// get nodes for DC2 from DC1
	resp1 := httptest.NewRecorder()
	obj1, err1 := srv1.CatalogNodes(resp1, req)
	if err1 != nil {
		t.Fatalf("err: %v", err1)
	}

	// Verify an index is set
	assertIndex(t, resp1)

	nodes1 := obj1.(structs.Nodes)
	if len(nodes1) != 2 {
		t.Fatalf("bad: %v", obj1)
	}

	var node1 *structs.Node

	for _, node := range nodes1 {
		if node.Node == "wan_translation_test" {
			node1 = node
		}
	}

	// Expect that DC1 gives us a public address (since the node is in DC2)
	if node1.Address != "127.0.0.2" {
		t.Fatalf("bad: %v", node1)
	}

	// get nodes for DC2 from DC2
	resp2 := httptest.NewRecorder()
	obj2, err2 := srv2.CatalogNodes(resp2, req)
	if err2 != nil {
		t.Fatalf("err: %v", err2)
	}

	// Verify an index is set
	assertIndex(t, resp2)

	nodes2 := obj2.(structs.Nodes)
	if len(nodes2) != 2 {
		t.Fatalf("bad: %v", obj2)
	}

	var node2 *structs.Node

	for _, node := range nodes2 {
		if node.Node == "wan_translation_test" {
			node2 = node
		}
	}

	// Expect that DC2 gives us a private address (since the node is in DC2)
	if node2.Address != "127.0.0.1" {
		t.Fatalf("bad: %v", node2)
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

func TestCatalogNodes_DistanceSort(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register nodes.
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	var out struct{}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
	}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nobody has coordinates set so this will still return them in the
	// order they are indexed.
	req, err := http.NewRequest("GET", "/v1/catalog/nodes?dc=dc1&near=foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := httptest.NewRecorder()
	obj, err := srv.CatalogNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)
	nodes := obj.(structs.Nodes)
	if len(nodes) != 3 {
		t.Fatalf("bad: %v", obj)
	}
	if nodes[0].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].Node != srv.agent.config.NodeName {
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
	time.Sleep(300 * time.Millisecond)

	// Query again and now foo should have moved to the front of the line.
	req, err = http.NewRequest("GET", "/v1/catalog/nodes?dc=dc1&near=foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = httptest.NewRecorder()
	obj, err = srv.CatalogNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)
	nodes = obj.(structs.Nodes)
	if len(nodes) != 3 {
		t.Fatalf("bad: %v", obj)
	}
	if nodes[0].Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[2].Node != srv.agent.config.NodeName {
		t.Fatalf("bad: %v", nodes)
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

	// Make sure an empty list is returned, not a nil
	{
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
		if nodes == nil || len(nodes) != 0 {
			t.Fatalf("bad: %v", obj)
		}
	}

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

func TestCatalogServiceNodes_WanTranslation(t *testing.T) {
	httpCtx1, httpCtx2 := setupWanHTTPServers(t)
	defer shutdownHTTPServer(httpCtx1)
	defer shutdownHTTPServer(httpCtx2)
	srv1 := httpCtx1.srv
	srv2 := httpCtx2.srv

	// Register a node with DC2
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

	req, err := http.NewRequest("GET", "/v1/catalog/service/http_wan_translation_test?dc=dc2", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ask HTTP server on DC1 for the node
	resp1 := httptest.NewRecorder()
	obj1, err1 := srv1.CatalogServiceNodes(resp1, req)
	if err1 != nil {
		t.Fatalf("err: %v", err1)
	}

	assertIndex(t, resp1)

	nodes1 := obj1.(structs.ServiceNodes)
	if len(nodes1) != 1 {
		t.Fatalf("bad: %v", obj1)
	}

	node1 := nodes1[0]

	// Expect that DC1 gives us a public address (since the node is in DC2)
	if node1.Address != "127.0.0.2" {
		t.Fatalf("bad: %v", node1)
	}

	// Ask HTTP server on DC2 for the node
	resp2 := httptest.NewRecorder()
	obj2, err2 := srv2.CatalogServiceNodes(resp2, req)
	if err2 != nil {
		t.Fatalf("err: %v", err2)
	}

	assertIndex(t, resp2)

	nodes2 := obj2.(structs.ServiceNodes)
	if len(nodes2) != 1 {
		t.Fatalf("bad: %v", obj2)
	}

	node2 := nodes2[0]

	// Expect that DC2 gives us a local address (since the node is in DC2)
	if node2.Address != "127.0.0.1" {
		t.Fatalf("bad: %v", node2)
	}
}

func TestCatalogServiceNodes_DistanceSort(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	testutil.WaitForLeader(t, srv.agent.RPC, "dc1")

	// Register nodes.
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
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

	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			Service: "api",
			Tags:    []string{"a"},
		},
	}
	if err := srv.agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nobody has coordinates set so this will still return them in the
	// order they are indexed.
	req, err = http.NewRequest("GET", "/v1/catalog/service/api?tag=a&near=foo", nil)
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
	time.Sleep(300 * time.Millisecond)

	// Query again and now foo should have moved to the front of the line.
	req, err = http.NewRequest("GET", "/v1/catalog/service/api?tag=a&near=foo", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = httptest.NewRecorder()
	obj, err = srv.CatalogServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)
	nodes = obj.(structs.ServiceNodes)
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", obj)
	}
	if nodes[0].Node != "foo" {
		t.Fatalf("bad: %v", nodes)
	}
	if nodes[1].Node != "bar" {
		t.Fatalf("bad: %v", nodes)
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

func TestCatalogNodeServices_WanTranslation(t *testing.T) {
	httpCtx1, httpCtx2 := setupWanHTTPServers(t)
	defer shutdownHTTPServer(httpCtx1)
	defer shutdownHTTPServer(httpCtx2)
	srv1 := httpCtx1.srv
	srv2 := httpCtx2.srv

	// Register a node with DC2
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

	req, err := http.NewRequest("GET", "/v1/catalog/node/foo?dc=dc2", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// ask DC1 for node in DC2
	resp1 := httptest.NewRecorder()
	obj1, err1 := srv1.CatalogNodeServices(resp1, req)
	if err1 != nil {
		t.Fatalf("err: %v", err1)
	}
	assertIndex(t, resp1)

	services1 := obj1.(*structs.NodeServices)
	if len(services1.Services) != 1 {
		t.Fatalf("bad: %v", obj1)
	}

	service1 := services1.Node

	// Expect that DC1 gives us a public address (since the node is in DC2)
	if service1.Address != "127.0.0.2" {
		t.Fatalf("bad: %v", service1)
	}

	// ask DC2 for node in DC2
	resp2 := httptest.NewRecorder()
	obj2, err2 := srv2.CatalogNodeServices(resp2, req)
	if err2 != nil {
		t.Fatalf("err: %v", err2)
	}
	assertIndex(t, resp2)

	services2 := obj2.(*structs.NodeServices)
	if len(services2.Services) != 1 {
		t.Fatalf("bad: %v", obj2)
	}

	service2 := services2.Node

	// Expect that DC2 gives us a private address (since the node is in DC2)
	if service2.Address != "127.0.0.1" {
		t.Fatalf("bad: %v", service2)
	}
}
