package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

func TestCatalogRegister_PeeringRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	t.Run("deny peer registrations by default", func(t *testing.T) {
		a := NewTestAgent(t, "")
		defer a.Shutdown()

		// Register request with peer
		args := &structs.RegisterRequest{Node: "foo", PeerName: "foo", Address: "127.0.0.1"}
		req, _ := http.NewRequest("PUT", "/v1/catalog/register", jsonReader(args))

		obj, err := a.srv.CatalogRegister(nil, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot register requests with PeerName in them")
		require.Nil(t, obj)
	})

	t.Run("cannot hcl set the peer registrations config", func(t *testing.T) {
		// this will have no effect, as the value is overriden in non user source
		a := NewTestAgent(t, "peering = { test_allow_peer_registrations = true }")
		defer a.Shutdown()

		// Register request with peer
		args := &structs.RegisterRequest{Node: "foo", PeerName: "foo", Address: "127.0.0.1"}
		req, _ := http.NewRequest("PUT", "/v1/catalog/register", jsonReader(args))

		obj, err := a.srv.CatalogRegister(nil, req)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot register requests with PeerName in them")
		require.Nil(t, obj)
	})

	t.Run("allow peer registrations with test overrides", func(t *testing.T) {
		// the only way to set the config in the agent is via the overrides
		a := StartTestAgent(t, TestAgent{HCL: ``, Overrides: `peering = { test_allow_peer_registrations = true }`})
		defer a.Shutdown()

		// Register request with peer
		args := &structs.RegisterRequest{Node: "foo", PeerName: "foo", Address: "127.0.0.1"}
		req, _ := http.NewRequest("PUT", "/v1/catalog/register", jsonReader(args))

		obj, err := a.srv.CatalogRegister(nil, req)
		require.NoError(t, err)
		applied, ok := obj.(bool)
		require.True(t, ok)
		require.True(t, applied)
	})

}

func TestCatalogRegister_Service_InvalidAddress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
			req, _ := http.NewRequest("PUT", "/v1/catalog/register", jsonReader(args))
			_, err := a.srv.CatalogRegister(nil, req)
			if err == nil || err.Error() != "Invalid service address" {
				t.Fatalf("err: %v", err)
			}
		})
	}
}

func TestCatalogDeregister(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Register node
	args := &structs.DeregisterRequest{Node: "foo"}
	req, _ := http.NewRequest("PUT", "/v1/catalog/deregister", jsonReader(args))
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/catalog/datacenters", nil)
		obj, err := a.srv.CatalogDatacenters(nil, req)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		t.Fatalf("bad: %v ; nodes:=%v", obj, nodes)
	}
}

func TestCatalogNodes_MetaFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

func TestCatalogNodes_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?filter="+url.QueryEscape("Meta.somekey == somevalue"), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodes(resp, req)
	require.NoError(t, err)

	// Verify an index is set
	assertIndex(t, resp)

	// Verify we only get the node with the correct meta field back
	nodes := obj.(structs.Nodes)
	require.Len(t, nodes, 1)

	v, ok := nodes[0].Meta["somekey"]
	require.True(t, ok)
	require.Equal(t, v, "somevalue")
}

func TestCatalogNodes_WanTranslation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, `
		datacenter = "dc1"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a1.Shutdown()
	testrpc.WaitForTestAgent(t, a1.RPC, "dc1")

	a2 := NewTestAgent(t, `
		datacenter = "dc2"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a2.Shutdown()
	testrpc.WaitForTestAgent(t, a2.RPC, "dc2")

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	if _, err := a2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	testrpc.WaitForLeader(t, a1.RPC, "dc1")
	testrpc.WaitForLeader(t, a2.RPC, "dc2")
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
		if err := a2.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		t.Fatalf("bad: %v, nodes:=%v", obj1, nodes1)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1", testrpc.WaitForAntiEntropySync())

	// Run the query
	args := &structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var out structs.IndexedNodes
	if err := a.RPC(context.Background(), "Catalog.ListNodes", *args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Async cause a change
	waitIndex := out.Index
	start := time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond)
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "foo",
			Address:    "127.0.0.1",
		}
		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			t.Errorf("err: %v", err)
		}
	}()

	const waitDuration = 3 * time.Second

	// Re-run the query, if errantly woken up with no change, resume blocking.
	var elapsed time.Duration
