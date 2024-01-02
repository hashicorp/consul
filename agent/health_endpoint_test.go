// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/serf/coordinate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func TestHealthChecksInState(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	t.Run("warning", func(t *testing.T) {
		a := NewTestAgent(t, "")
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
		a := NewTestAgent(t, "")
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

func TestHealthChecksInState_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.1",
		NodeMeta:   map[string]string{"somekey": "somevalue"},
		Check: &structs.HealthCheck{
			Node:   "bar",
			Name:   "node check 2",
			Status: api.HealthCritical,
		},
		SkipNodeUpdate: true,
	}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", "/v1/health/state/critical?filter="+url.QueryEscape("Name == `node check 2`"), nil)
	retry.Run(t, func(r *retry.R) {
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthChecksInState(resp, req)
		require.NoError(r, err)
		require.NoError(r, checkIndex(resp))

		// Should be 1 health check for the server
		nodes := obj.(structs.HealthChecks)
		require.Len(r, nodes, 1)
	})
}

func TestHealthChecksInState_DistanceSort(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Node, args.Check.Node = "foo", "foo"
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
	if err := a.RPC(context.Background(), "Coordinate.Update", &arg, &out); err != nil {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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

func TestHealthNodeChecks_Filtering(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create a node check
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "test-health-node",
		Address:    "127.0.0.2",
		Check: &structs.HealthCheck{
			Node: "test-health-node",
			Name: "check1",
		},
	}

	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	// Create a second check
	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "test-health-node",
		Address:    "127.0.0.2",
		Check: &structs.HealthCheck{
			Node: "test-health-node",
			Name: "check2",
		},
		SkipNodeUpdate: true,
	}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ := http.NewRequest("GET", "/v1/health/node/test-health-node?filter="+url.QueryEscape("Name == check2"), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthNodeChecks(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	// Should be 1 health check for the server
	nodes := obj.(structs.HealthChecks)
	require.Len(t, nodes, 1)
}

func TestHealthServiceChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
			Type:      "grpc",
		},
	}

	var out struct{}
	if err = a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
	if nodes[0].Type != "grpc" {
		t.Fatalf("expected grpc check type, got %s", nodes[0].Type)
	}
}

func TestHealthServiceChecks_NodeMetaFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
	if err = a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

