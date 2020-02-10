package local_test

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentAntiEntropy_Services(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register info
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
	}

	// Exists both, same (noop)
	var out struct{}
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"master"},
		Port:    5000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddService(srv1, "")
	args.Service = srv1
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists both, different (update)
	srv2 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{},
		Port:    8000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 0,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddService(srv2, "")

	srv2_mod := new(structs.NodeService)
	*srv2_mod = *srv2
	srv2_mod.Port = 9000
	args.Service = srv2_mod
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists local (create)
	srv3 := &structs.NodeService{
		ID:      "web",
		Service: "web",
		Tags:    []string{},
		Port:    80,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddService(srv3, "")

	// Exists remote (delete)
	srv4 := &structs.NodeService{
		ID:      "lb",
		Service: "lb",
		Tags:    []string{},
		Port:    443,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 0,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	args.Service = srv4
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists both, different address (update)
	srv5 := &structs.NodeService{
		ID:      "api",
		Service: "api",
		Tags:    []string{},
		Address: "127.0.0.10",
		Port:    8000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddService(srv5, "")

	srv5_mod := new(structs.NodeService)
	*srv5_mod = *srv5
	srv5_mod.Address = "127.0.0.1"
	args.Service = srv5_mod
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists local, in sync, remote missing (create)
	srv6 := &structs.NodeService{
		ID:      "cache",
		Service: "cache",
		Tags:    []string{},
		Port:    11211,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 0,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.SetServiceState(&local.ServiceState{
		Service: srv6,
		InSync:  true,
	})

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	var services structs.IndexedNodeServices
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}

	if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure we sent along our node info when we synced.
	id := services.NodeServices.Node.ID
	addrs := services.NodeServices.Node.TaggedAddresses
	meta := services.NodeServices.Node.Meta
	delete(meta, structs.MetaSegmentKey) // Added later, not in config.
	assert.Equal(t, a.Config.NodeID, id)
	assert.Equal(t, a.Config.TaggedAddresses, addrs)
	assert.Equal(t, a.Config.NodeMeta, meta)

	// We should have 6 services (consul included)
	if len(services.NodeServices.Services) != 6 {
		t.Fatalf("bad: %v", services.NodeServices.Services)
	}

	// All the services should match
	for id, serv := range services.NodeServices.Services {
		serv.CreateIndex, serv.ModifyIndex = 0, 0
		switch id {
		case "mysql":
			require.Equal(t, srv1, serv)
		case "redis":
			require.Equal(t, srv2, serv)
		case "web":
			require.Equal(t, srv3, serv)
		case "api":
			require.Equal(t, srv5, serv)
		case "cache":
			require.Equal(t, srv6, serv)
		case structs.ConsulServiceID:
			// ignore
		default:
			t.Fatalf("unexpected service: %v", id)
		}
	}

	if err := servicesInSync(a.State, 5, structs.DefaultEnterpriseMeta()); err != nil {
		t.Fatal(err)
	}

	// Remove one of the services
	a.State.RemoveService(structs.NewServiceID("api", nil))

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should have 5 services (consul included)
	if len(services.NodeServices.Services) != 5 {
		t.Fatalf("bad: %v", services.NodeServices.Services)
	}

	// All the services should match
	for id, serv := range services.NodeServices.Services {
		serv.CreateIndex, serv.ModifyIndex = 0, 0
		switch id {
		case "mysql":
			require.Equal(t, srv1, serv)
		case "redis":
			require.Equal(t, srv2, serv)
		case "web":
			require.Equal(t, srv3, serv)
		case "cache":
			require.Equal(t, srv6, serv)
		case structs.ConsulServiceID:
			// ignore
		default:
			t.Fatalf("unexpected service: %v", id)
		}
	}

	if err := servicesInSync(a.State, 4, structs.DefaultEnterpriseMeta()); err != nil {
		t.Fatal(err)
	}
}

func TestAgentAntiEntropy_Services_ConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := agent.NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register node info
	var out struct{}
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
	}

	// Exists both same (noop)
	srv1 := &structs.NodeService{
		Kind:    structs.ServiceKindConnectProxy,
		ID:      "mysql-proxy",
		Service: "mysql-proxy",
		Port:    5000,
		Proxy:   structs.ConnectProxyConfig{DestinationServiceName: "db"},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddService(srv1, "")
	args.Service = srv1
	assert.Nil(a.RPC("Catalog.Register", args, &out))

	// Exists both, different (update)
	srv2 := &structs.NodeService{
		ID:      "redis-proxy",
		Service: "redis-proxy",
		Port:    8000,
		Kind:    structs.ServiceKindConnectProxy,
		Proxy:   structs.ConnectProxyConfig{DestinationServiceName: "redis"},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 0,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddService(srv2, "")

	srv2_mod := new(structs.NodeService)
	*srv2_mod = *srv2
	srv2_mod.Port = 9000
	args.Service = srv2_mod
	assert.Nil(a.RPC("Catalog.Register", args, &out))

	// Exists local (create)
	srv3 := &structs.NodeService{
		ID:      "web-proxy",
		Service: "web-proxy",
		Port:    80,
		Kind:    structs.ServiceKindConnectProxy,
		Proxy:   structs.ConnectProxyConfig{DestinationServiceName: "web"},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddService(srv3, "")

	// Exists remote (delete)
	srv4 := &structs.NodeService{
		ID:      "lb-proxy",
		Service: "lb-proxy",
		Port:    443,
		Kind:    structs.ServiceKindConnectProxy,
		Proxy:   structs.ConnectProxyConfig{DestinationServiceName: "lb"},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 0,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	args.Service = srv4
	assert.Nil(a.RPC("Catalog.Register", args, &out))

	// Exists local, in sync, remote missing (create)
	srv5 := &structs.NodeService{
		ID:      "cache-proxy",
		Service: "cache-proxy",
		Port:    11211,
		Kind:    structs.ServiceKindConnectProxy,
		Proxy:   structs.ConnectProxyConfig{DestinationServiceName: "cache-proxy"},
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.SetServiceState(&local.ServiceState{
		Service: srv5,
		InSync:  true,
	})

	assert.Nil(a.State.SyncFull())

	var services structs.IndexedNodeServices
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}
	assert.Nil(a.RPC("Catalog.NodeServices", &req, &services))

	// We should have 5 services (consul included)
	assert.Len(services.NodeServices.Services, 5)

	// All the services should match
	for id, serv := range services.NodeServices.Services {
		serv.CreateIndex, serv.ModifyIndex = 0, 0
		switch id {
		case "mysql-proxy":
			assert.Equal(srv1, serv)
		case "redis-proxy":
			assert.Equal(srv2, serv)
		case "web-proxy":
			assert.Equal(srv3, serv)
		case "cache-proxy":
			assert.Equal(srv5, serv)
		case structs.ConsulServiceID:
			// ignore
		default:
			t.Fatalf("unexpected service: %v", id)
		}
	}

	assert.Nil(servicesInSync(a.State, 4, structs.DefaultEnterpriseMeta()))

	// Remove one of the services
	a.State.RemoveService(structs.NewServiceID("cache-proxy", nil))
	assert.Nil(a.State.SyncFull())
	assert.Nil(a.RPC("Catalog.NodeServices", &req, &services))

	// We should have 4 services (consul included)
	assert.Len(services.NodeServices.Services, 4)

	// All the services should match
	for id, serv := range services.NodeServices.Services {
		serv.CreateIndex, serv.ModifyIndex = 0, 0
		switch id {
		case "mysql-proxy":
			assert.Equal(srv1, serv)
		case "redis-proxy":
			assert.Equal(srv2, serv)
		case "web-proxy":
			assert.Equal(srv3, serv)
		case structs.ConsulServiceID:
			// ignore
		default:
			t.Fatalf("unexpected service: %v", id)
		}
	}

	assert.Nil(servicesInSync(a.State, 3, structs.DefaultEnterpriseMeta()))
}

func TestAgent_ServiceWatchCh(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	require := require.New(t)

	// register a local service
	srv1 := &structs.NodeService{
		ID:      "svc_id1",
		Service: "svc1",
		Tags:    []string{"tag1"},
		Port:    6100,
	}
	require.NoError(a.State.AddService(srv1, ""))

	verifyState := func(ss *local.ServiceState) {
		require.NotNil(ss)
		require.NotNil(ss.WatchCh)

		// Sanity check WatchCh blocks
		select {
		case <-ss.WatchCh:
			t.Fatal("should block until service changes")
		default:
		}
	}

	// Should be able to get a ServiceState
	ss := a.State.ServiceState(srv1.CompoundServiceID())
	verifyState(ss)

	// Update service in another go routine
	go func() {
		srv2 := srv1
		srv2.Port = 6200
		require.NoError(a.State.AddService(srv2, ""))
	}()

	// We should observe WatchCh close
	select {
	case <-ss.WatchCh:
		// OK!
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for WatchCh to close")
	}

	// Should also fire for state being set explicitly
	ss = a.State.ServiceState(srv1.CompoundServiceID())
	verifyState(ss)

	go func() {
		a.State.SetServiceState(&local.ServiceState{
			Service: ss.Service,
			Token:   "foo",
		})
	}()

	// We should observe WatchCh close
	select {
	case <-ss.WatchCh:
		// OK!
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for WatchCh to close")
	}

	// Should also fire for service being removed
	ss = a.State.ServiceState(srv1.CompoundServiceID())
	verifyState(ss)

	go func() {
		require.NoError(a.State.RemoveService(srv1.CompoundServiceID()))
	}()

	// We should observe WatchCh close
	select {
	case <-ss.WatchCh:
		// OK!
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for WatchCh to close")
	}
}

func TestAgentAntiEntropy_EnableTagOverride(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
	}
	var out struct{}

	// register a local service with tag override enabled
	srv1 := &structs.NodeService{
		ID:                "svc_id1",
		Service:           "svc1",
		Tags:              []string{"tag1"},
		Port:              6100,
		EnableTagOverride: true,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}
	a.State.AddService(srv1, "")

	// register a local service with tag override disabled
	srv2 := &structs.NodeService{
		ID:                "svc_id2",
		Service:           "svc2",
		Tags:              []string{"tag2"},
		Port:              6200,
		EnableTagOverride: false,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}
	a.State.AddService(srv2, "")

	// make sure they are both in the catalog
	if err := a.State.SyncChanges(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// update the services in the catalog and change the tags and port.
	// Only tag changes should be propagated for services where tag
	// override is enabled.
	args.Service = &structs.NodeService{
		ID:                srv1.ID,
		Service:           srv1.Service,
		Tags:              []string{"tag1_mod"},
		Port:              7100,
		EnableTagOverride: true,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	args.Service = &structs.NodeService{
		ID:                srv2.ID,
		Service:           srv2.Service,
		Tags:              []string{"tag2_mod"},
		Port:              7200,
		EnableTagOverride: false,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 0,
		},
	}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// sync catalog and local state
	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}
	var services structs.IndexedNodeServices

	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			r.Fatalf("err: %v", err)
		}

		// All the services should match
		for id, serv := range services.NodeServices.Services {
			serv.CreateIndex, serv.ModifyIndex = 0, 0
			switch id {
			case "svc_id1":
				// tags should be modified but not the port
				got := serv
				want := &structs.NodeService{
					ID:                "svc_id1",
					Service:           "svc1",
					Tags:              []string{"tag1_mod"},
					Port:              6100,
					EnableTagOverride: true,
					Weights: &structs.Weights{
						Passing: 1,
						Warning: 1,
					},
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				}
				assert.Equal(r, want, got)
			case "svc_id2":
				got, want := serv, srv2
				assert.Equal(r, want, got)
			case structs.ConsulServiceID:
				// ignore
			default:
				r.Fatalf("unexpected service: %v", id)
			}
		}

		if err := servicesInSync(a.State, 2, structs.DefaultEnterpriseMeta()); err != nil {
			r.Fatal(err)
		}
	})
}