RUN_BLOCKING_QUERY:
	req, err := http.NewRequest("GET", fmt.Sprintf("/v1/catalog/nodes?wait=%s&index=%d",
		waitDuration.String(),
		waitIndex), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodes(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	elapsed = time.Since(start)

	idx := getIndex(t, resp)
	if idx < waitIndex {
		t.Fatalf("bad: %v", idx)
	} else if idx == waitIndex {
		if elapsed > waitDuration {
			// This should prevent the loop from running longer than the
			// waitDuration
			t.Fatalf("too slow: %v", elapsed)
		}
		goto RUN_BLOCKING_QUERY
	}

	// Should block at least 100ms before getting the changed results
	if elapsed < 100*time.Millisecond {
		t.Fatalf("too fast: %v", elapsed)
	}

	nodes := obj.(structs.Nodes)
	if len(nodes) != 2 {
		t.Fatalf("bad: %v", obj)
	}

}

func TestCatalogNodes_DistanceSort(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register nodes.
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
	}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	// Nobody has coordinates set so this will still return them in the
	// order they are indexed.
	req, _ := http.NewRequest("GET", "/v1/catalog/nodes?dc=dc1&near=foo", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodes(resp, req)
	require.NoError(t, err)

	assertIndex(t, resp)
	nodes := obj.(structs.Nodes)
	require.Len(t, nodes, 3)
	require.Equal(t, "bar", nodes[0].Node)
	require.Equal(t, "foo", nodes[1].Node)
	require.Equal(t, a.Config.NodeName, nodes[2].Node)

	// Send an update for the node and wait for it to get applied.
	arg := structs.CoordinateUpdateRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Coord:      coordinate.NewCoordinate(coordinate.DefaultConfig()),
	}
	require.NoError(t, a.RPC(context.Background(), "Coordinate.Update", &arg, &out))
	time.Sleep(300 * time.Millisecond)

	// Query again and now foo should have moved to the front of the line.
	req, _ = http.NewRequest("GET", "/v1/catalog/nodes?dc=dc1&near=foo", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.CatalogNodes(resp, req)
	require.NoError(t, err)

	assertIndex(t, resp)
	nodes = obj.(structs.Nodes)
	require.Len(t, nodes, 3)
	require.Equal(t, "foo", nodes[0].Node)
	require.Equal(t, "bar", nodes[1].Node)
	require.Equal(t, a.Config.NodeName, nodes[2].Node)
}

func TestCatalogServices(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

func TestCatalogRegister_checkRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Register node with a service and check
	check := structs.HealthCheck{
		Node:      "foo",
		CheckID:   "foo-check",
		Name:      "foo check",
		ServiceID: "api",
		Definition: structs.HealthCheckDefinition{
			TCP:      "localhost:8888",
			Interval: 5 * time.Second,
		},
	}

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
		},
		Check: &check,
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/health/checks/api", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthServiceChecks(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		checks := obj.(structs.HealthChecks)
		if len(checks) != 1 {
			r.Fatalf("expected 1 check, got: %d", len(checks))
		}
		if checks[0].CheckID != check.CheckID {
			r.Fatalf("expected check id %s, got %s", check.Type, checks[0].Type)
		}
		if checks[0].Type != "tcp" {
			r.Fatalf("expected check type tcp, got %s", checks[0].Type)
		}
	})
}

func TestCatalogRegister_checkRegistration_UDP(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Register node with a service and check
	check := structs.HealthCheck{
		Node:      "foo",
		CheckID:   "foo-check",
		Name:      "foo check",
		ServiceID: "api",
		Definition: structs.HealthCheckDefinition{
			UDP:      "localhost:8888",
			Interval: 5 * time.Second,
		},
	}

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
		},
		Check: &check,
	}

	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/health/checks/api", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthServiceChecks(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		checks := obj.(structs.HealthChecks)
		if len(checks) != 1 {
			r.Fatalf("expected 1 check, got: %d", len(checks))
		}
		if checks[0].CheckID != check.CheckID {
			r.Fatalf("expected check id %s, got %s", check.Type, checks[0].Type)
		}
		if checks[0].Type != "udp" {
			r.Fatalf("expected check type udp, got %s", checks[0].Type)
		}
	})
}

func TestCatalogServiceNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

	// Test caching
	{
		// List instances with cache enabled
		req, _ := http.NewRequest("GET", "/v1/catalog/service/api?cached", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.CatalogServiceNodes(resp, req)
		require.NoError(t, err)
		nodes := obj.(structs.ServiceNodes)
		assert.Len(t, nodes, 1)

		// Should be a cache miss
		assert.Equal(t, "MISS", resp.Header().Get("X-Cache"))
	}

	{
		// List instances with cache enabled
		req, _ := http.NewRequest("GET", "/v1/catalog/service/api?cached", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.CatalogServiceNodes(resp, req)
		require.NoError(t, err)
		nodes := obj.(structs.ServiceNodes)
		assert.Len(t, nodes, 1)

		// Should be a cache HIT now!
		assert.Equal(t, "HIT", resp.Header().Get("X-Cache"))
		assert.Equal(t, "0", resp.Header().Get("Age"))
	}

	// Ensure background refresh works
	{
		// Register a new instance of the service
		args2 := args
		args2.Node = "bar"
		args2.Address = "127.0.0.2"
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

		retry.Run(t, func(r *retry.R) {
			// List it again
			req, _ := http.NewRequest("GET", "/v1/catalog/service/api?cached", nil)
			resp := httptest.NewRecorder()
			obj, err := a.srv.CatalogServiceNodes(resp, req)
			r.Check(err)

			nodes := obj.(structs.ServiceNodes)
			if len(nodes) != 2 {
				r.Fatalf("Want 2 nodes")
			}

			// Should be a cache hit! The data should've updated in the cache
			// in the background so this should've been fetched directly from
			// the cache.
			if resp.Header().Get("X-Cache") != "HIT" {
				r.Fatalf("should be a cache hit")
			}
		})
	}
}

func TestCatalogServiceNodes_NodeMetaFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

func TestCatalogServiceNodes_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	queryPath := "/v1/catalog/service/api?filter=" + url.QueryEscape("ServiceMeta.somekey == somevalue")

	// Make sure an empty list is returned, not a nil
	{
		req, _ := http.NewRequest("GET", queryPath, nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.CatalogServiceNodes(resp, req)
		require.NoError(t, err)

		assertIndex(t, resp)

		nodes := obj.(structs.ServiceNodes)
		require.Empty(t, nodes)
	}

	// Register node
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "api",
			Meta: map[string]string{
				"somekey": "somevalue",
			},
		},
	}

	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	// Register a second service for the node
	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "api2",
			Service: "api",
			Meta: map[string]string{
				"somekey": "notvalue",
			},
		},
		SkipNodeUpdate: true,
	}

	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", queryPath, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogServiceNodes(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	nodes := obj.(structs.ServiceNodes)
	require.Len(t, nodes, 1)
}

func TestCatalogServiceNodes_WanTranslation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, `
		datacenter = "dc1"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a1.Shutdown()

	a2 := NewTestAgent(t, `
		datacenter = "dc2"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a2.Shutdown()

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	_, err := a2.srv.agent.JoinWAN([]string{addr})
	require.NoError(t, err)
	retry.Run(t, func(r *retry.R) {
		require.Len(r, a1.WANMembers(), 2)
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
				Address: "127.0.0.1",
				Port:    8080,
				TaggedAddresses: map[string]structs.ServiceAddress{
					"wan": {
						Address: "1.2.3.4",
						Port:    80,
					},
				},
			},
		}

		var out struct{}
		require.NoError(t, a2.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// Query for the node in DC2 from DC1.
	req, _ := http.NewRequest("GET", "/v1/catalog/service/http_wan_translation_test?dc=dc2", nil)
	resp1 := httptest.NewRecorder()
	obj1, err1 := a1.srv.CatalogServiceNodes(resp1, req)
	require.NoError(t, err1)
	require.NoError(t, checkIndex(resp1))

	// Expect that DC1 gives us a WAN address (since the node is in DC2).
	nodes1, ok := obj1.(structs.ServiceNodes)
	require.True(t, ok, "obj1 is not a structs.ServiceNodes")
	require.Len(t, nodes1, 1)
	node1 := nodes1[0]
	require.Equal(t, node1.Address, "127.0.0.2")
	require.Equal(t, node1.ServiceAddress, "1.2.3.4")
	require.Equal(t, node1.ServicePort, 80)

	// Query DC2 from DC2.
	resp2 := httptest.NewRecorder()
	obj2, err2 := a2.srv.CatalogServiceNodes(resp2, req)
	require.NoError(t, err2)
	require.NoError(t, checkIndex(resp2))

	// Expect that DC2 gives us a local address (since the node is in DC2).
	nodes2, ok := obj2.(structs.ServiceNodes)
	require.True(t, ok, "obj2 is not a structs.ServiceNodes")
	require.Len(t, nodes2, 1)
	node2 := nodes2[0]
	require.Equal(t, node2.Address, "127.0.0.1")
	require.Equal(t, node2.ServiceAddress, "127.0.0.1")
	require.Equal(t, node2.ServicePort, 8080)
}