func TestHealthServiceChecks_Filtering(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1&node-meta=somekey:somevalue", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceChecks(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	// Should be a non-nil empty list
	nodes := obj.(structs.HealthChecks)
	require.Empty(t, nodes)

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
		SkipNodeUpdate: true,
	}

	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	// Create a new node, service and check
	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "test-health-node",
		Address:    "127.0.0.2",
		NodeMeta:   map[string]string{"somekey": "somevalue"},
		Service: &structs.NodeService{
			ID:      "consul",
			Service: "consul",
		},
		Check: &structs.HealthCheck{
			Node:      "test-health-node",
			Name:      "consul check",
			ServiceID: "consul",
		},
	}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ = http.NewRequest("GET", "/v1/health/checks/consul?dc=dc1&filter="+url.QueryEscape("Node == `test-health-node`"), nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.HealthServiceChecks(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	// Should be 1 health check for consul
	nodes = obj.(structs.HealthChecks)
	require.Len(t, nodes, 1)
}

func TestHealthServiceChecks_DistanceSort(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Node, args.Check.Node = "foo", "foo"
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
	if err := a.RPC(context.Background(), "Coordinate.Update", &arg, &out); err != nil {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := StartTestAgent(t, TestAgent{HCL: ``, Overrides: `peering = { test_allow_peer_registrations = true }`})
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	testingPeerNames := []string{"", "my-peer"}

	for _, peerName := range testingPeerNames {
		req, err := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1"+peerQuerySuffix(peerName), nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthServiceNodes(resp, req)
		require.NoError(t, err)

		assertIndex(t, resp)

		nodes := obj.(structs.CheckServiceNodes)
		if peerName == "" {
			// Should be 1 health check for consul
			require.Len(t, nodes, 1)
		} else {
			require.NotNil(t, nodes)
			require.Len(t, nodes, 0)
		}

		req, err = http.NewRequest("GET", "/v1/health/service/nope?dc=dc1"+peerQuerySuffix(peerName), nil)
		require.NoError(t, err)
		resp = httptest.NewRecorder()
		obj, err = a.srv.HealthServiceNodes(resp, req)
		require.NoError(t, err)

		assertIndex(t, resp)

		// Should be a non-nil empty list
		nodes = obj.(structs.CheckServiceNodes)
		require.NotNil(t, nodes)
		require.Len(t, nodes, 0)
	}

	// TODO(peering): will have to seed this data differently in the future
	originalRegister := make(map[string]*structs.RegisterRequest)
	for _, peerName := range testingPeerNames {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			Node:       "bar",
			Address:    "127.0.0.1",
			PeerName:   peerName,
			Service: &structs.NodeService{
				ID:       "test",
				Service:  "test",
				PeerName: peerName,
			},
		}

		var out struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
		originalRegister[peerName] = args
	}

	verify := func(t *testing.T, peerName string, nodes structs.CheckServiceNodes) {
		require.Len(t, nodes, 1)
		require.Equal(t, peerName, nodes[0].Node.PeerName)
		require.Equal(t, "bar", nodes[0].Node.Node)
		require.Equal(t, peerName, nodes[0].Service.PeerName)
		require.Equal(t, "test", nodes[0].Service.Service)
		require.NotNil(t, nodes[0].Checks)
		require.Len(t, nodes[0].Checks, 0)
	}

	for _, peerName := range testingPeerNames {
		req, err := http.NewRequest("GET", "/v1/health/service/test?dc=dc1"+peerQuerySuffix(peerName), nil)
		require.NoError(t, err)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthServiceNodes(resp, req)
		require.NoError(t, err)

		assertIndex(t, resp)

		// Should be a non-nil empty list for checks
		nodes := obj.(structs.CheckServiceNodes)
		verify(t, peerName, nodes)

		// Test caching
		{
			// List instances with cache enabled
			req, err := http.NewRequest("GET", "/v1/health/service/test?cached"+peerQuerySuffix(peerName), nil)
			require.NoError(t, err)
			resp := httptest.NewRecorder()
			obj, err := a.srv.HealthServiceNodes(resp, req)
			require.NoError(t, err)
			nodes := obj.(structs.CheckServiceNodes)
			verify(t, peerName, nodes)

			// Should be a cache miss
			require.Equal(t, "MISS", resp.Header().Get("X-Cache"))
		}

		{
			// List instances with cache enabled
			req, err := http.NewRequest("GET", "/v1/health/service/test?cached"+peerQuerySuffix(peerName), nil)
			require.NoError(t, err)
			resp := httptest.NewRecorder()
			obj, err := a.srv.HealthServiceNodes(resp, req)
			require.NoError(t, err)
			nodes := obj.(structs.CheckServiceNodes)
			verify(t, peerName, nodes)

			// Should be a cache HIT now!
			require.Equal(t, "HIT", resp.Header().Get("X-Cache"))
		}
	}

	// Ensure background refresh works
	{
		// TODO(peering): will have to seed this data differently in the future
		for _, peerName := range testingPeerNames {
			args := originalRegister[peerName]
			// Register a new instance of the service
			args2 := *args
			args2.Node = "baz"
			args2.Address = "127.0.0.2"
			var out struct{}
			require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &args2, &out))
		}

		for _, peerName := range testingPeerNames {
			retry.Run(t, func(r *retry.R) {
				// List it again
				req, err := http.NewRequest("GET", "/v1/health/service/test?cached"+peerQuerySuffix(peerName), nil)
				require.NoError(r, err)
				resp := httptest.NewRecorder()
				obj, err := a.srv.HealthServiceNodes(resp, req)
				require.NoError(r, err)

				nodes := obj.(structs.CheckServiceNodes)
				require.Len(r, nodes, 2)

				header := resp.Header().Get("X-Consul-Index")
				if header == "" || header == "0" {
					r.Fatalf("Want non-zero header: %q", header)
				}
				_, err = strconv.ParseUint(header, 10, 64)
				require.NoError(r, err)

				// Should be a cache hit! The data should've updated in the cache
				// in the background so this should've been fetched directly from
				// the cache.
				if resp.Header().Get("X-Cache") != "HIT" {
					r.Fatalf("should be a cache hit")
				}
			})
		}
	}
}

func TestHealthServiceNodes_Blocking(t *testing.T) {
	t.Run("local data", func(t *testing.T) {
		testHealthServiceNodes_Blocking(t, structs.DefaultPeerKeyword)
	})

	t.Run("peered data", func(t *testing.T) {
		testHealthServiceNodes_Blocking(t, "my-peer")
	})
}