func TestAgentAntiEntropy_Services_WithChecks(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	{
		// Single check
		srv := &structs.NodeService{
			ID:      "mysql",
			Service: "mysql",
			Tags:    []string{"master"},
			Port:    5000,
		}
		a.State.AddService(srv, "")

		chk := &structs.HealthCheck{
			Node:      a.Config.NodeName,
			CheckID:   "mysql",
			Name:      "mysql",
			ServiceID: "mysql",
			Status:    api.HealthPassing,
		}
		a.State.AddCheck(chk, "")

		if err := a.State.SyncFull(); err != nil {
			t.Fatal("sync failed: ", err)
		}

		// We should have 2 services (consul included)
		svcReq := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
		}
		var services structs.IndexedNodeServices
		if err := a.RPC("Catalog.NodeServices", &svcReq, &services); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(services.NodeServices.Services) != 2 {
			t.Fatalf("bad: %v", services.NodeServices.Services)
		}

		// We should have one health check
		chkReq := structs.ServiceSpecificRequest{
			Datacenter:  "dc1",
			ServiceName: "mysql",
		}
		var checks structs.IndexedHealthChecks
		if err := a.RPC("Health.ServiceChecks", &chkReq, &checks); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(checks.HealthChecks) != 1 {
			t.Fatalf("bad: %v", checks)
		}
	}

	{
		// Multiple checks
		srv := &structs.NodeService{
			ID:      "redis",
			Service: "redis",
			Tags:    []string{"master"},
			Port:    5000,
		}
		a.State.AddService(srv, "")

		chk1 := &structs.HealthCheck{
			Node:      a.Config.NodeName,
			CheckID:   "redis:1",
			Name:      "redis:1",
			ServiceID: "redis",
			Status:    api.HealthPassing,
		}
		a.State.AddCheck(chk1, "")

		chk2 := &structs.HealthCheck{
			Node:      a.Config.NodeName,
			CheckID:   "redis:2",
			Name:      "redis:2",
			ServiceID: "redis",
			Status:    api.HealthPassing,
		}
		a.State.AddCheck(chk2, "")

		if err := a.State.SyncFull(); err != nil {
			t.Fatal("sync failed: ", err)
		}

		// We should have 3 services (consul included)
		svcReq := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
		}
		var services structs.IndexedNodeServices
		if err := a.RPC("Catalog.NodeServices", &svcReq, &services); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(services.NodeServices.Services) != 3 {
			t.Fatalf("bad: %v", services.NodeServices.Services)
		}

		// We should have two health checks
		chkReq := structs.ServiceSpecificRequest{
			Datacenter:  "dc1",
			ServiceName: "redis",
		}
		var checks structs.IndexedHealthChecks
		if err := a.RPC("Health.ServiceChecks", &chkReq, &checks); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(checks.HealthChecks) != 2 {
			t.Fatalf("bad: %v", checks)
		}
	}
}