func TestCatalogServiceNodes_DistanceSort(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
	if err := a.RPC(context.Background(), "Coordinate.Update", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Eventually foo should move to the front of the line.
	retry.Run(t, func(r *retry.R) {
		req, _ = http.NewRequest("GET", "/v1/catalog/service/api?tag=a&near=foo", nil)
		resp = httptest.NewRecorder()
		obj, err = a.srv.CatalogServiceNodes(resp, req)
		if err != nil {
			r.Fatalf("err: %v", err)
		}

		assertIndex(t, resp)
		nodes = obj.(structs.ServiceNodes)
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

// Test that connect proxies can be queried via /v1/catalog/service/:service
// directly and that their results contain the proxy fields.
func TestCatalogServiceNodes_ConnectProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", fmt.Sprintf(
		"/v1/catalog/service/%s", args.Service.Service), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogServiceNodes(resp, req)
	assert.Nil(t, err)
	assertIndex(t, resp)

	nodes := obj.(structs.ServiceNodes)
	assert.Len(t, nodes, 1)
	assert.Equal(t, structs.ServiceKindConnectProxy, nodes[0].ServiceKind)
	assert.Equal(t, args.Service.Proxy, nodes[0].ServiceProxy)
}

func registerService(t *testing.T, a *TestAgent) (registerServiceReq *structs.RegisterRequest) {
	t.Helper()
	entMeta := acl.DefaultEnterpriseMeta()
	registerServiceReq = structs.TestRegisterRequestProxy(t)
	registerServiceReq.EnterpriseMeta = *entMeta
	registerServiceReq.Service.EnterpriseMeta = *entMeta
	registerServiceReq.Service.Proxy.Upstreams = structs.TestAddDefaultsToUpstreams(t, registerServiceReq.Service.Proxy.Upstreams, *entMeta)
	registerServiceReq.Check = &structs.HealthCheck{
		Node: registerServiceReq.Node,
		Name: "check1",
	}

	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", registerServiceReq, &out))

	return
}

func registerProxyDefaults(t *testing.T, a *TestAgent) (proxyGlobalEntry structs.ProxyConfigEntry) {
	t.Helper()
	// Register proxy-defaults
	proxyGlobalEntry = structs.ProxyConfigEntry{
		Kind: structs.ProxyDefaults,
		Name: structs.ProxyConfigGlobal,
		Mode: structs.ProxyModeDirect,
		Config: map[string]interface{}{
			"local_connect_timeout_ms": uint64(1000),
			"handshake_timeout_ms":     uint64(1000),
		},
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}
	proxyDefaultsConfigEntryReq := &structs.ConfigEntryRequest{
		Op:         structs.ConfigEntryUpsert,
		Datacenter: "dc1",
		Entry:      &proxyGlobalEntry,
	}
	var proxyDefaultsConfigEntryResp bool
	require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &proxyDefaultsConfigEntryReq, &proxyDefaultsConfigEntryResp))
	return
}

func registerServiceDefaults(t *testing.T, a *TestAgent, serviceName string) (serviceDefaultsConfigEntry structs.ServiceConfigEntry) {
	t.Helper()
	limits := 512
	serviceDefaultsConfigEntry = structs.ServiceConfigEntry{
		Kind: structs.ServiceDefaults,
		Name: serviceName,
		Mode: structs.ProxyModeTransparent,
		UpstreamConfig: &structs.UpstreamConfiguration{
			Defaults: &structs.UpstreamConfig{
				MeshGateway: structs.MeshGatewayConfig{
					Mode: structs.MeshGatewayModeLocal,
				},
				Limits: &structs.UpstreamLimits{
					MaxConnections: &limits,
				},
			},
		},
		EnterpriseMeta: *acl.DefaultEnterpriseMeta(),
	}
	serviceDefaultsConfigEntryReq := &structs.ConfigEntryRequest{
		Op:         structs.ConfigEntryUpsert,
		Datacenter: "dc1",
		Entry:      &serviceDefaultsConfigEntry,
	}
	var serviceDefaultsConfigEntryResp bool
	require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &serviceDefaultsConfigEntryReq, &serviceDefaultsConfigEntryResp))
	return
}

func validateMergeCentralConfigResponse(t *testing.T, v *structs.ServiceNode,
	registerServiceReq *structs.RegisterRequest,
	proxyGlobalEntry structs.ProxyConfigEntry,
	serviceDefaultsConfigEntry structs.ServiceConfigEntry) {

	t.Helper()
	require.Equal(t, registerServiceReq.Service.Service, v.ServiceName)
	// validate proxy global defaults are resolved in the merged service config
	require.Equal(t, proxyGlobalEntry.Config, v.ServiceProxy.Config)
	// validate service defaults override proxy-defaults/global
	require.NotEqual(t, proxyGlobalEntry.Mode, v.ServiceProxy.Mode)
	require.Equal(t, serviceDefaultsConfigEntry.Mode, v.ServiceProxy.Mode)
	// validate service defaults are resolved in the merged service config
	// expected number of upstreams = (number of upstreams defined in the register request proxy config +
	//	1 centrally configured default from service defaults)
	require.Equal(t, len(registerServiceReq.Service.Proxy.Upstreams)+1, len(v.ServiceProxy.Upstreams))
	for _, up := range v.ServiceProxy.Upstreams {
		if up.DestinationType != "" && up.DestinationType != structs.UpstreamDestTypeService {
			continue
		}
		require.Contains(t, up.Config, "limits")
		upstreamLimits := up.Config["limits"].(*structs.UpstreamLimits)
		require.Equal(t, serviceDefaultsConfigEntry.UpstreamConfig.Defaults.Limits.MaxConnections, upstreamLimits.MaxConnections)
		require.Equal(t, serviceDefaultsConfigEntry.UpstreamConfig.Defaults.MeshGateway.Mode, up.MeshGateway.Mode)
	}
}