func testHealthServiceNodes_Blocking(t *testing.T, peerName string) {
	cases := []struct {
		name         string
		hcl          string
		grpcMetrics  bool
		queryBackend string
	}{
		{
			name:         "no streaming",
			queryBackend: "blocking-query",
			hcl:          `use_streaming_backend = false`,
		},
		{
			name:        "streaming",
			grpcMetrics: true,
			hcl: `
rpc { enable_streaming = true }
use_streaming_backend = true
`,
			queryBackend: "streaming",
		},
	}

	verify := func(t *testing.T, expectN int, nodes structs.CheckServiceNodes) {
		require.Len(t, nodes, expectN)

		for i, node := range nodes {
			require.Equal(t, peerName, node.Node.PeerName)
			if i == 2 {
				require.Equal(t, "zoo", node.Node.Node)
			} else {
				require.Equal(t, "bar", node.Node.Node)
			}
			require.Equal(t, "test", node.Service.Service)
		}
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sink := metrics.NewInmemSink(5*time.Second, time.Minute)
			metrics.NewGlobal(&metrics.Config{
				ServiceName:     "testing",
				AllowedPrefixes: []string{"testing.grpc."},
			}, sink)

			a := StartTestAgent(t, TestAgent{HCL: tc.hcl, Overrides: `peering = { test_allow_peer_registrations = true }`})
			defer a.Shutdown()

			testrpc.WaitForTestAgent(t, a.RPC, "dc1")

			// Register some initial service instances
			// TODO(peering): will have to seed this data differently in the future
			for i := 0; i < 2; i++ {
				args := &structs.RegisterRequest{
					Datacenter: "dc1",
					Node:       "bar",
					Address:    "127.0.0.1",
					PeerName:   peerName,
					Service: &structs.NodeService{
						ID:       fmt.Sprintf("test%03d", i),
						Service:  "test",
						PeerName: peerName,
					},
				}

				var out struct{}
				require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
			}

			// Initial request should return two instances
			req, _ := http.NewRequest("GET", "/v1/health/service/test?dc=dc1"+peerQuerySuffix(peerName), nil)
			resp := httptest.NewRecorder()
			obj, err := a.srv.HealthServiceNodes(resp, req)
			require.NoError(t, err)

			nodes := obj.(structs.CheckServiceNodes)
			verify(t, 2, nodes)

			idx := getIndex(t, resp)
			require.True(t, idx > 0)

			// errCh collects errors from goroutines since it's unsafe for them to use
			// t to fail tests directly.
			errCh := make(chan error, 1)

			checkErrs := func() {
				// Ensure no errors were sent on errCh and drain any nils we have
				for {
					select {
					case err := <-errCh:
						require.NoError(t, err)
					default:
						return
					}
				}
			}

			// Blocking on that index should block. We test that by launching another
			// goroutine that will wait a while before updating the registration and
			// make sure that we unblock before timeout and see the update but that it
			// takes at least as long as the sleep time.
			sleep := 200 * time.Millisecond
			start := time.Now()
			go func() {
				time.Sleep(sleep)

				// TODO(peering): will have to seed this data differently in the future
				args := &structs.RegisterRequest{
					Datacenter: "dc1",
					Node:       "zoo",
					Address:    "127.0.0.3",
					PeerName:   peerName,
					Service: &structs.NodeService{
						ID:       "test",
						Service:  "test",
						PeerName: peerName,
					},
				}

				var out struct{}
				errCh <- a.RPC(context.Background(), "Catalog.Register", args, &out)
			}()

			{
				timeout := 30 * time.Second
				url := fmt.Sprintf("/v1/health/service/test?dc=dc1&index=%d&wait=%s"+peerQuerySuffix(peerName), idx, timeout)
				req, _ := http.NewRequest("GET", url, nil)
				resp := httptest.NewRecorder()
				obj, err := a.srv.HealthServiceNodes(resp, req)
				require.NoError(t, err)
				elapsed := time.Since(start)
				require.True(t, elapsed > sleep, "request should block for at "+
					" least as long as sleep. sleep=%s, elapsed=%s", sleep, elapsed)

				require.True(t, elapsed < timeout, "request should unblock before"+
					" it timed out. timeout=%s, elapsed=%s", timeout, elapsed)

				nodes := obj.(structs.CheckServiceNodes)
				verify(t, 3, nodes)

				newIdx := getIndex(t, resp)
				require.True(t, idx < newIdx, "index should have increased."+
					"idx=%d, newIdx=%d", idx, newIdx)

				require.Equal(t, tc.queryBackend, resp.Header().Get("X-Consul-Query-Backend"))

				idx = newIdx

				checkErrs()
			}

			// Blocking should last until timeout in absence of updates
			start = time.Now()
			{
				timeout := 200 * time.Millisecond
				url := fmt.Sprintf("/v1/health/service/test?dc=dc1&index=%d&wait=%s"+peerQuerySuffix(peerName),
					idx, timeout)
				req, _ := http.NewRequest("GET", url, nil)
				resp := httptest.NewRecorder()
				obj, err := a.srv.HealthServiceNodes(resp, req)
				require.NoError(t, err)
				elapsed := time.Since(start)
				// Note that servers add jitter to timeout requested but don't remove it
				// so this should always be true.
				require.True(t, elapsed > timeout, "request should block for at "+
					" least as long as timeout. timeout=%s, elapsed=%s", timeout, elapsed)

				nodes := obj.(structs.CheckServiceNodes)
				verify(t, 3, nodes)

				newIdx := getIndex(t, resp)
				require.Equal(t, idx, newIdx)
				require.Equal(t, tc.queryBackend, resp.Header().Get("X-Consul-Query-Backend"))
			}

			if tc.grpcMetrics {
				data := sink.Data()
				if l := len(data); l < 1 {
					t.Errorf("expected at least 1 metrics interval, got :%v", l)
				}
				if count := len(data[0].Gauges); count < 2 {
					t.Errorf("expected at least 2 grpc gauge metrics, got: %v", count)
				}
			}
		})
	}
}