var testRegisterRules = `
 node "" {
 	policy = "write"
 }

 service "api" {
 	policy = "write"
 }

 service "consul" {
 	policy = "write"
 }
 `

func TestAgentAntiEntropy_Services_ACLDeny(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), `
		acl_datacenter = "dc1"
		acl_master_token = "root"
		acl_default_policy = "deny"
		acl_enforce_version_8 = true`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Create the ACL
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTokenTypeClient,
			Rules: testRegisterRules,
		},
		WriteRequest: structs.WriteRequest{
			Token: "root",
		},
	}
	var token string
	if err := a.RPC("ACL.Apply", &arg, &token); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create service (disallowed)
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"master"},
		Port:    5000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
	}
	a.State.AddService(srv1, token)

	// Create service (allowed)
	srv2 := &structs.NodeService{
		ID:      "api",
		Service: "api",
		Tags:    []string{"foo"},
		Port:    5001,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 0,
		},
	}
	a.State.AddService(srv2, token)

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that we are in sync
	{
		req := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
			QueryOptions: structs.QueryOptions{
				Token: "root",
			},
		}
		var services structs.IndexedNodeServices
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			t.Fatalf("err: %v", err)
		}

		// We should have 2 services (consul included)
		if len(services.NodeServices.Services) != 2 {
			t.Fatalf("bad: %v", services.NodeServices.Services)
		}

		// All the services should match
		for id, serv := range services.NodeServices.Services {
			serv.CreateIndex, serv.ModifyIndex = 0, 0
			switch id {
			case "mysql":
				t.Fatalf("should not be permitted")
			case "api":
				require.Equal(t, srv2, serv)
			case structs.ConsulServiceID:
				// ignore
			default:
				t.Fatalf("unexpected service: %v", id)
			}
		}

		if err := servicesInSync(a.State, 2, structs.DefaultEnterpriseMeta()); err != nil {
			t.Fatal(err)
		}
	}

	// Now remove the service and re-sync
	a.State.RemoveService(structs.NewServiceID("api", nil))
	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that we are in sync
	{
		req := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
			QueryOptions: structs.QueryOptions{
				Token: "root",
			},
		}
		var services structs.IndexedNodeServices
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			t.Fatalf("err: %v", err)
		}

		// We should have 1 service (just consul)
		if len(services.NodeServices.Services) != 1 {
			t.Fatalf("bad: %v", services.NodeServices.Services)
		}

		// All the services should match
		for id, serv := range services.NodeServices.Services {
			serv.CreateIndex, serv.ModifyIndex = 0, 0
			switch id {
			case "mysql":
				t.Fatalf("should not be permitted")
			case "api":
				t.Fatalf("should be deleted")
			case structs.ConsulServiceID:
				// ignore
			default:
				t.Fatalf("unexpected service: %v", id)
			}
		}

		if err := servicesInSync(a.State, 1, structs.DefaultEnterpriseMeta()); err != nil {
			t.Fatal(err)
		}
	}

	// Make sure the token got cleaned up.
	if token := a.State.ServiceToken(structs.NewServiceID("api", nil)); token != "" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgentAntiEntropy_Checks(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	// Register info
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
	}

	// Exists both, same (noop)
	var out struct{}
	chk1 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "mysql",
		Name:           "mysql",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddCheck(chk1, "")
	args.Check = chk1
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists both, different (update)
	chk2 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "redis",
		Name:           "redis",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddCheck(chk2, "")

	chk2_mod := new(structs.HealthCheck)
	*chk2_mod = *chk2
	chk2_mod.Status = api.HealthCritical
	args.Check = chk2_mod
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists local (create)
	chk3 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "web",
		Name:           "web",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddCheck(chk3, "")

	// Exists remote (delete)
	chk4 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "lb",
		Name:           "lb",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	args.Check = chk4
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists local, in sync, remote missing (create)
	chk5 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "cache",
		Name:           "cache",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.SetCheckState(&local.CheckState{
		Check:  chk5,
		InSync: true,
	})

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}
	var checks structs.IndexedHealthChecks

	// Verify that we are in sync
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should have 5 checks (serf included)
	if len(checks.HealthChecks) != 5 {
		t.Fatalf("bad: %v", checks)
	}

	// All the checks should match
	for _, chk := range checks.HealthChecks {
		chk.CreateIndex, chk.ModifyIndex = 0, 0
		switch chk.CheckID {
		case "mysql":
			require.Equal(t, chk, chk1)
		case "redis":
			require.Equal(t, chk, chk2)
		case "web":
			require.Equal(t, chk, chk3)
		case "cache":
			require.Equal(t, chk, chk5)
		case "serfHealth":
			// ignore
		default:
			t.Fatalf("unexpected check: %v", chk)
		}
	}

	if err := checksInSync(a.State, 4, structs.DefaultEnterpriseMeta()); err != nil {
		t.Fatal(err)
	}

	// Make sure we sent along our node info addresses when we synced.
	{
		req := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
		}
		var services structs.IndexedNodeServices
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			t.Fatalf("err: %v", err)
		}

		id := services.NodeServices.Node.ID
		addrs := services.NodeServices.Node.TaggedAddresses
		meta := services.NodeServices.Node.Meta
		delete(meta, structs.MetaSegmentKey) // Added later, not in config.
		assert.Equal(t, a.Config.NodeID, id)
		assert.Equal(t, a.Config.TaggedAddresses, addrs)
		assert.Equal(t, a.Config.NodeMeta, meta)
	}

	// Remove one of the checks
	a.State.RemoveCheck(structs.NewCheckID("redis", nil))

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that we are in sync
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should have 5 checks (serf included)
	if len(checks.HealthChecks) != 4 {
		t.Fatalf("bad: %v", checks)
	}

	// All the checks should match
	for _, chk := range checks.HealthChecks {
		chk.CreateIndex, chk.ModifyIndex = 0, 0
		switch chk.CheckID {
		case "mysql":
			require.Equal(t, chk1, chk)
		case "web":
			require.Equal(t, chk3, chk)
		case "cache":
			require.Equal(t, chk5, chk)
		case "serfHealth":
			// ignore
		default:
			t.Fatalf("unexpected check: %v", chk)
		}
	}

	if err := checksInSync(a.State, 3, structs.DefaultEnterpriseMeta()); err != nil {
		t.Fatal(err)
	}
}