func TestListServiceNodes_MergeCentralConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the service
	registerServiceReq := registerService(t, a)
	// Register proxy-defaults
	proxyGlobalEntry := registerProxyDefaults(t, a)
	// Register service-defaults
	serviceDefaultsConfigEntry := registerServiceDefaults(t, a, registerServiceReq.Service.Proxy.DestinationServiceName)

	type testCase struct {
		testCaseName string
		serviceName  string
		connect      bool
	}

	run := func(t *testing.T, tc testCase) {
		url := fmt.Sprintf("/v1/catalog/service/%s?merge-central-config", tc.serviceName)
		if tc.connect {
			url = fmt.Sprintf("/v1/catalog/connect/%s?merge-central-config", tc.serviceName)
		}
		req, _ := http.NewRequest("GET", url, nil)
		resp := httptest.NewRecorder()
		var obj interface{}
		var err error
		if tc.connect {
			obj, err = a.srv.CatalogConnectServiceNodes(resp, req)
		} else {
			obj, err = a.srv.CatalogServiceNodes(resp, req)
		}

		require.NoError(t, err)
		assertIndex(t, resp)

		serviceNodes := obj.(structs.ServiceNodes)

		// validate response
		require.Len(t, serviceNodes, 1)
		v := serviceNodes[0]

		validateMergeCentralConfigResponse(t, v, registerServiceReq, proxyGlobalEntry, serviceDefaultsConfigEntry)
	}
	testCases := []testCase{
		{
			testCaseName: "List service instances with merge-central-config",
			serviceName:  registerServiceReq.Service.Service,
		},
		{
			testCaseName: "List connect capable service instances with merge-central-config",
			serviceName:  registerServiceReq.Service.Proxy.DestinationServiceName,
			connect:      true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.testCaseName, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestCatalogServiceNodes_MergeCentralConfigBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the service
	registerServiceReq := registerService(t, a)
	// Register proxy-defaults
	proxyGlobalEntry := registerProxyDefaults(t, a)

	// Run the query
	rpcReq := structs.ServiceSpecificRequest{
		Datacenter:         "dc1",
		ServiceName:        registerServiceReq.Service.Service,
		MergeCentralConfig: true,
	}
	var rpcResp structs.IndexedServiceNodes
	require.NoError(t, a.RPC(context.Background(), "Catalog.ServiceNodes", &rpcReq, &rpcResp))

	require.Len(t, rpcResp.ServiceNodes, 1)
	serviceNode := rpcResp.ServiceNodes[0]
	require.Equal(t, registerServiceReq.Service.Service, serviceNode.ServiceName)
	// validate proxy global defaults are resolved in the merged service config
	require.Equal(t, proxyGlobalEntry.Config, serviceNode.ServiceProxy.Config)
	require.Equal(t, proxyGlobalEntry.Mode, serviceNode.ServiceProxy.Mode)

	// Async cause a change - register service defaults
	waitIndex := rpcResp.Index
	start := time.Now()
	var serviceDefaultsConfigEntry structs.ServiceConfigEntry
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Register service-defaults
		serviceDefaultsConfigEntry = registerServiceDefaults(t, a, registerServiceReq.Service.Proxy.DestinationServiceName)
	}()

	const waitDuration = 3 * time.Second
RUN_BLOCKING_QUERY:
	url := fmt.Sprintf("/v1/catalog/service/%s?merge-central-config&wait=%s&index=%d",
		registerServiceReq.Service.Service, waitDuration.String(), waitIndex)
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogServiceNodes(resp, req)

	require.NoError(t, err)
	assertIndex(t, resp)

	elapsed := time.Since(start)
	idx := getIndex(t, resp)
	if idx < waitIndex {
		t.Fatalf("bad index returned: %v", idx)
	} else if idx == waitIndex {
		if elapsed > waitDuration {
			// This should prevent the loop from running longer than the waitDuration
			t.Fatalf("too slow: %v", elapsed)
		}
		goto RUN_BLOCKING_QUERY
	}
	// Should block at least 100ms before getting the changed results
	if elapsed < 100*time.Millisecond {
		t.Fatalf("too fast: %v", elapsed)
	}

	serviceNodes := obj.(structs.ServiceNodes)

	// validate response
	require.Len(t, serviceNodes, 1)
	v := serviceNodes[0]

	validateMergeCentralConfigResponse(t, v, registerServiceReq, proxyGlobalEntry, serviceDefaultsConfigEntry)
}

