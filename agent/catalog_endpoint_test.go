package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/serf/coordinate"
)

func TestCatalogRegister(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register node
	args := &structs.RegisterRequest{
		Node:    "foo",
		Address: "127.0.0.1",
	}
	req, _ := http.NewRequest("GET", "/v1/catalog/register", jsonReader(args))
	obj, err := a.srv.CatalogRegister(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	res := obj.(bool)
	if res != true {
		t.Fatalf("bad: %v", res)
	}

	// Service should be in sync
	if err := a.state.syncService("foo"); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, ok := a.state.serviceStatus["foo"]; !ok {
		t.Fatalf("bad: %#v", a.state.serviceStatus)
	}
	if !a.state.serviceStatus["foo"].inSync {
		t.Fatalf("should be in sync")
	}
}

func TestCatalogRegister_Service_InvalidAddress(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	for _, addr := range []string{"0.0.0.0", "::", "[::]"} {
		t.Run("addr "+addr, func(t *testing.T) {
			args := &structs.RegisterRequest{
				Node:    "foo",
				Address: "127.0.0.1",
				Service: &structs.NodeService{
					Service: "test",
					Address: addr,
					Port:    8080,
				},
			}
			req, _ := http.NewRequest("GET", "/v1/catalog/register", jsonReader(args))
			_, err := a.srv.CatalogRegister(nil, req)
			if err == nil || err.Error() != "Invalid service address" {
				t.Fatalf("err: %v", err)
			}
		})
	}
}

func TestCatalogDeregister(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register node
	args := &structs.DeregisterRequest{Node: "foo"}
	req, _ := http.NewRequest("GET", "/v1/catalog/deregister", jsonReader(args))
	obj, err := a.srv.CatalogDeregister(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	res := obj.(bool)
	if res != true {
		t.Fatalf("bad: %v", res)
	}
}

func TestCatalogDatacenters(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	retry.Run(t, func(r *retry.R) {
		obj, err := a.srv.CatalogDatacenters(nil, nil)
		if err != nil {
			r.Fatal(err)
		}

		dcs := obj.([]string)
		if got, want := len(dcs), 1; got != want {
			r.Fatalf("got %d data centers want %d", got, want)
		}
	})
}

func TestCatalogNodes(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?dc=dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodes(resp, req)
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

func TestCatalogNodes_MetaFilter(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register a node with a meta field
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"somekey": "somevalue",
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?node-meta=somekey:somevalue", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify an index is set
	assertIndex(t, resp)

	// Verify we only get the node with the correct meta field back
	nodes := obj.(structs.Nodes)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
	if v, ok := nodes[0].Meta["somekey"]; !ok || v != "somevalue" {
		t.Fatalf("bad: %v", nodes[0].Meta)
	}
}

func TestCatalogNodes_WanTranslation(t *testing.T) {
	t.Parallel()
	cfg1 := TestConfig()
	cfg1.Datacenter = "dc1"
	cfg1.TranslateWanAddrs = true
	cfg1.ACLDatacenter = ""
	a1 := NewTestAgent(t.Name(), cfg1)
	defer a1.Shutdown()

	cfg2 := TestConfig()
	cfg2.Datacenter = "dc2"
	cfg2.TranslateWanAddrs = true
	cfg2.ACLDatacenter = ""
	a2 := NewTestAgent(t.Name(), cfg2)
	defer a2.Shutdown()

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.Ports.SerfWan)
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
		if err := a2.RPC("Catalog.Register", args, &out); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// Query nodes in DC2 from DC1.
	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?dc=dc2", nil)
	resp1 := httptest.NewRecorder()
	obj1, err1 := a1.srv.CatalogNodes(resp1, req)
	if err1 != nil {
		t.Fatalf("err: %v", err1)
	}
	assertIndex(t, resp1)

	// Expect that DC1 gives us a WAN address (since the node is in DC2).
	nodes1 := obj1.(structs.Nodes)
	if len(nodes1) != 2 {
		t.Fatalf("bad: %v", obj1)
	}
	var address string
	for _, node := range nodes1 {
		if node.Node == "wan_translation_test" {
			address = node.Address
		}
	}
	if address != "127.0.0.2" {
		t.Fatalf("bad: %s", address)
	}

	// Query DC2 from DC2.
	resp2 := httptest.NewRecorder()
	obj2, err2 := a2.srv.CatalogNodes(resp2, req)
	if err2 != nil {
		t.Fatalf("err: %v", err2)
	}
	assertIndex(t, resp2)

	// Expect that DC2 gives us a private address (since the node is in DC2).
	nodes2 := obj2.(structs.Nodes)
	if len(nodes2) != 2 {
		t.Fatalf("bad: %v", obj2)
	}
	for _, node := range nodes2 {
		if node.Node == "wan_translation_test" {
			address = node.Address
		}
	}
	if address != "127.0.0.1" {
		t.Fatalf("bad: %s", address)
	}
}

func TestCatalogNodes_Blocking(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register node
	args := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}

	var out structs.IndexedNodes
	if err := a.RPC("Catalog.ListNodes", *args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// t.Fatal must be called from the main go routine
	// of the test. Because of this we cannot call
	// t.Fatal from within the go routines and use
	// an error channel instead.
	errch := make(chan error, 2)
	go func() {
		start := time.Now()

		// register a service after the blocking call
		// in order to unblock it.
		time.AfterFunc(100*time.Millisecond, func() {
			args := &structs.RegisterRequest{
				Datacenter: "dc1",
				Node:       "foo",
				Address:    "127.0.0.1",
			}
			var out struct{}
			errch <- a.RPC("Catalog.Register", args, &out)
		})

		// now block
		req, _ := http.NewRequest("GET", fmt.Sprintf("/v1/catalog/nodes?wait=3s&index=%d", out.Index+1), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.CatalogNodes(resp, req)
		if err != nil {
			errch <- err
		}

		// Should block for a while
		if d := time.Now().Sub(start); d < 50*time.Millisecond {
			errch <- fmt.Errorf("too fast: %v", d)
		}

		if idx := getIndex(t, resp); idx <= out.Index {
			errch <- fmt.Errorf("bad: %v", idx)
		}

		nodes := obj.(structs.Nodes)
		if len(nodes) != 2 {
			errch <- fmt.Errorf("bad: %v", obj)
		}
		errch <- nil
	}()

	// wait for both go routines to return
	if err := <-errch; err != nil {
		t.Fatal(err)
	}
	if err := <-errch; err != nil {
		t.Fatal(err)
	}
}

func TestCatalogNodes_DistanceSort(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register nodes.
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
	}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nobody has coordinates set so this will still return them in the
	// order they are indexed.
	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?dc=dc1&near=foo", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodes(resp, req)
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
	if nodes[2].Node != a.Config.NodeName {
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
	time.Sleep(300 * time.Millisecond)

	// Query again and now foo should have moved to the front of the line.
	req, _ = http.NewRequest("GET", "/v1/catalog/nodes?dc=dc1&near=foo", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.CatalogNodes(resp, req)
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
	if nodes[2].Node != a.Config.NodeName {
		t.Fatalf("bad: %v", nodes)
	}
}

func TestCatalogServices(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/catalog/services?dc=dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogServices(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	services := obj.(structs.Services)
	if len(services) != 2 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestCatalogServices_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		NodeMeta: map[string]string{
			"somekey": "somevalue",
		},
		Service: &structs.NodeService{
			Service: "api",
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/catalog/services?node-meta=somekey:somevalue", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogServices(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	services := obj.(structs.Services)
	if len(services) != 1 {
		t.Fatalf("bad: %v", obj)
	}
	if _, ok := services[args.Service.Service]; !ok {
		t.Fatalf("bad: %v", services)
	}
}

func TestCatalogServiceNodes(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Make sure an empty list is returned, not a nil
	{
		req, _ := http.NewRequest("GET", "/v1/catalog/service/api?tag=a", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.CatalogServiceNodes(resp, req)
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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/catalog/service/api?tag=a", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogServiceNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	assertIndex(t, resp)

	nodes := obj.(structs.ServiceNodes)
	if len(nodes) != 1 {
		t.Fatalf("bad: %v", obj)
	}
}

func TestCatalogServiceNodes_NodeMetaFilter(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	// Make sure an empty list is returned, not a nil
	{
		req, _ := http.NewRequest("GET", "/v1/catalog/service/api?node-meta=somekey:somevalue", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.CatalogServiceNodes(resp, req)
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
		NodeMeta: map[string]string{
			"somekey": "somevalue",
		},
		Service: &structs.NodeService{
			Service: "api",
		},
	}

	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/catalog/service/api?node-meta=somekey:somevalue", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogServiceNodes(resp, req)
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
	t.Parallel()
	cfg1 := TestConfig()
	cfg1.Datacenter = "dc1"
	cfg1.TranslateWanAddrs = true
	cfg1.ACLDatacenter = ""
	a1 := NewTestAgent(t.Name(), cfg1)
	defer a1.Shutdown()

	cfg2 := TestConfig()
	cfg2.Datacenter = "dc2"
	cfg2.TranslateWanAddrs = true
	cfg2.ACLDatacenter = ""
	a2 := NewTestAgent(t.Name(), cfg2)
	defer a2.Shutdown()

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.Ports.SerfWan)
	if _, err := a2.srv.agent.JoinWAN([]string{addr}); err != nil {
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

	// Query for the node in DC2 from DC1.
	req, _ := http.NewRequest("GET", "/v1/catalog/service/http_wan_translation_test?dc=dc2", nil)
	resp1 := httptest.NewRecorder()
	obj1, err1 := a1.srv.CatalogServiceNodes(resp1, req)
	if err1 != nil {
		t.Fatalf("err: %v", err1)
	}
	assertIndex(t, resp1)

	// Expect that DC1 gives us a WAN address (since the node is in DC2).
	nodes1 := obj1.(structs.ServiceNodes)
	if len(nodes1) != 1 {
		t.Fatalf("bad: %v", obj1)
	}
	node1 := nodes1[0]
	if node1.Address != "127.0.0.2" {
		t.Fatalf("bad: %v", node1)
	}

	// Query DC2 from DC2.
	resp2 := httptest.NewRecorder()
	obj2, err2 := a2.srv.CatalogServiceNodes(resp2, req)
	if err2 != nil {
		t.Fatalf("err: %v", err2)
	}
	assertIndex(t, resp2)

	// Expect that DC2 gives us a local address (since the node is in DC2).
	nodes2 := obj2.(structs.ServiceNodes)
	if len(nodes2) != 1 {
		t.Fatalf("bad: %v", obj2)
	}
	node2 := nodes2[0]
	if node2.Address != "127.0.0.1" {
		t.Fatalf("bad: %v", node2)
	}
}

func TestCatalogServiceNodes_DistanceSort(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/catalog/service/api?tag=a", nil)
	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			Service: "api",
			Tags:    []string{"a"},
		},
	}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Nobody has coordinates set so this will still return them in the
	// order they are indexed.
	req, _ = http.NewRequest("GET", "/v1/catalog/service/api?tag=a&near=foo", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogServiceNodes(resp, req)
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
	if err := a.RPC("Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	// Query again and now foo should have moved to the front of the line.
	req, _ = http.NewRequest("GET", "/v1/catalog/service/api?tag=a&near=foo", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.CatalogServiceNodes(resp, req)
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
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

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
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	req, _ := http.NewRequest("GET", "/v1/catalog/node/foo?dc=dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodeServices(resp, req)
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
	t.Parallel()
	cfg1 := TestConfig()
	cfg1.Datacenter = "dc1"
	cfg1.TranslateWanAddrs = true
	cfg1.ACLDatacenter = ""
	a1 := NewTestAgent(t.Name(), cfg1)
	defer a1.Shutdown()

	cfg2 := TestConfig()
	cfg2.Datacenter = "dc2"
	cfg2.TranslateWanAddrs = true
	cfg2.ACLDatacenter = ""
	a2 := NewTestAgent(t.Name(), cfg2)
	defer a2.Shutdown()

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.Ports.SerfWan)
	if _, err := a2.srv.agent.JoinWAN([]string{addr}); err != nil {
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

	// Query for the node in DC2 from DC1.
	req, _ := http.NewRequest("GET", "/v1/catalog/node/foo?dc=dc2", nil)
	resp1 := httptest.NewRecorder()
	obj1, err1 := a1.srv.CatalogNodeServices(resp1, req)
	if err1 != nil {
		t.Fatalf("err: %v", err1)
	}
	assertIndex(t, resp1)

	// Expect that DC1 gives us a WAN address (since the node is in DC2).
	services1 := obj1.(*structs.NodeServices)
	if len(services1.Services) != 1 {
		t.Fatalf("bad: %v", obj1)
	}
	service1 := services1.Node
	if service1.Address != "127.0.0.2" {
		t.Fatalf("bad: %v", service1)
	}

	// Query DC2 from DC2.
	resp2 := httptest.NewRecorder()
	obj2, err2 := a2.srv.CatalogNodeServices(resp2, req)
	if err2 != nil {
		t.Fatalf("err: %v", err2)
	}
	assertIndex(t, resp2)

	// Expect that DC2 gives us a private address (since the node is in DC2).
	services2 := obj2.(*structs.NodeServices)
	if len(services2.Services) != 1 {
		t.Fatalf("bad: %v", obj2)
	}
	service2 := services2.Node
	if service2.Address != "127.0.0.1" {
		t.Fatalf("bad: %v", service2)
	}
}