func TestAgentAntiEntropy_RemovingServiceAndCheck(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	// Register info
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
	}

	var out struct{}

	// Exists remote (delete)
	svcID := "deleted-check-service"
	srv := &structs.NodeService{
		ID:      svcID,
		Service: "echo",
		Tags:    []string{},
		Address: "127.0.0.1",
		Port:    8080,
	}
	args.Service = srv
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists remote (delete)
	chk := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "lb",
		Name:           "lb",
		ServiceID:      svcID,
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}

	args.Check = chk
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	var services structs.IndexedNodeServices
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}

	if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	// The consul service will still be registered
	if len(services.NodeServices.Services) != 1 {
		t.Fatalf("Expected all services to be deleted, got: %#v", services.NodeServices.Services)
	}

	var checks structs.IndexedHealthChecks
	// Verify that we are in sync
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	// The serfHealth check will still be here
	if len(checks.HealthChecks) != 1 {
		t.Fatalf("Expected the health check to be deleted, got: %#v", checks.HealthChecks)
	}
}

func TestAgentAntiEntropy_Checks_ACLDeny(t *testing.T) {
	t.Parallel()
	dc := "dc1"
	a := &agent.TestAgent{Name: t.Name(), HCL: `
		acl_datacenter = "` + dc + `"
		acl_master_token = "root"
		acl_default_policy = "deny"
		acl_enforce_version_8 = true`}
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, dc)

	// Create the ACL
	arg := structs.ACLRequest{
		Datacenter: dc,
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTokenTypeClient,
			Rules: testRegisterRules,
		},
		WriteRequest: structs.WriteRequest{
			Token: "root",
		},
	}
	var token string
	if err := a.RPC("ACL.Apply", &arg, &token); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create services using the root token
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"master"},
		Port:    5000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddService(srv1, "root")
	srv2 := &structs.NodeService{
		ID:      "api",
		Service: "api",
		Tags:    []string{"foo"},
		Port:    5001,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddService(srv2, "root")

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that we are in sync
	{
		req := structs.NodeSpecificRequest{
			Datacenter: dc,
			Node:       a.Config.NodeName,
			QueryOptions: structs.QueryOptions{
				Token: "root",
			},
		}
		var services structs.IndexedNodeServices
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			t.Fatalf("err: %v", err)
		}

		// We should have 3 services (consul included)
		if len(services.NodeServices.Services) != 3 {
			t.Fatalf("bad: %v", services.NodeServices.Services)
		}

		// All the services should match
		for id, serv := range services.NodeServices.Services {
			serv.CreateIndex, serv.ModifyIndex = 0, 0
			switch id {
			case "mysql":
				require.Equal(t, srv1, serv)
			case "api":
				require.Equal(t, srv2, serv)
			case structs.ConsulServiceID:
				// ignore
			default:
				t.Fatalf("unexpected service: %v", id)
			}
		}

		if err := servicesInSync(a.State, 2, structs.DefaultEnterpriseMeta()); err != nil {
			t.Fatal(err)
		}
	}

	// This check won't be allowed.
	chk1 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		ServiceID:      "mysql",
		ServiceName:    "mysql",
		ServiceTags:    []string{"master"},
		CheckID:        "mysql-check",
		Name:           "mysql",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddCheck(chk1, token)

	// This one will be allowed.
	chk2 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		ServiceID:      "api",
		ServiceName:    "api",
		ServiceTags:    []string{"foo"},
		CheckID:        "api-check",
		Name:           "api",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}
	a.State.AddCheck(chk2, token)

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that we are in sync
	req := structs.NodeSpecificRequest{
		Datacenter: dc,
		Node:       a.Config.NodeName,
		QueryOptions: structs.QueryOptions{
			Token: "root",
		},
	}
	var checks structs.IndexedHealthChecks
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should have 2 checks (serf included)
	if len(checks.HealthChecks) != 2 {
		t.Fatalf("bad: %v", checks)
	}

	// All the checks should match
	for _, chk := range checks.HealthChecks {
		chk.CreateIndex, chk.ModifyIndex = 0, 0
		switch chk.CheckID {
		case "mysql-check":
			t.Fatalf("should not be permitted")
		case "api-check":
			require.Equal(t, chk, chk2)
		case "serfHealth":
			// ignore
		default:
			t.Fatalf("unexpected check: %v", chk)
		}
	}

	if err := checksInSync(a.State, 2, structs.DefaultEnterpriseMeta()); err != nil {
		t.Fatal(err)
	}

	// Now delete the check and wait for sync.
	a.State.RemoveCheck(structs.NewCheckID("api-check", nil))
	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that we are in sync
	{
		req := structs.NodeSpecificRequest{
			Datacenter: dc,
			Node:       a.Config.NodeName,
			QueryOptions: structs.QueryOptions{
				Token: "root",
			},
		}
		var checks structs.IndexedHealthChecks
		if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
			t.Fatalf("err: %v", err)
		}

		// We should have 1 check (just serf)
		if len(checks.HealthChecks) != 1 {
			t.Fatalf("bad: %v", checks)
		}

		// All the checks should match
		for _, chk := range checks.HealthChecks {
			chk.CreateIndex, chk.ModifyIndex = 0, 0
			switch chk.CheckID {
			case "mysql-check":
				t.Fatalf("should not be permitted")
			case "api-check":
				t.Fatalf("should be deleted")
			case "serfHealth":
				// ignore
			default:
				t.Fatalf("unexpected check: %v", chk)
			}
		}
	}

	if err := checksInSync(a.State, 1, structs.DefaultEnterpriseMeta()); err != nil {
		t.Fatal(err)
	}

	// Make sure the token got cleaned up.
	if token := a.State.CheckToken(structs.NewCheckID("api-check", nil)); token != "" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_UpdateCheck_DiscardOutput(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), `
		discard_check_output = true
		check_update_interval = "0s" # set to "0s" since otherwise output checks are deferred
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	inSync := func(id string) bool {
		s := a.State.CheckState(structs.NewCheckID(types.CheckID(id), nil))
		if s == nil {
			return false
		}
		return s.InSync
	}

	// register a check
	check := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "web",
		Name:    "web",
		Status:  api.HealthPassing,
		Output:  "first output",
	}
	if err := a.State.AddCheck(check, ""); err != nil {
		t.Fatalf("bad: %s", err)
	}
	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("bad: %s", err)
	}
	if !inSync("web") {
		t.Fatal("check should be in sync")
	}

	// update the check with the same status but different output
	// and the check should still be in sync.
	a.State.UpdateCheck(check.CompoundCheckID(), api.HealthPassing, "second output")
	if !inSync("web") {
		t.Fatal("check should be in sync")
	}

	// disable discarding of check output and update the check again with different
	// output. Then the check should be out of sync.
	a.State.SetDiscardCheckOutput(false)
	a.State.UpdateCheck(check.CompoundCheckID(), api.HealthPassing, "third output")
	if inSync("web") {
		t.Fatal("check should be out of sync")
	}
}

func TestAgentAntiEntropy_Check_DeferSync(t *testing.T) {
	t.Parallel()
	a := &agent.TestAgent{Name: t.Name(), HCL: `
		check_update_interval = "500ms"
	`}
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create a check
	check := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "web",
		Name:    "web",
		Status:  api.HealthPassing,
		Output:  "",
	}
	a.State.AddCheck(check, "")

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that we are in sync
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}
	var checks structs.IndexedHealthChecks
	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
			r.Fatalf("err: %v", err)
		}
		if got, want := len(checks.HealthChecks), 2; got != want {
			r.Fatalf("got %d health checks want %d", got, want)
		}
	})

	// Update the check output! Should be deferred
	a.State.UpdateCheck(structs.NewCheckID("web", nil), api.HealthPassing, "output")

	// We are going to wait up to 850ms for the deferred check update to run. The update
	// can happen any time within: check_update_interval / 2 + random(min: 0, max: check_update_interval)
	// For this test that means it will get deferred for 250ms - 750ms. We add up to 100ms on top of that to
	// account for potentially slow tests on a overloaded system.
	timer := &retry.Timer{Timeout: 850 * time.Millisecond, Wait: 50 * time.Millisecond}
	start := time.Now()
	retry.RunWith(timer, t, func(r *retry.R) {
		cs := a.State.CheckState(structs.NewCheckID("web", nil))
		if cs == nil {
			r.Fatalf("check is not registered")
		}

		if cs.DeferCheck != nil {
			r.Fatalf("Deferred Check timeout not removed yet")
		}
	})
	elapsed := time.Since(start)

	// ensure the check deferral didn't update too fast
	if elapsed < 240*time.Millisecond {
		t.Fatalf("early update: elapsed %v\n\n%+v", elapsed, checks)
	}

	// ensure the check deferral didn't update too late
	if elapsed > 850*time.Millisecond {
		t.Fatalf("late update: elapsed: %v\n\n%+v", elapsed, checks)
	}

	// Wait for a deferred update. TODO (slackpad) This isn't a great test
	// because we might be stuck in the random stagger from the full sync
	// after the leader election (~3 seconds) so it's easy to exceed the
	// default retry timeout here. Extending this makes the test a little
	// less flaky, but this isn't very clean for this first deferred update
	// since the full sync might pick it up, not the timer trigger. The
	// good news is that the later update below should be well past the full
	// sync so we are getting some coverage. We should rethink this a bit and
	// rework the deferred update stuff to be more testable.
	//
	// TODO - figure out why after the deferred check calls TriggerSyncChanges that this
	// takes so long to happen. I have seen it take upwards of 1.5s before the check gets
	// synced.
	timer = &retry.Timer{Timeout: 6 * time.Second, Wait: 100 * time.Millisecond}
	retry.RunWith(timer, t, func(r *retry.R) {
		if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
			r.Fatalf("err: %v", err)
		}

		// Verify updated
		for _, chk := range checks.HealthChecks {
			switch chk.CheckID {
			case "web":
				if chk.Output != "output" {
					r.Fatalf("no update: %v", chk)
				}
			}
		}
	})

	// Change the output in the catalog to force it out of sync.
	eCopy := check.Clone()
	eCopy.Output = "changed"
	reg := structs.RegisterRequest{
		Datacenter:      a.Config.Datacenter,
		Node:            a.Config.NodeName,
		Address:         a.Config.AdvertiseAddrLAN.IP.String(),
		TaggedAddresses: a.Config.TaggedAddresses,
		Check:           eCopy,
		WriteRequest:    structs.WriteRequest{},
	}
	var out struct{}
	if err := a.RPC("Catalog.Register", &reg, &out); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify that the output is out of sync.
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, chk := range checks.HealthChecks {
		switch chk.CheckID {
		case "web":
			if chk.Output != "changed" {
				t.Fatalf("unexpected update: %v", chk)
			}
		}
	}

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that the output was synced back to the agent's value.
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, chk := range checks.HealthChecks {
		switch chk.CheckID {
		case "web":
			if chk.Output != "output" {
				t.Fatalf("missed update: %v", chk)
			}
		}
	}

	// Reset the catalog again.
	if err := a.RPC("Catalog.Register", &reg, &out); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify that the output is out of sync.
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, chk := range checks.HealthChecks {
		switch chk.CheckID {
		case "web":
			if chk.Output != "changed" {
				t.Fatalf("unexpected update: %v", chk)
			}
		}
	}

	// Now make an update that should be deferred.
	a.State.UpdateCheck(structs.NewCheckID("web", nil), api.HealthPassing, "deferred")

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that the output is still out of sync since there's a deferred
	// update pending.
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}
	for _, chk := range checks.HealthChecks {
		switch chk.CheckID {
		case "web":
			if chk.Output != "changed" {
				t.Fatalf("unexpected update: %v", chk)
			}
		}
	}
	// Wait for the deferred update.
	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
			r.Fatal(err)
		}

		// Verify updated
		for _, chk := range checks.HealthChecks {
			switch chk.CheckID {
			case "web":
				if chk.Output != "deferred" {
					r.Fatalf("no update: %v", chk)
				}
			}
		}
	})

}

func TestAgentAntiEntropy_NodeInfo(t *testing.T) {
	t.Parallel()
	nodeID := types.NodeID("40e4a748-2192-161a-0510-9bf59fe950b5")
	nodeMeta := map[string]string{
		"somekey": "somevalue",
	}
	a := &agent.TestAgent{Name: t.Name(), HCL: `
		node_id = "40e4a748-2192-161a-0510-9bf59fe950b5"
		node_meta {
			somekey = "somevalue"
		}`}
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Register info
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
	}
	var out struct{}
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}
	var services structs.IndexedNodeServices
	if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	id := services.NodeServices.Node.ID
	addrs := services.NodeServices.Node.TaggedAddresses
	meta := services.NodeServices.Node.Meta
	delete(meta, structs.MetaSegmentKey) // Added later, not in config.
	require.Equal(t, a.Config.NodeID, id)
	require.Equal(t, a.Config.TaggedAddresses, addrs)
	require.Equal(t, a.Config.NodeMeta, meta)

	// Blow away the catalog version of the node info
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the sync - this should have been a sync of just the node info
	if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	{
		id := services.NodeServices.Node.ID
		addrs := services.NodeServices.Node.TaggedAddresses
		meta := services.NodeServices.Node.Meta
		delete(meta, structs.MetaSegmentKey) // Added later, not in config.
		require.Equal(t, nodeID, id)
		require.Equal(t, a.Config.TaggedAddresses, addrs)
		require.Equal(t, nodeMeta, meta)
	}
}

func TestAgent_ServiceTokens(t *testing.T) {
	t.Parallel()

	tokens := new(token.Store)
	tokens.UpdateUserToken("default", token.TokenSourceConfig)
	cfg := config.DefaultRuntimeConfig(`bind_addr = "127.0.0.1" data_dir = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, tokens)
	l.TriggerSyncChanges = func() {}

	l.AddService(&structs.NodeService{ID: "redis"}, "")

	// Returns default when no token is set
	if token := l.ServiceToken(structs.NewServiceID("redis", nil)); token != "default" {
		t.Fatalf("bad: %s", token)
	}

	// Returns configured token
	l.AddService(&structs.NodeService{ID: "redis"}, "abc123")
	if token := l.ServiceToken(structs.NewServiceID("redis", nil)); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}

	// Keeps token around for the delete
	l.RemoveService(structs.NewServiceID("redis", nil))
	if token := l.ServiceToken(structs.NewServiceID("redis", nil)); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_CheckTokens(t *testing.T) {
	t.Parallel()

	tokens := new(token.Store)
	tokens.UpdateUserToken("default", token.TokenSourceConfig)
	cfg := config.DefaultRuntimeConfig(`bind_addr = "127.0.0.1" data_dir = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, tokens)
	l.TriggerSyncChanges = func() {}

	// Returns default when no token is set
	l.AddCheck(&structs.HealthCheck{CheckID: types.CheckID("mem")}, "")
	if token := l.CheckToken(structs.NewCheckID("mem", nil)); token != "default" {
		t.Fatalf("bad: %s", token)
	}

	// Returns configured token
	l.AddCheck(&structs.HealthCheck{CheckID: types.CheckID("mem")}, "abc123")
	if token := l.CheckToken(structs.NewCheckID("mem", nil)); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}

	// Keeps token around for the delete
	l.RemoveCheck(structs.NewCheckID("mem", nil))
	if token := l.CheckToken(structs.NewCheckID("mem", nil)); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_CheckCriticalTime(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultRuntimeConfig(`bind_addr = "127.0.0.1" data_dir = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, new(token.Store))
	l.TriggerSyncChanges = func() {}

	svc := &structs.NodeService{ID: "redis", Service: "redis", Port: 8000}
	l.AddService(svc, "")

	// Add a passing check and make sure it's not critical.
	checkID := types.CheckID("redis:1")
	chk := &structs.HealthCheck{
		Node:      "node",
		CheckID:   checkID,
		Name:      "redis:1",
		ServiceID: "redis",
		Status:    api.HealthPassing,
	}
	l.AddCheck(chk, "")
	if checks := l.CriticalCheckStates(structs.DefaultEnterpriseMeta()); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Set it to warning and make sure that doesn't show up as critical.
	l.UpdateCheck(structs.NewCheckID(checkID, nil), api.HealthWarning, "")
	if checks := l.CriticalCheckStates(structs.DefaultEnterpriseMeta()); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Fail the check and make sure the time looks reasonable.
	l.UpdateCheck(structs.NewCheckID(checkID, nil), api.HealthCritical, "")
	if c, ok := l.CriticalCheckStates(structs.DefaultEnterpriseMeta())[structs.NewCheckID(checkID, nil)]; !ok {
		t.Fatalf("should have a critical check")
	} else if c.CriticalFor() > time.Millisecond {
		t.Fatalf("bad: %#v, check was critical for %v", c, c.CriticalFor())
	}

	// Wait a while, then fail it again and make sure the time keeps track
	// of the initial failure, and doesn't reset here. Since we are sleeping for
	// 50ms the check should not be any less than that.
	time.Sleep(50 * time.Millisecond)
	l.UpdateCheck(chk.CompoundCheckID(), api.HealthCritical, "")
	if c, ok := l.CriticalCheckStates(structs.DefaultEnterpriseMeta())[structs.NewCheckID(checkID, nil)]; !ok {
		t.Fatalf("should have a critical check")
	} else if c.CriticalFor() < 50*time.Millisecond {
		t.Fatalf("bad: %#v, check was critical for %v", c, c.CriticalFor())
	}

	// Set it passing again.
	l.UpdateCheck(structs.NewCheckID(checkID, nil), api.HealthPassing, "")
	if checks := l.CriticalCheckStates(structs.DefaultEnterpriseMeta()); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Fail the check and make sure the time looks like it started again
	// from the latest failure, not the original one.
	l.UpdateCheck(structs.NewCheckID(checkID, nil), api.HealthCritical, "")
	if c, ok := l.CriticalCheckStates(structs.DefaultEnterpriseMeta())[structs.NewCheckID(checkID, nil)]; !ok {
		t.Fatalf("should have a critical check")
	} else if c.CriticalFor() > time.Millisecond {
		t.Fatalf("bad: %#v, check was critical for %v", c, c.CriticalFor())
	}
}