func TestHealthServiceNodes_Blocking_withFilter(t *testing.T) {
	t.Run("local data", func(t *testing.T) {
		testHealthServiceNodes_Blocking_withFilter(t, structs.DefaultPeerKeyword)
	})

	t.Run("peered data", func(t *testing.T) {
		testHealthServiceNodes_Blocking_withFilter(t, "my-peer")
	})
}

func testHealthServiceNodes_Blocking_withFilter(t *testing.T, peerName string) {
	cases := []struct {
		name         string
		hcl          string
		queryBackend string
	}{
		{
			name:         "no streaming",
			queryBackend: "blocking-query",
			hcl:          `use_streaming_backend = false`,
		},
		{
			name: "streaming",
			hcl: `
rpc { enable_streaming = true }
use_streaming_backend = true
`,
			queryBackend: "streaming",
		},
	}

	// TODO(peering): will have to seed this data differently in the future
	register := func(t *testing.T, a *TestAgent, name, tag string) {
		args := &structs.RegisterRequest{
			Datacenter: "dc1",
			ID:         types.NodeID("43d419c0-433b-42c3-bf8a-193eba0b41a3"),
			Node:       "node1",
			Address:    "127.0.0.1",
			PeerName:   peerName,
			Service: &structs.NodeService{
				ID:       name,
				Service:  name,
				PeerName: peerName,
				Tags:     []string{tag},
			},
		}

		var out struct{}
		require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			a := StartTestAgent(t, TestAgent{HCL: tc.hcl, Overrides: `peering = { test_allow_peer_registrations = true }`})
			defer a.Shutdown()

			testrpc.WaitForTestAgent(t, a.RPC, "dc1")

			// Register one with a tag.
			register(t, a, "web", "foo")

			filterUrlPart := "filter=" + url.QueryEscape("foo in Service.Tags")

			// TODO: use other call format

			// Initial request with a filter should return one.
			var lastIndex uint64
			testutil.RunStep(t, "read original", func(t *testing.T) {
				req, err := http.NewRequest("GET", "/v1/health/service/web?dc=dc1&"+filterUrlPart+peerQuerySuffix(peerName), nil)
				require.NoError(t, err)

				resp := httptest.NewRecorder()
				obj, err := a.srv.HealthServiceNodes(resp, req)
				require.NoError(t, err)

				nodes := obj.(structs.CheckServiceNodes)

				require.Len(t, nodes, 1)

				node := nodes[0]
				require.Equal(t, "node1", node.Node.Node)
				require.Equal(t, "web", node.Service.Service)
				require.Equal(t, []string{"foo"}, node.Service.Tags)

				require.Equal(t, "blocking-query", resp.Header().Get("X-Consul-Query-Backend"))

				idx := getIndex(t, resp)
				require.True(t, idx > 0)

				lastIndex = idx
			})

			const timeout = 30 * time.Second
			testutil.RunStep(t, "read blocking query result", func(t *testing.T) {
				var (
					// out and resp are not safe to read until reading from errCh
					out   structs.CheckServiceNodes
					resp  = httptest.NewRecorder()
					errCh = make(chan error, 1)
				)
				go func() {
					url := fmt.Sprintf("/v1/health/service/web?dc=dc1&index=%d&wait=%s&%s"+peerQuerySuffix(peerName), lastIndex, timeout, filterUrlPart)
					req, err := http.NewRequest("GET", url, nil)
					if err != nil {
						errCh <- err
						return
					}

					obj, err := a.srv.HealthServiceNodes(resp, req)
					if err != nil {
						errCh <- err
						return
					}

					nodes := obj.(structs.CheckServiceNodes)
					out = nodes
					errCh <- nil
				}()

				time.Sleep(200 * time.Millisecond)

				// Change the tags.
				register(t, a, "web", "bar")

				if err := <-errCh; err != nil {
					require.NoError(t, err)
				}

				require.Len(t, out, 0)
				require.Equal(t, tc.queryBackend, resp.Header().Get("X-Consul-Query-Backend"))
			})
		})
	}
}