// Test that the Connect-compatible endpoints can be queried for a
// service via /v1/catalog/connect/:service.
func TestCatalogConnectServiceNodes_good(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register
	args := structs.TestRegisterRequestProxy(t)
	args.Service.Address = "127.0.0.55"
	var out struct{}
	assert.Nil(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", fmt.Sprintf(
		"/v1/catalog/connect/%s", args.Service.Proxy.DestinationServiceName), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogConnectServiceNodes(resp, req)
	assert.Nil(t, err)
	assertIndex(t, resp)

	nodes := obj.(structs.ServiceNodes)
	assert.Len(t, nodes, 1)
	assert.Equal(t, structs.ServiceKindConnectProxy, nodes[0].ServiceKind)
	assert.Equal(t, args.Service.Address, nodes[0].ServiceAddress)
	assert.Equal(t, args.Service.Proxy, nodes[0].ServiceProxy)
}

func TestCatalogConnectServiceNodes_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register
	args := structs.TestRegisterRequestProxy(t)
	args.Service.Address = "127.0.0.55"
	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	args = structs.TestRegisterRequestProxy(t)
	args.Service.Address = "127.0.0.55"
	args.Service.Meta = map[string]string{
		"version": "2",
	}
	args.Service.ID = "web-proxy2"
	args.SkipNodeUpdate = true
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", fmt.Sprintf(
		"/v1/catalog/connect/%s?filter=%s",
		args.Service.Proxy.DestinationServiceName,
		url.QueryEscape("ServiceMeta.version == 2")), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogConnectServiceNodes(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	nodes := obj.(structs.ServiceNodes)
	require.Len(t, nodes, 1)
	require.Equal(t, structs.ServiceKindConnectProxy, nodes[0].ServiceKind)
	require.Equal(t, args.Service.Address, nodes[0].ServiceAddress)
	require.Equal(t, args.Service.Proxy, nodes[0].ServiceProxy)
}

func TestCatalogNodeServices(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register node with a regular service and connect proxy
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a connect proxy
	args.Service = structs.TestNodeServiceProxy(t)
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", "/v1/catalog/node/foo?dc=dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodeServices(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	services := obj.(*structs.NodeServices)
	if len(services.Services) != 2 {
		t.Fatalf("bad: %v", obj)
	}

	// Proxy service should have it's config intact
	require.Equal(t, args.Service.Proxy, services.Services["web-proxy"].Proxy)
}

func TestCatalogNodeServiceList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register node with a regular service and connect proxy
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a connect proxy
	args.Service = structs.TestNodeServiceProxy(t)
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", "/v1/catalog/node-services/foo?dc=dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodeServiceList(resp, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertIndex(t, resp)

	services := obj.(*structs.NodeServiceList)
	if len(services.Services) != 2 {
		t.Fatalf("bad: %v", obj)
	}

	var proxySvc *structs.NodeService
	for _, svc := range services.Services {
		if svc.ID == "web-proxy" {
			proxySvc = svc
		}
	}
	require.NotNil(t, proxySvc, "Missing proxy service registration")
	// Proxy service should have it's config intact
	require.Equal(t, args.Service.Proxy, proxySvc.Proxy)
}

func TestCatalogNodeServiceList_MergeCentralConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the service
	registerServiceReq := registerService(t, a)
	// Register proxy-defaults
	proxyGlobalEntry := registerProxyDefaults(t, a)
	// Register service-defaults
	serviceDefaultsConfigEntry := registerServiceDefaults(t, a, registerServiceReq.Service.Proxy.DestinationServiceName)

	url := fmt.Sprintf("/v1/catalog/node-services/%s?merge-central-config", registerServiceReq.Node)
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodeServiceList(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	nodeServices := obj.(*structs.NodeServiceList)
	// validate response
	require.Len(t, nodeServices.Services, 1)
	validateMergeCentralConfigResponse(t, nodeServices.Services[0].ToServiceNode(nodeServices.Node.Node), registerServiceReq, proxyGlobalEntry, serviceDefaultsConfigEntry)
}

func TestCatalogNodeServiceList_MergeCentralConfigBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register the service
	registerServiceReq := registerService(t, a)
	// Register proxy-defaults
	proxyGlobalEntry := registerProxyDefaults(t, a)

	// Run the query
	rpcReq := structs.NodeSpecificRequest{
		Datacenter:         "dc1",
		Node:               registerServiceReq.Node,
		MergeCentralConfig: true,
	}
	var rpcResp structs.IndexedNodeServiceList
	require.NoError(t, a.RPC(context.Background(), "Catalog.NodeServiceList", &rpcReq, &rpcResp))
	require.Len(t, rpcResp.NodeServices.Services, 1)
	nodeService := rpcResp.NodeServices.Services[0]
	require.Equal(t, registerServiceReq.Service.Service, nodeService.Service)
	// validate proxy global defaults are resolved in the merged service config
	require.Equal(t, proxyGlobalEntry.Config, nodeService.Proxy.Config)
	require.Equal(t, proxyGlobalEntry.Mode, nodeService.Proxy.Mode)

	// Async cause a change - register service defaults
	waitIndex := rpcResp.Index
	start := time.Now()
	var serviceDefaultsConfigEntry structs.ServiceConfigEntry
	go func() {
		time.Sleep(100 * time.Millisecond)
		// Register service-defaults
		serviceDefaultsConfigEntry = registerServiceDefaults(t, a, registerServiceReq.Service.Proxy.DestinationServiceName)
	}()

	const waitDuration = 3 * time.Second
RUN_BLOCKING_QUERY:

	url := fmt.Sprintf("/v1/catalog/node-services/%s?merge-central-config&wait=%s&index=%d",
		registerServiceReq.Node, waitDuration.String(), waitIndex)
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodeServiceList(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	elapsed := time.Since(start)
	idx := getIndex(t, resp)
	if idx < waitIndex {
		t.Fatalf("bad index returned: %v", idx)
	} else if idx == waitIndex {
		if elapsed > waitDuration {
			// This should prevent the loop from running longer than the waitDuration
			t.Fatalf("too slow: %v", elapsed)
		}
		goto RUN_BLOCKING_QUERY
	}
	// Should block at least 100ms before getting the changed results
	if elapsed < 100*time.Millisecond {
		t.Fatalf("too fast: %v", elapsed)
	}

	nodeServices := obj.(*structs.NodeServiceList)
	// validate response
	require.Len(t, nodeServices.Services, 1)
	validateMergeCentralConfigResponse(t, nodeServices.Services[0].ToServiceNode(nodeServices.Node.Node), registerServiceReq, proxyGlobalEntry, serviceDefaultsConfigEntry)
}

func TestCatalogNodeServices_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register node with a regular service and connect proxy
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
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	// Register a connect proxy
	args.Service = structs.TestNodeServiceProxy(t)
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", "/v1/catalog/node/foo?dc=dc1&filter="+url.QueryEscape("Kind == `connect-proxy`"), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodeServices(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	services := obj.(*structs.NodeServices)
	require.Len(t, services.Services, 1)

	// Proxy service should have it's config intact
	require.Equal(t, args.Service.Proxy, services.Services["web-proxy"].Proxy)
}