func TestAgent_AddCheckFailure(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultRuntimeConfig(`bind_addr = "127.0.0.1" data_dir = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, new(token.Store))
	l.TriggerSyncChanges = func() {}

	// Add a check for a service that does not exist and verify that it fails
	checkID := types.CheckID("redis:1")
	chk := &structs.HealthCheck{
		Node:      "node",
		CheckID:   checkID,
		Name:      "redis:1",
		ServiceID: "redis",
		Status:    api.HealthPassing,
	}
	wantErr := errors.New(`Check "redis:1" refers to non-existent service "redis"`)

	got := l.AddCheck(chk, "")
	require.Equal(t, wantErr, got)
}

func TestAgent_AliasCheck(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	cfg := config.DefaultRuntimeConfig(`bind_addr = "127.0.0.1" data_dir = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, new(token.Store))
	l.TriggerSyncChanges = func() {}

	// Add checks
	require.NoError(l.AddService(&structs.NodeService{Service: "s1"}, ""))
	require.NoError(l.AddService(&structs.NodeService{Service: "s2"}, ""))
	require.NoError(l.AddCheck(&structs.HealthCheck{CheckID: types.CheckID("c1"), ServiceID: "s1"}, ""))
	require.NoError(l.AddCheck(&structs.HealthCheck{CheckID: types.CheckID("c2"), ServiceID: "s2"}, ""))

	// Add an alias
	notifyCh := make(chan struct{}, 1)
	require.NoError(l.AddAliasCheck(structs.NewCheckID(types.CheckID("a1"), nil), structs.NewServiceID("s1", nil), notifyCh))

	// Update and verify we get notified
	l.UpdateCheck(structs.NewCheckID(types.CheckID("c1"), nil), api.HealthCritical, "")
	select {
	case <-notifyCh:
	default:
		t.Fatal("notify not received")
	}

	// Update again and verify we do not get notified
	l.UpdateCheck(structs.NewCheckID(types.CheckID("c1"), nil), api.HealthCritical, "")
	select {
	case <-notifyCh:
		t.Fatal("notify received")
	default:
	}

	// Update other check and verify we do not get notified
	l.UpdateCheck(structs.NewCheckID(types.CheckID("c2"), nil), api.HealthCritical, "")
	select {
	case <-notifyCh:
		t.Fatal("notify received")
	default:
	}

	// Update change and verify we get notified
	l.UpdateCheck(structs.NewCheckID(types.CheckID("c1"), nil), api.HealthPassing, "")
	select {
	case <-notifyCh:
	default:
		t.Fatal("notify not received")
	}
}