func TestHealthServiceNodes_NodeMetaFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	tests := []struct {
		name         string
		config       string
		queryBackend string
	}{
		{
			name:         "blocking-query",
			config:       `use_streaming_backend=false`,
			queryBackend: "blocking-query",
		},
		{
			name: "cache-with-streaming",
			config: `
			rpc{
				enable_streaming=true
			}
			use_streaming_backend=true
		    `,
			queryBackend: "streaming",
		},
	}
	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			a := NewTestAgent(t, tst.config)
			testrpc.WaitForLeader(t, a.RPC, "dc1")
			testrpc.WaitForTestAgent(t, a.RPC, "dc1")
			waitForStreamingToBeReady(t, a)

			encodedMeta := url.QueryEscape("somekey:somevalue")

			var lastIndex uint64
			testutil.RunStep(t, "do initial read", func(t *testing.T) {
				u := fmt.Sprintf("/v1/health/service/test?dc=dc1&node-meta=%s", encodedMeta)

				req, err := http.NewRequest("GET", u, nil)
				require.NoError(t, err)
				resp := httptest.NewRecorder()
				obj, err := a.srv.HealthServiceNodes(resp, req)
				require.NoError(t, err)

				lastIndex = getIndex(t, resp)
				require.True(t, lastIndex > 0)

				// Should be a non-nil empty list
				nodes := obj.(structs.CheckServiceNodes)
				require.NotNil(t, nodes)
				require.Empty(t, nodes)
			})

			require.True(t, lastIndex > 0, "lastindex = %d", lastIndex)

			testutil.RunStep(t, "register item 1", func(t *testing.T) {
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

				var ignored struct{}
				require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &ignored))
			})

			testutil.RunStep(t, "register item 2", func(t *testing.T) {
				args := &structs.RegisterRequest{
					Datacenter: "dc1",
					Node:       "bar2",
					Address:    "127.0.0.1",
					NodeMeta:   map[string]string{"somekey": "othervalue"},
					Service: &structs.NodeService{
						ID:      "test2",
						Service: "test",
					},
				}
				var ignored struct{}
				require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &ignored))
			})

			testutil.RunStep(t, "do blocking read", func(t *testing.T) {
				u := fmt.Sprintf("/v1/health/service/test?dc=dc1&node-meta=%s&index=%d&wait=100ms&cached", encodedMeta, lastIndex)

				req, err := http.NewRequest("GET", u, nil)
				require.NoError(t, err)
				resp := httptest.NewRecorder()
				obj, err := a.srv.HealthServiceNodes(resp, req)
				require.NoError(t, err)

				assertIndex(t, resp)

				// Should be a non-nil empty list for checks
				nodes := obj.(structs.CheckServiceNodes)
				require.Len(t, nodes, 1)
				require.NotNil(t, nodes[0].Checks)
				require.Empty(t, nodes[0].Checks)

				require.Equal(t, tst.queryBackend, resp.Header().Get("X-Consul-Query-Backend"))
			})
		})
	}
}

func TestHealthServiceNodes_Filter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1&filter="+url.QueryEscape("Node.Node == `test-health-node`"), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceNodes(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	// Should be a non-nil empty list
	nodes := obj.(structs.CheckServiceNodes)
	require.Empty(t, nodes)

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
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	// Create a new node, service and check
	args = &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "test-health-node",
		Address:    "127.0.0.2",
		NodeMeta:   map[string]string{"somekey": "somevalue"},
		Service: &structs.NodeService{
			ID:      "consul",
			Service: "consul",
		},
		Check: &structs.HealthCheck{
			Node:      "test-health-node",
			Name:      "consul check",
			ServiceID: "consul",
		},
	}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ = http.NewRequest("GET", "/v1/health/service/consul?dc=dc1&filter="+url.QueryEscape("Node.Node == `test-health-node`"), nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.HealthServiceNodes(resp, req)
	require.NoError(t, err)

	assertIndex(t, resp)

	// Should be a list of checks with 1 element
	nodes = obj.(structs.CheckServiceNodes)
	require.Len(t, nodes, 1)
	require.Len(t, nodes[0].Checks, 1)
}