// Test that the services on a node contain all the Connect proxies on
// the node as well with their fields properly populated.
func TestCatalogNodeServices_ConnectProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", fmt.Sprintf(
		"/v1/catalog/node/%s", args.Node), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.CatalogNodeServices(resp, req)
	assert.Nil(t, err)
	assertIndex(t, resp)

	ns := obj.(*structs.NodeServices)
	assert.Len(t, ns.Services, 1)
	v := ns.Services[args.Service.Service]
	assert.Equal(t, structs.ServiceKindConnectProxy, v.Kind)
}

func TestCatalogNodeServices_WanTranslation(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := NewTestAgent(t, `
		datacenter = "dc1"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a1.Shutdown()
	testrpc.WaitForTestAgent(t, a1.RPC, "dc1")

	a2 := NewTestAgent(t, `
		datacenter = "dc2"
		translate_wan_addrs = true
		acl_datacenter = ""
	`)
	defer a2.Shutdown()
	testrpc.WaitForTestAgent(t, a2.RPC, "dc2")

	// Wait for the WAN join.
	addr := fmt.Sprintf("127.0.0.1:%d", a1.Config.SerfPortWAN)
	_, err := a2.srv.agent.JoinWAN([]string{addr})
	require.NoError(t, err)
	retry.Run(t, func(r *retry.R) {
		require.Len(r, a1.WANMembers(), 2)
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
				Address: "127.0.0.1",
				Port:    8080,
				TaggedAddresses: map[string]structs.ServiceAddress{
					"wan": {
						Address: "1.2.3.4",
						Port:    80,
					},
				},
			},
		}

		var out struct{}
		require.NoError(t, a2.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	// Query for the node in DC2 from DC1.
	req, _ := http.NewRequest("GET", "/v1/catalog/node/foo?dc=dc2", nil)
	resp1 := httptest.NewRecorder()
	obj1, err1 := a1.srv.CatalogNodeServices(resp1, req)
	require.NoError(t, err1)
	require.NoError(t, checkIndex(resp1))

	// Expect that DC1 gives us a WAN address (since the node is in DC2).
	service1, ok := obj1.(*structs.NodeServices)
	require.True(t, ok, "obj1 is not a *structs.NodeServices")
	require.NotNil(t, service1.Node)
	require.Equal(t, service1.Node.Address, "127.0.0.2")
	require.Len(t, service1.Services, 1)
	ns1, ok := service1.Services["http_wan_translation_test"]
	require.True(t, ok, "Missing service http_wan_translation_test")
	require.Equal(t, "1.2.3.4", ns1.Address)
	require.Equal(t, 80, ns1.Port)

	// Query DC2 from DC2.
	resp2 := httptest.NewRecorder()
	obj2, err2 := a2.srv.CatalogNodeServices(resp2, req)
	require.NoError(t, err2)
	require.NoError(t, checkIndex(resp2))

	// Expect that DC2 gives us a private address (since the node is in DC2).
	service2 := obj2.(*structs.NodeServices)
	require.True(t, ok, "obj2 is not a *structs.NodeServices")
	require.NotNil(t, service2.Node)
	require.Equal(t, service2.Node.Address, "127.0.0.1")
	require.Len(t, service2.Services, 1)
	ns2, ok := service2.Services["http_wan_translation_test"]
	require.True(t, ok, "Missing service http_wan_translation_test")
	require.Equal(t, ns2.Address, "127.0.0.1")
	require.Equal(t, ns2.Port, 8080)
}

func TestCatalog_GatewayServices_Terminating(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register a service to be covered by the wildcard  in the config entry
	args := structs.TestRegisterRequest(t)
	args.Service.Service = "redis"
	args.Check = &structs.HealthCheck{
		Name:      "redis",
		Status:    api.HealthPassing,
		ServiceID: args.Service.Service,
	}
	var out struct{}
	assert.NoError(t, a.RPC(context.Background(), "Catalog.Register", &args, &out))

	// Associate the gateway and api/redis services
	entryArgs := &structs.ConfigEntryRequest{
		Op:         structs.ConfigEntryUpsert,
		Datacenter: "dc1",
		Entry: &structs.TerminatingGatewayConfigEntry{
			Kind: "terminating-gateway",
			Name: "terminating",
			Services: []structs.LinkedService{
				{
					Name:     "api",
					CAFile:   "api/ca.crt",
					CertFile: "api/client.crt",
					KeyFile:  "api/client.key",
					SNI:      "my-domain",
				},
				{
					Name:     "*",
					CAFile:   "ca.crt",
					CertFile: "client.crt",
					KeyFile:  "client.key",
					SNI:      "my-alt-domain",
				},
			},
		},
	}
	var entryResp bool
	assert.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &entryArgs, &entryResp))

	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/catalog/gateway-services/terminating", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.CatalogGatewayServices(resp, req)
		assert.NoError(r, err)

		header := resp.Header().Get("X-Consul-Index")
		if header == "" || header == "0" {
			r.Fatalf("Bad: %v", header)
		}

		gatewayServices := obj.(structs.GatewayServices)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("terminating", nil),
				GatewayKind: structs.ServiceKindTerminatingGateway,
				CAFile:      "api/ca.crt",
				CertFile:    "api/client.crt",
				KeyFile:     "api/client.key",
				SNI:         "my-domain",
			},
			{
				Service:      structs.NewServiceName("redis", nil),
				Gateway:      structs.NewServiceName("terminating", nil),
				GatewayKind:  structs.ServiceKindTerminatingGateway,
				CAFile:       "ca.crt",
				CertFile:     "client.crt",
				KeyFile:      "client.key",
				SNI:          "my-alt-domain",
				FromWildcard: true,
			},
		}

		// Ignore raft index for equality
		for _, s := range gatewayServices {
			s.RaftIndex = structs.RaftIndex{}
		}
		assert.Equal(r, expect, gatewayServices)
	})
}

func TestCatalog_GatewayServices_Ingress(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Associate an ingress gateway with api/redis
	entryArgs := &structs.ConfigEntryRequest{
		Op:         structs.ConfigEntryUpsert,
		Datacenter: "dc1",
		Entry: &structs.IngressGatewayConfigEntry{
			Kind: "ingress-gateway",
			Name: "ingress",
			Listeners: []structs.IngressListener{
				{
					Port: 8888,
					Services: []structs.IngressService{
						{
							Name: "api",
						},
					},
				},
				{
					Port: 9999,
					Services: []structs.IngressService{
						{
							Name: "redis",
						},
					},
				},
			},
		},
	}

	var entryResp bool
	require.NoError(t, a.RPC(context.Background(), "ConfigEntry.Apply", &entryArgs, &entryResp))

	retry.Run(t, func(r *retry.R) {
		req, _ := http.NewRequest("GET", "/v1/catalog/gateway-services/ingress", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.CatalogGatewayServices(resp, req)
		require.NoError(r, err)

		header := resp.Header().Get("X-Consul-Index")
		if header == "" || header == "0" {
			r.Fatalf("Bad: %v", header)
		}

		gatewayServices := obj.(structs.GatewayServices)

		expect := structs.GatewayServices{
			{
				Service:     structs.NewServiceName("api", nil),
				Gateway:     structs.NewServiceName("ingress", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Protocol:    "tcp",
				Port:        8888,
			},
			{
				Service:     structs.NewServiceName("redis", nil),
				Gateway:     structs.NewServiceName("ingress", nil),
				GatewayKind: structs.ServiceKindIngressGateway,
				Protocol:    "tcp",
				Port:        9999,
			},
		}

		// Ignore raft index for equality
		for _, s := range gatewayServices {
			s.RaftIndex = structs.RaftIndex{}
		}
		require.Equal(r, expect, gatewayServices)
	})
}