func TestAgent_sendCoordinate(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t, t.Name(), `
		sync_coordinate_interval_min = "1ms"
		sync_coordinate_rate_target = 10.0
		consul = {
			coordinate = {
				update_period = "100ms"
				update_batch_size = 10
				update_max_batches = 1
			}
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	t.Logf("%d %d %s",
		a.Config.ConsulCoordinateUpdateBatchSize,
		a.Config.ConsulCoordinateUpdateMaxBatches,
		a.Config.ConsulCoordinateUpdatePeriod.String())

	// Make sure the coordinate is present.
	req := structs.DCSpecificRequest{
		Datacenter: a.Config.Datacenter,
	}
	var reply structs.IndexedCoordinates
	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Coordinate.ListNodes", &req, &reply); err != nil {
			r.Fatalf("err: %s", err)
		}
		if len(reply.Coordinates) != 1 {
			r.Fatalf("expected a coordinate: %v", reply)
		}
		coord := reply.Coordinates[0]
		if coord.Node != a.Config.NodeName || coord.Coord == nil {
			r.Fatalf("bad: %v", coord)
		}
	})
}

func servicesInSync(state *local.State, wantServices int, entMeta *structs.EnterpriseMeta) error {
	services := state.ServiceStates(entMeta)
	if got, want := len(services), wantServices; got != want {
		return fmt.Errorf("got %d services want %d", got, want)
	}
	for id, s := range services {
		if !s.InSync {
			return fmt.Errorf("service %q should be in sync %+v", id.String(), s)
		}
	}
	return nil
}

func checksInSync(state *local.State, wantChecks int, entMeta *structs.EnterpriseMeta) error {
	checks := state.CheckStates(entMeta)
	if got, want := len(checks), wantChecks; got != want {
		return fmt.Errorf("got %d checks want %d", got, want)
	}
	for id, c := range checks {
		if !c.InSync {
			return fmt.Errorf("check %q should be in sync", id.String())
		}
	}
	return nil
}

func TestState_Notify(t *testing.T) {
	t.Parallel()
	logger := hclog.New(&hclog.LoggerOptions{
		Output: os.Stderr,
	})

	state := local.NewState(local.Config{},
		logger, &token.Store{})

	// Stub state syncing
	state.TriggerSyncChanges = func() {}

	require := require.New(t)
	assert := assert.New(t)

	// Register a notifier
	notifyCh := make(chan struct{}, 1)
	state.Notify(notifyCh)
	defer state.StopNotify(notifyCh)
	assert.Empty(notifyCh)
	drainCh(notifyCh)

	// Add a service
	err := state.AddService(&structs.NodeService{
		Service: "web",
	}, "fake-token-web")
	require.NoError(err)

	// Should have a notification
	assert.NotEmpty(notifyCh)
	drainCh(notifyCh)

	// Re-Add same service
	err = state.AddService(&structs.NodeService{
		Service: "web",
		Port:    4444,
	}, "fake-token-web")
	require.NoError(err)

	// Should have a notification
	assert.NotEmpty(notifyCh)
	drainCh(notifyCh)

	// Remove service
	require.NoError(state.RemoveService(structs.NewServiceID("web", nil)))

	// Should have a notification
	assert.NotEmpty(notifyCh)
	drainCh(notifyCh)

	// Stopping should... stop
	state.StopNotify(notifyCh)

	// Add a service
	err = state.AddService(&structs.NodeService{
		Service: "web",
	}, "fake-token-web")
	require.NoError(err)

	// Should NOT have a notification
	assert.Empty(notifyCh)
	drainCh(notifyCh)
}

// Test that alias check is updated after AddCheck, UpdateCheck, and RemoveCheck for the same service id
func TestAliasNotifications_local(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Register service with a failing TCP check
	svcID := "socat"
	srv := &structs.NodeService{
		ID:      svcID,
		Service: "echo",
		Tags:    []string{},
		Address: "127.0.0.10",
		Port:    8080,
	}
	a.State.AddService(srv, "")

	scID := "socat-sidecar-proxy"
	sc := &structs.NodeService{
		ID:      scID,
		Service: scID,
		Tags:    []string{},
		Address: "127.0.0.10",
		Port:    9090,
	}
	a.State.AddService(sc, "")

	tcpID := types.CheckID("service:socat-tcp")
	chk0 := &structs.HealthCheck{
		Node:      "",
		CheckID:   tcpID,
		Name:      "tcp check",
		Status:    api.HealthPassing,
		ServiceID: svcID,
	}
	a.State.AddCheck(chk0, "")

	// Register an alias for the service
	proxyID := types.CheckID("service:socat-sidecar-proxy:2")
	chk1 := &structs.HealthCheck{
		Node:      "",
		CheckID:   proxyID,
		Name:      "Connect Sidecar Aliasing socat",
		Status:    api.HealthPassing,
		ServiceID: scID,
	}
	chkt := &structs.CheckType{
		AliasService: svcID,
	}
	require.NoError(t, a.AddCheck(chk1, chkt, true, "", agent.ConfigSourceLocal))

	// Add a failing check to the same service ID, alias should also fail
	maintID := types.CheckID("service:socat-maintenance")
	chk2 := &structs.HealthCheck{
		Node:      "",
		CheckID:   maintID,
		Name:      "socat:Service Maintenance Mode",
		Status:    api.HealthCritical,
		ServiceID: svcID,
	}
	a.State.AddCheck(chk2, "")

	retry.Run(t, func(r *retry.R) {
		check := a.State.Check(structs.NewCheckID(proxyID, nil))
		require.NotNil(r, check)
		require.Equal(r, api.HealthCritical, check.Status)
	})

	// Remove the failing check, alias should pass
	a.State.RemoveCheck(structs.NewCheckID(maintID, nil))

	retry.Run(t, func(r *retry.R) {
		check := a.State.Check(structs.NewCheckID(proxyID, nil))
		require.NotNil(r, check)
		require.Equal(r, api.HealthPassing, check.Status)
	})

	// Update TCP check to failing, alias should fail
	a.State.UpdateCheck(structs.NewCheckID(tcpID, nil), api.HealthCritical, "")

	retry.Run(t, func(r *retry.R) {
		check := a.State.Check(structs.NewCheckID(proxyID, nil))
		require.NotNil(r, check)
		require.Equal(r, api.HealthCritical, check.Status)
	})
}

// drainCh drains a channel by reading messages until it would block.
func drainCh(ch chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