func TestHealthServiceNodes_DistanceSort(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	dc := "dc1"
	// Create a service check
	args := &structs.RegisterRequest{
		Datacenter: dc,
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
	testrpc.WaitForLeader(t, a.RPC, dc)
	var out struct{}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Node, args.Check.Node = "foo", "foo"
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
	if err := a.RPC(context.Background(), "Coordinate.Update", &arg, &out); err != nil {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()

	dc := "dc1"
	// Create a failing service check
	args := &structs.RegisterRequest{
		Datacenter: dc,
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Node:      a.Config.NodeName,
			Name:      "consul check",
			ServiceID: "consul",
			Status:    api.HealthCritical,
		},
	}

	retry.Run(t, func(r *retry.R) {
		var out struct{}
		if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
			r.Fatalf("err: %v", err)
		}
	})

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
		_, err := a.srv.HealthServiceNodes(resp, req)
		require.True(t, isHTTPBadRequest(err), fmt.Sprintf("Expected bad request HTTP error but got %v", err))
		if !strings.Contains(err.Error(), "Invalid value for ?passing") {
			t.Errorf("bad %s", err.Error())
		}
	})
}

func TestHealthServiceNodes_CheckType(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	req, _ := http.NewRequest("GET", "/v1/health/service/consul?dc=dc1", nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceNodes(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	// Should be 1 health check for consul
	nodes := obj.(structs.CheckServiceNodes)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		NodeMeta:   map[string]string{"somekey": "somevalue"},
		Check: &structs.HealthCheck{
			Node:      a.Config.NodeName,
			Name:      "consul check",
			ServiceID: "consul",
			Type:      "grpc",
		},
	}

	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	req, _ = http.NewRequest("GET", "/v1/health/service/consul?dc=dc1", nil)
	resp = httptest.NewRecorder()
	obj, err = a.srv.HealthServiceNodes(resp, req)
	require.NoError(t, err)

	assertIndex(t, resp)

	// Should be a non-nil empty list for checks
	nodes = obj.(structs.CheckServiceNodes)
	require.Len(t, nodes, 1)
	require.Len(t, nodes[0].Checks, 2)

	for _, check := range nodes[0].Checks {
		if check.Name == "consul check" && check.Type != "grpc" {
			t.Fatalf("exptected grpc check type, got %s", check.Type)
		}
	}
}

func TestHealthServiceNodes_WanTranslation(t *testing.T) {
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

	// Query for a service in DC2 from DC1.
	req, _ := http.NewRequest("GET", "/v1/health/service/http_wan_translation_test?dc=dc2", nil)
	resp1 := httptest.NewRecorder()
	obj1, err1 := a1.srv.HealthServiceNodes(resp1, req)
	require.NoError(t, err1)
	require.NoError(t, checkIndex(resp1))

	// Expect that DC1 gives us a WAN address (since the node is in DC2).
	nodes1, ok := obj1.(structs.CheckServiceNodes)
	require.True(t, ok, "obj1 is not a structs.CheckServiceNodes")
	require.Len(t, nodes1, 1)
	node1 := nodes1[0]
	require.NotNil(t, node1.Node)
	require.Equal(t, node1.Node.Address, "127.0.0.2")
	require.NotNil(t, node1.Service)
	require.Equal(t, node1.Service.Address, "1.2.3.4")
	require.Equal(t, node1.Service.Port, 80)

	// Query DC2 from DC2.
	resp2 := httptest.NewRecorder()
	obj2, err2 := a2.srv.HealthServiceNodes(resp2, req)
	require.NoError(t, err2)
	require.NoError(t, checkIndex(resp2))

	// Expect that DC2 gives us a local address (since the node is in DC2).
	nodes2, ok := obj2.(structs.CheckServiceNodes)
	require.True(t, ok, "obj2 is not a structs.ServiceNodes")
	require.Len(t, nodes2, 1)
	node2 := nodes2[0]
	require.NotNil(t, node2.Node)
	require.Equal(t, node2.Node.Address, "127.0.0.1")
	require.NotNil(t, node2.Service)
	require.Equal(t, node2.Service.Address, "127.0.0.1")
	require.Equal(t, node2.Service.Port, 8080)
}

func TestHealthConnectServiceNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
	defer a.Shutdown()

	// Register
	args := structs.TestRegisterRequestProxy(t)
	var out struct{}
	assert.Nil(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	// Request
	req, _ := http.NewRequest("GET", fmt.Sprintf(
		"/v1/health/connect/%s?dc=dc1", args.Service.Proxy.DestinationServiceName), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthConnectServiceNodes(resp, req)
	assert.Nil(t, err)
	assertIndex(t, resp)

	// Should be a non-nil empty list for checks
	nodes := obj.(structs.CheckServiceNodes)
	assert.Len(t, nodes, 1)
	assert.Len(t, nodes[0].Checks, 0)
}

func TestHealthIngressServiceNodes(t *testing.T) {
	t.Run("no streaming", func(t *testing.T) {
		testHealthIngressServiceNodes(t, ` rpc { enable_streaming = false } use_streaming_backend = false `)
	})
	t.Run("cache with streaming", func(t *testing.T) {
		testHealthIngressServiceNodes(t, ` rpc { enable_streaming = true } use_streaming_backend = true `)
	})
}

func testHealthIngressServiceNodes(t *testing.T, agentHCL string) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := NewTestAgent(t, agentHCL)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")
	waitForStreamingToBeReady(t, a)

	// Register gateway
	gatewayArgs := structs.TestRegisterIngressGateway(t)
	gatewayArgs.Service.Address = "127.0.0.27"
	var out struct{}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", gatewayArgs, &out))

	args := structs.TestRegisterRequest(t)
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	// Associate service to gateway
	cfgArgs := &structs.IngressGatewayConfigEntry{
		Name: "ingress-gateway",
		Kind: structs.IngressGateway,
		Listeners: []structs.IngressListener{
			{
				Port:     8888,
				Protocol: "tcp",
				Services: []structs.IngressService{
					{Name: args.Service.Service},
				},
			},
		},
	}

	req := structs.ConfigEntryRequest{
		Op:         structs.ConfigEntryUpsert,
		Datacenter: "dc1",
		Entry:      cfgArgs,
	}
	var outB bool
	require.Nil(t, a.RPC(context.Background(), "ConfigEntry.Apply", req, &outB))
	require.True(t, outB)

	checkResults := func(t *testing.T, obj interface{}) {
		nodes := obj.(structs.CheckServiceNodes)
		require.Len(t, nodes, 1)
		require.Equal(t, structs.ServiceKindIngressGateway, nodes[0].Service.Kind)
		require.Equal(t, gatewayArgs.Service.Address, nodes[0].Service.Address)
		require.Equal(t, gatewayArgs.Service.Proxy, nodes[0].Service.Proxy)
	}

	require.True(t, t.Run("associated service", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/ingress/%s", args.Service.Service), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthIngressServiceNodes(resp, req)
		require.NoError(t, err)
		assertIndex(t, resp)

		checkResults(t, obj)
	}))

	require.True(t, t.Run("non-associated service", func(t *testing.T) {
		req, _ := http.NewRequest("GET",
			"/v1/health/connect/notexist", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthIngressServiceNodes(resp, req)
		require.NoError(t, err)
		assertIndex(t, resp)

		nodes := obj.(structs.CheckServiceNodes)
		require.Len(t, nodes, 0)
	}))

	require.True(t, t.Run("test caching miss", func(t *testing.T) {
		// List instances with cache enabled
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/ingress/%s?cached", args.Service.Service), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthIngressServiceNodes(resp, req)
		require.NoError(t, err)

		checkResults(t, obj)

		// Should be a cache miss
		require.Equal(t, "MISS", resp.Header().Get("X-Cache"))
		// always a blocking query, because the ingress endpoint does not yet support streaming.
		require.Equal(t, "blocking-query", resp.Header().Get("X-Consul-Query-Backend"))
	}))

	require.True(t, t.Run("test caching hit", func(t *testing.T) {
		// List instances with cache enabled
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/ingress/%s?cached", args.Service.Service), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthIngressServiceNodes(resp, req)
		require.NoError(t, err)

		checkResults(t, obj)

		// Should be a cache HIT now!
		require.Equal(t, "HIT", resp.Header().Get("X-Cache"))
		// always a blocking query, because the ingress endpoint does not yet support streaming.
		require.Equal(t, "blocking-query", resp.Header().Get("X-Consul-Query-Backend"))
	}))
}

func TestHealthConnectServiceNodes_Filter(t *testing.T) {
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
		"/v1/health/connect/%s?filter=%s",
		args.Service.Proxy.DestinationServiceName,
		url.QueryEscape("Service.Meta.version == 2")), nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthConnectServiceNodes(resp, req)
	require.NoError(t, err)
	assertIndex(t, resp)

	nodes := obj.(structs.CheckServiceNodes)
	require.Len(t, nodes, 1)
	require.Equal(t, structs.ServiceKindConnectProxy, nodes[0].Service.Kind)
	require.Equal(t, args.Service.Address, nodes[0].Service.Address)
	require.Equal(t, args.Service.Proxy, nodes[0].Service.Proxy)
}

func TestHealthConnectServiceNodes_PassingFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := NewTestAgent(t, "")
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
	assert.Nil(t, a.RPC(context.Background(), "Catalog.Register", args, &out))

	t.Run("bc_no_query_value", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/connect/%s?passing", args.Service.Proxy.DestinationServiceName), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthConnectServiceNodes(resp, req)
		assert.Nil(t, err)
		assertIndex(t, resp)

		// Should be 0 health check for consul
		nodes := obj.(structs.CheckServiceNodes)
		assert.Len(t, nodes, 0)
	})

	t.Run("passing_true", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/connect/%s?passing=true", args.Service.Proxy.DestinationServiceName), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthConnectServiceNodes(resp, req)
		assert.Nil(t, err)
		assertIndex(t, resp)

		// Should be 0 health check for consul
		nodes := obj.(structs.CheckServiceNodes)
		assert.Len(t, nodes, 0)
	})

	t.Run("passing_false", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/connect/%s?passing=false", args.Service.Proxy.DestinationServiceName), nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.HealthConnectServiceNodes(resp, req)
		assert.Nil(t, err)
		assertIndex(t, resp)

		// Should be 1
		nodes := obj.(structs.CheckServiceNodes)
		assert.Len(t, nodes, 1)
	})

	t.Run("passing_bad", func(t *testing.T) {
		req, _ := http.NewRequest("GET", fmt.Sprintf(
			"/v1/health/connect/%s?passing=nope-nope", args.Service.Proxy.DestinationServiceName), nil)
		resp := httptest.NewRecorder()
		_, err := a.srv.HealthConnectServiceNodes(resp, req)
		assert.NotNil(t, err)
		assert.True(t, isHTTPBadRequest(err))

		assert.True(t, strings.Contains(err.Error(), "Invalid value for ?passing"))
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

func TestListHealthyServiceNodes_MergeCentralConfig(t *testing.T) {
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
		url := fmt.Sprintf("/v1/health/service/%s?merge-central-config", tc.serviceName)
		if tc.connect {
			url = fmt.Sprintf("/v1/health/connect/%s?merge-central-config", tc.serviceName)
		}
		req, _ := http.NewRequest("GET", url, nil)
		resp := httptest.NewRecorder()
		var obj interface{}
		var err error
		if tc.connect {
			obj, err = a.srv.HealthConnectServiceNodes(resp, req)
		} else {
			obj, err = a.srv.HealthServiceNodes(resp, req)
		}

		require.NoError(t, err)
		assertIndex(t, resp)

		checkServiceNodes := obj.(structs.CheckServiceNodes)

		// validate response
		require.Len(t, checkServiceNodes, 1)
		v := checkServiceNodes[0]

		validateMergeCentralConfigResponse(t, v.Service.ToServiceNode(registerServiceReq.Node), registerServiceReq, proxyGlobalEntry, serviceDefaultsConfigEntry)
	}
	testCases := []testCase{
		{
			testCaseName: "List healthy service instances with merge-central-config",
			serviceName:  registerServiceReq.Service.Service,
		},
		{
			testCaseName: "List healthy connect capable service instances with merge-central-config",
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

func TestHealthServiceNodes_MergeCentralConfigBlocking(t *testing.T) {
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
	var rpcResp structs.IndexedCheckServiceNodes
	require.NoError(t, a.RPC(context.Background(), "Health.ServiceNodes", &rpcReq, &rpcResp))

	require.Len(t, rpcResp.Nodes, 1)
	nodeService := rpcResp.Nodes[0].Service
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
	url := fmt.Sprintf("/v1/health/service/%s?merge-central-config&wait=%s&index=%d",
		registerServiceReq.Service.Service, waitDuration.String(), waitIndex)
	req, _ := http.NewRequest("GET", url, nil)
	resp := httptest.NewRecorder()
	obj, err := a.srv.HealthServiceNodes(resp, req)

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

	checkServiceNodes := obj.(structs.CheckServiceNodes)

	// validate response
	require.Len(t, checkServiceNodes, 1)
	v := checkServiceNodes[0].Service.ToServiceNode(registerServiceReq.Node)

	validateMergeCentralConfigResponse(t, v, registerServiceReq, proxyGlobalEntry, serviceDefaultsConfigEntry)
}

func peerQuerySuffix(peerName string) string {
	if peerName == "" {
		return ""
	}
	return "&peer=" + peerName
}

func waitForStreamingToBeReady(t *testing.T, a *TestAgent) {
	retry.Run(t, func(r *retry.R) {
		require.True(r, a.rpcClientHealth.IsReadyForStreaming())
	})
}
