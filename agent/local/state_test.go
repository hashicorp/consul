// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package local_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/types"
)

func unNilMap(in map[string]string) map[string]string {
	if in == nil {
		return make(map[string]string)
	}
	return in
}

func TestAgentAntiEntropy_Services(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, "")
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
		Tags:    []string{"primary"},
		Port:    5000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	assert.False(t, a.State.ServiceExists(structs.ServiceID{ID: srv1.ID}))
	a.State.AddServiceWithChecks(srv1, nil, "", false)
	assert.True(t, a.State.ServiceExists(structs.ServiceID{ID: srv1.ID}))
	args.Service = srv1
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv2, nil, "", false)

	srv2_mod := new(structs.NodeService)
	*srv2_mod = *srv2
	srv2_mod.Port = 9000
	args.Service = srv2_mod
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv3, nil, "", false)

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	args.Service = srv4
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv5, nil, "", false)

	srv5_mod := new(structs.NodeService)
	*srv5_mod = *srv5
	srv5_mod.Address = "127.0.0.1"
	args.Service = srv5_mod
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
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

	if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure we sent along our node info when we synced.
	id := services.NodeServices.Node.ID
	addrs := services.NodeServices.Node.TaggedAddresses
	meta := services.NodeServices.Node.Meta
	delete(meta, structs.MetaSegmentKey)    // Added later, not in config.
	delete(meta, structs.MetaConsulVersion) // Added later, not in config.
	assert.Equal(t, a.Config.NodeID, id)
	assert.Equal(t, a.Config.TaggedAddresses, addrs)
	assert.Equal(t, unNilMap(a.Config.NodeMeta), meta)

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

	if err := servicesInSync(a.State, 5, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
		t.Fatal(err)
	}

	// Remove one of the services
	a.State.RemoveService(structs.NewServiceID("api", nil))

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
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

	if err := servicesInSync(a.State, 4, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
		t.Fatal(err)
	}
}

func TestAgentAntiEntropy_Services_ConnectProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	clone := func(ns *structs.NodeService) *structs.NodeService {
		raw, err := copystructure.Copy(ns)
		require.NoError(t, err)
		return raw.(*structs.NodeService)
	}

	// Register node info
	var out struct{}

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv1, nil, "", false)
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		Service:    srv1,
	}, &out))

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv2, nil, "", false)

	srv2_mod := clone(srv2)
	srv2_mod.Port = 9000
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		Service:    srv2_mod,
	}, &out))

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv3, nil, "", false)

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	require.NoError(t, a.RPC(context.Background(), "Catalog.Register", &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
		Service:    srv4,
	}, &out))

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.SetServiceState(&local.ServiceState{
		Service: srv5,
		InSync:  true,
	})

	require.NoError(t, a.State.SyncFull())

	var services structs.IndexedNodeServices
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}
	require.NoError(t, a.RPC(context.Background(), "Catalog.NodeServices", &req, &services))

	// We should have 5 services (consul included)
	require.Len(t, services.NodeServices.Services, 5)

	// check that virtual ips have been set
	vips := make(map[string]struct{})
	serviceToVIP := make(map[string]string)
	for _, serv := range services.NodeServices.Services {
		if serv.TaggedAddresses != nil {
			serviceVIP := serv.TaggedAddresses[structs.TaggedAddressVirtualIP].Address
			require.NotEmpty(t, serviceVIP)
			vips[serviceVIP] = struct{}{}
			serviceToVIP[serv.ID] = serviceVIP
		}
	}
	require.Len(t, vips, 4)

	// Update our assertions for the tagged addresses.
	srv1.TaggedAddresses = map[string]structs.ServiceAddress{
		structs.TaggedAddressVirtualIP: {
			Address: serviceToVIP["mysql-proxy"],
			Port:    srv1.Port,
		},
	}
	srv2.TaggedAddresses = map[string]structs.ServiceAddress{
		structs.TaggedAddressVirtualIP: {
			Address: serviceToVIP["redis-proxy"],
			Port:    srv2.Port,
		},
	}
	srv3.TaggedAddresses = map[string]structs.ServiceAddress{
		structs.TaggedAddressVirtualIP: {
			Address: serviceToVIP["web-proxy"],
			Port:    srv3.Port,
		},
	}
	srv5.TaggedAddresses = map[string]structs.ServiceAddress{
		structs.TaggedAddressVirtualIP: {
			Address: serviceToVIP["cache-proxy"],
			Port:    srv5.Port,
		},
	}

	// All the services should match
	// Retry to mitigate data races between local and remote state
	retry.Run(t, func(r *retry.R) {
		require.NoError(r, a.State.SyncFull())
		for id, serv := range services.NodeServices.Services {
			serv.CreateIndex, serv.ModifyIndex = 0, 0
			switch id {
			case "mysql-proxy":
				require.Equal(r, srv1, serv)
			case "redis-proxy":
				require.Equal(r, srv2, serv)
			case "web-proxy":
				require.Equal(r, srv3, serv)
			case "cache-proxy":
				require.Equal(r, srv5, serv)
			case structs.ConsulServiceID:
				// ignore
			default:
				r.Fatalf("unexpected service: %v", id)
			}
		}
	})

	require.NoError(t, servicesInSync(a.State, 4, structs.DefaultEnterpriseMetaInDefaultPartition()))

	// Remove one of the services
	a.State.RemoveService(structs.NewServiceID("cache-proxy", nil))
	require.NoError(t, a.State.SyncFull())
	require.NoError(t, a.RPC(context.Background(), "Catalog.NodeServices", &req, &services))

	// We should have 4 services (consul included)
	require.Len(t, services.NodeServices.Services, 4)

	// All the services should match
	for id, serv := range services.NodeServices.Services {
		serv.CreateIndex, serv.ModifyIndex = 0, 0
		switch id {
		case "mysql-proxy":
			require.Equal(t, srv1, serv)
		case "redis-proxy":
			require.Equal(t, srv2, serv)
		case "web-proxy":
			require.Equal(t, srv3, serv)
		case structs.ConsulServiceID:
			// ignore
		default:
			t.Fatalf("unexpected service: %v", id)
		}
	}

	require.NoError(t, servicesInSync(a.State, 3, structs.DefaultEnterpriseMetaInDefaultPartition()))
}

func TestAgent_ServiceWatchCh(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// register a local service
	srv1 := &structs.NodeService{
		ID:      "svc_id1",
		Service: "svc1",
		Tags:    []string{"tag1"},
		Port:    6100,
	}
	require.NoError(t, a.State.AddServiceWithChecks(srv1, nil, "", false))

	verifyState := func(ss *local.ServiceState) {
		require.NotNil(t, ss)
		require.NotNil(t, ss.WatchCh)

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
		require.NoError(t, a.State.AddServiceWithChecks(srv2, nil, "", false))
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
		require.NoError(t, a.State.RemoveService(srv1.CompoundServiceID()))
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, "")
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv1, nil, "", false)

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv2, nil, "", false)

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
		if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
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
					EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
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

		if err := servicesInSync(a.State, 2, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
			r.Fatal(err)
		}
	})
}

func TestAgentAntiEntropy_Services_WithChecks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	{
		// Single check
		srv := &structs.NodeService{
			ID:      "mysql",
			Service: "mysql",
			Tags:    []string{"primary"},
			Port:    5000,
		}
		a.State.AddServiceWithChecks(srv, nil, "", false)

		chk := &structs.HealthCheck{
			Node:      a.Config.NodeName,
			CheckID:   "mysql",
			Name:      "mysql",
			ServiceID: "mysql",
			Status:    api.HealthPassing,
		}
		a.State.AddCheck(chk, "", false)

		if err := a.State.SyncFull(); err != nil {
			t.Fatal("sync failed: ", err)
		}

		// We should have 2 services (consul included)
		svcReq := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
		}
		var services structs.IndexedNodeServices
		if err := a.RPC(context.Background(), "Catalog.NodeServices", &svcReq, &services); err != nil {
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
		if err := a.RPC(context.Background(), "Health.ServiceChecks", &chkReq, &checks); err != nil {
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
			Tags:    []string{"primary"},
			Port:    5000,
		}
		a.State.AddServiceWithChecks(srv, nil, "", false)

		chk1 := &structs.HealthCheck{
			Node:      a.Config.NodeName,
			CheckID:   "redis:1",
			Name:      "redis:1",
			ServiceID: "redis",
			Status:    api.HealthPassing,
		}
		a.State.AddCheck(chk1, "", false)

		chk2 := &structs.HealthCheck{
			Node:      a.Config.NodeName,
			CheckID:   "redis:2",
			Name:      "redis:2",
			ServiceID: "redis",
			Status:    api.HealthPassing,
		}
		a.State.AddCheck(chk2, "", false)

		if err := a.State.SyncFull(); err != nil {
			t.Fatal("sync failed: ", err)
		}

		// We should have 3 services (consul included)
		svcReq := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
		}
		var services structs.IndexedNodeServices
		if err := a.RPC(context.Background(), "Catalog.NodeServices", &svcReq, &services); err != nil {
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
		if err := a.RPC(context.Background(), "Health.ServiceChecks", &chkReq, &checks); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(checks.HealthChecks) != 2 {
			t.Fatalf("bad: %v", checks)
		}
	}
}

var testRegisterRules = `
 service "api" {
 	policy = "write"
 }

 service "consul" {
 	policy = "write"
 }
 `

func TestAgentAntiEntropy_Services_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, `
		primary_datacenter = "dc1"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
			}
		}
	`)
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// The agent token is the only token used for deleteService.
	setAgentToken(t, a)

	token := createToken(t, a, testRegisterRules)

	// Create service (disallowed)
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"primary"},
		Port:    5000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv1, nil, token, false)

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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv2, nil, token, false)

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
		if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
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

		if err := servicesInSync(a.State, 2, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
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
		if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
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

		if err := servicesInSync(a.State, 1, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
			t.Fatal(err)
		}
	}

	// Make sure the token got cleaned up.
	if token := a.State.ServiceToken(structs.NewServiceID("api", nil)); token != "" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgentAntiEntropy_ConfigFileRegistrationToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	tokens := map[string]string{
		"api": "5ece2854-989a-4e7a-8145-4801c13350d5",
		"web": "b85e99b7-8d97-45a3-a175-5f33e167177b",
	}

	// Configure the agent with the config_file_service_registration token.
	agentConfig := fmt.Sprintf(`
		primary_datacenter = "dc1"

		acl {
			enabled = true
			default_policy = "deny"
			tokens {
				initial_management = "root"
				config_file_service_registration = "%s"
			}
		}
	`, tokens["api"])

	// We need separate files because we can't put multiple 'service' stanzas in one config string/file.
	dir := testutil.TempDir(t, "config")
	apiFile := filepath.Join(dir, "api.hcl")
	dbFile := filepath.Join(dir, "db.hcl")
	webFile := filepath.Join(dir, "web.hcl")

	// The "api" service and checks are able to register because the config_file_service_registration token
	// has service:write for the "api" service.
	require.NoError(t, os.WriteFile(apiFile, []byte(`
		service {
			name = "api"
			id = "api"

			check {
				id = "api inline check"
				status = "passing"
				ttl = "99999h"
			}
		}

		check {
			id = "api standalone check"
			status = "passing"
			service_id = "api"
			ttl = "99999h"
		}
	`), 0600))

	// The "db" service and check is unable to register because the config_file_service_registration token
	// does not have service:write for "db" and there are no inline tokens.
	require.NoError(t, os.WriteFile(dbFile, []byte(`
		service {
			name = "db"
			id = "db"
		}

		check {
			id = "db standalone check"
			service_id = "db"
			status = "passing"
			ttl = "99999h"
		}
	`), 0600))

	// The "web" service is able to register because the inline tokens have service:write for "web".
	// This tests that inline tokens take precedence over the config_file_service_registration token.
	require.NoError(t, os.WriteFile(webFile, []byte(fmt.Sprintf(`
		service {
			name = "web"
			id = "web"
			token = "%[1]s"
		}

		check {
			id = "web standalone check"
			service_id = "web"
			status = "passing"
			ttl = "99999h"
			token = "%[1]s"
		}
	`, tokens["web"])), 0600))

	a := agent.NewTestAgentWithConfigFile(t, agentConfig, []string{apiFile, dbFile, webFile})
	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	// Create the tokens referenced in the config files.
	for svc, secret := range tokens {
		req := structs.ACLTokenSetRequest{
			ACLToken: structs.ACLToken{
				SecretID:          secret,
				ServiceIdentities: []*structs.ACLServiceIdentity{{ServiceName: svc}},
			},
			WriteRequest: structs.WriteRequest{Token: "root"},
		}
		if err := a.RPC(context.Background(), "ACL.TokenSet", &req, &structs.ACLToken{}); err != nil {
			t.Fatalf("err: %v", err)
		}
	}

	// All services are added from files into local state.
	assert.True(t, a.State.ServiceExists(structs.ServiceID{ID: "api"}))
	assert.True(t, a.State.ServiceExists(structs.ServiceID{ID: "db"}))
	assert.True(t, a.State.ServiceExists(structs.ServiceID{ID: "web"}))

	// Sync services with the remote.
	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Validate which services were able to register.
	var services structs.IndexedNodeServices
	require.NoError(t, a.RPC(
		context.Background(),
		"Catalog.NodeServices",
		&structs.NodeSpecificRequest{
			Datacenter:   "dc1",
			Node:         a.Config.NodeName,
			QueryOptions: structs.QueryOptions{Token: "root"},
		},
		&services,
	))

	assert.Len(t, services.NodeServices.Services, 3)
	assert.Contains(t, services.NodeServices.Services, "api")
	assert.Contains(t, services.NodeServices.Services, "consul")
	assert.Contains(t, services.NodeServices.Services, "web")
	// No token with permission to register the "db" service.
	assert.NotContains(t, services.NodeServices.Services, "db")

	// Validate which checks were able to register.
	var checks structs.IndexedHealthChecks
	require.NoError(t, a.RPC(
		context.Background(),
		"Health.NodeChecks",
		&structs.NodeSpecificRequest{
			Datacenter:   "dc1",
			Node:         a.Config.NodeName,
			QueryOptions: structs.QueryOptions{Token: "root"},
		},
		&checks,
	))

	sort.Slice(checks.HealthChecks, func(i, j int) bool {
		return checks.HealthChecks[i].CheckID < checks.HealthChecks[j].CheckID
	})
	assert.Len(t, checks.HealthChecks, 4)
	assert.Equal(t, checks.HealthChecks[0].CheckID, types.CheckID("api inline check"))
	assert.Equal(t, checks.HealthChecks[1].CheckID, types.CheckID("api standalone check"))
	assert.Equal(t, checks.HealthChecks[2].CheckID, types.CheckID("serfHealth"))
	assert.Equal(t, checks.HealthChecks[3].CheckID, types.CheckID("web standalone check"))
}

type RPC interface {
	RPC(ctx context.Context, method string, args interface{}, reply interface{}) error
}

func createToken(t *testing.T, rpc RPC, policyRules string) string {
	t.Helper()

	uniqueId, err := uuid.GenerateUUID()
	require.NoError(t, err)
	policyName := "the-policy-" + uniqueId

	reqPolicy := structs.ACLPolicySetRequest{
		Datacenter: "dc1",
		Policy: structs.ACLPolicy{
			Name:  policyName,
			Rules: policyRules,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	err = rpc.RPC(context.Background(), "ACL.PolicySet", &reqPolicy, &structs.ACLPolicy{})
	require.NoError(t, err)

	token, err := uuid.GenerateUUID()
	require.NoError(t, err)

	reqToken := structs.ACLTokenSetRequest{
		Datacenter: "dc1",
		ACLToken: structs.ACLToken{
			SecretID: token,
			Policies: []structs.ACLTokenPolicyLink{{Name: policyName}},
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	err = rpc.RPC(context.Background(), "ACL.TokenSet", &reqToken, &structs.ACLToken{})
	require.NoError(t, err)
	return token
}

// setAgentToken sets the 'agent' token for this agent. It creates a new token
// with node:write for the agent's node name, and service:write for any
// service.
func setAgentToken(t *testing.T, a *agent.TestAgent) {
	var policy = fmt.Sprintf(`
	  node "%s" {
		policy = "write"
	  }
	  service_prefix "" {
		policy = "read"
	  }
	`, a.Config.NodeName)

	token := createToken(t, a, policy)

	_, err := a.Client().Agent().UpdateAgentACLToken(token, &api.WriteOptions{Token: "root"})
	if err != nil {
		t.Fatalf("setting agent token: %v", err)
	}
}

func TestAgentAntiEntropy_Checks(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, "")
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
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddCheck(chk1, "", false)
	args.Check = chk1
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists both, different (update)
	chk2 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "redis",
		Name:           "redis",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddCheck(chk2, "", false)

	chk2_mod := new(structs.HealthCheck)
	*chk2_mod = *chk2
	chk2_mod.Status = api.HealthCritical
	args.Check = chk2_mod
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists local (create)
	chk3 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "web",
		Name:           "web",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddCheck(chk3, "", false)

	// Exists remote (delete)
	chk4 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "lb",
		Name:           "lb",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	args.Check = chk4
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists local, in sync, remote missing (create)
	chk5 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "cache",
		Name:           "cache",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
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

	retry.Run(t, func(r *retry.R) {

		// Verify that we are in sync
		if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
			r.Fatalf("err: %v", err)
		}

		// We should have 5 checks (serf included)
		if len(checks.HealthChecks) != 5 {
			r.Fatalf("bad: %v", checks)
		}

		// All the checks should match
		for _, chk := range checks.HealthChecks {
			chk.CreateIndex, chk.ModifyIndex = 0, 0
			switch chk.CheckID {
			case "mysql":
				require.Equal(r, chk, chk1)
			case "redis":
				require.Equal(r, chk, chk2)
			case "web":
				require.Equal(r, chk, chk3)
			case "cache":
				require.Equal(r, chk, chk5)
			case "serfHealth":
				// ignore
			default:
				r.Fatalf("unexpected check: %v", chk)
			}
		}

		if err := checksInSync(a.State, 4, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
			r.Fatal(err)
		}
	})

	retry.Run(t, func(r *retry.R) {

		// Make sure we sent along our node info addresses when we synced.
		{
			req := structs.NodeSpecificRequest{
				Datacenter: "dc1",
				Node:       a.Config.NodeName,
			}
			var services structs.IndexedNodeServices
			if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
				r.Fatalf("err: %v", err)
			}

			id := services.NodeServices.Node.ID
			addrs := services.NodeServices.Node.TaggedAddresses
			meta := services.NodeServices.Node.Meta
			delete(meta, structs.MetaSegmentKey)    // Added later, not in config.
			delete(meta, structs.MetaConsulVersion) // Added later, not in config.
			assert.Equal(r, a.Config.NodeID, id)
			assert.Equal(r, a.Config.TaggedAddresses, addrs)
			assert.Equal(r, unNilMap(a.Config.NodeMeta), meta)
		}
	})
	retry.Run(t, func(r *retry.R) {

		// Remove one of the checks
		a.State.RemoveCheck(structs.NewCheckID("redis", nil))

		if err := a.State.SyncFull(); err != nil {
			r.Fatalf("err: %v", err)
		}

		// Verify that we are in sync
		if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
			r.Fatalf("err: %v", err)
		}

		// We should have 5 checks (serf included)
		if len(checks.HealthChecks) != 4 {
			r.Fatalf("bad: %v", checks)
		}

		// All the checks should match
		for _, chk := range checks.HealthChecks {
			chk.CreateIndex, chk.ModifyIndex = 0, 0
			switch chk.CheckID {
			case "mysql":
				require.Equal(r, chk1, chk)
			case "web":
				require.Equal(r, chk3, chk)
			case "cache":
				require.Equal(r, chk5, chk)
			case "serfHealth":
				// ignore
			default:
				r.Fatalf("unexpected check: %v", chk)
			}
		}

		if err := checksInSync(a.State, 3, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
			r.Fatal(err)
		}
	})
}

func TestAgentAntiEntropy_RemovingServiceAndCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, "")
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists remote (delete)
	chk := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		CheckID:        "lb",
		Name:           "lb",
		ServiceID:      svcID,
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}

	args.Check = chk
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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

	if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	// The consul service will still be registered
	if len(services.NodeServices.Services) != 1 {
		t.Fatalf("Expected all services to be deleted, got: %#v", services.NodeServices.Services)
	}

	var checks structs.IndexedHealthChecks
	// Verify that we are in sync
	if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	// The serfHealth check will still be here
	if len(checks.HealthChecks) != 1 {
		t.Fatalf("Expected the health check to be deleted, got: %#v", checks.HealthChecks)
	}
}

func TestAgentAntiEntropy_Checks_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dc := "dc1"
	a := &agent.TestAgent{HCL: `
		primary_datacenter = "` + dc + `"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
			}
		}
	`}
	if err := a.Start(t); err != nil {
		t.Fatal(err)
	}
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, dc)

	// The agent token is the only token used for deleteCheck.
	setAgentToken(t, a)

	token := createToken(t, a, testRegisterRules)

	// Create services using the root token
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"primary"},
		Port:    5000,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv1, nil, "root", false)
	srv2 := &structs.NodeService{
		ID:      "api",
		Service: "api",
		Tags:    []string{"foo"},
		Port:    5001,
		Weights: &structs.Weights{
			Passing: 1,
			Warning: 1,
		},
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddServiceWithChecks(srv2, nil, "root", false)

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
		if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
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

		if err := servicesInSync(a.State, 2, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
			t.Fatal(err)
		}
	}

	// This check won't be allowed.
	chk1 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		ServiceID:      "mysql",
		ServiceName:    "mysql",
		ServiceTags:    []string{"primary"},
		CheckID:        "mysql-check",
		Name:           "mysql",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddCheck(chk1, token, false)

	// This one will be allowed.
	chk2 := &structs.HealthCheck{
		Node:           a.Config.NodeName,
		ServiceID:      "api",
		ServiceName:    "api",
		ServiceTags:    []string{"foo"},
		CheckID:        "api-check",
		Name:           "api",
		Status:         api.HealthPassing,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	a.State.AddCheck(chk2, token, false)

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
	if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
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

	if err := checksInSync(a.State, 2, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
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
		if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
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

	if err := checksInSync(a.State, 1, structs.DefaultEnterpriseMetaInDefaultPartition()); err != nil {
		t.Fatal(err)
	}

	// Make sure the token got cleaned up.
	if token := a.State.CheckToken(structs.NewCheckID("api-check", nil)); token != "" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_UpdateCheck_DiscardOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, `
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
	if err := a.State.AddCheck(check, "", false); err != nil {
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
	t.Skip("TODO: flaky")
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := &agent.TestAgent{HCL: `
		check_update_interval = "500ms"
	`}
	if err := a.Start(t); err != nil {
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
	a.State.AddCheck(check, "", false)

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
		if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
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
		if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
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
	if err := a.RPC(context.Background(), "Catalog.Register", &reg, &out); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify that the output is out of sync.
	if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
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
	if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
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
	if err := a.RPC(context.Background(), "Catalog.Register", &reg, &out); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify that the output is out of sync.
	if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
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
	if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
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
		if err := a.RPC(context.Background(), "Health.NodeChecks", &req, &checks); err != nil {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	nodeID := types.NodeID("40e4a748-2192-161a-0510-9bf59fe950b5")
	nodeMeta := map[string]string{
		"somekey": "somevalue",
	}
	a := &agent.TestAgent{HCL: `
		node_id = "40e4a748-2192-161a-0510-9bf59fe950b5"
		node_meta {
			somekey = "somevalue"
		}
		locality {
			region = "us-west-1"
			zone = "us-west-1a"
		}`}
	if err := a.Start(t); err != nil {
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
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
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
	if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	id := services.NodeServices.Node.ID
	addrs := services.NodeServices.Node.TaggedAddresses
	meta := services.NodeServices.Node.Meta
	nodeLocality := services.NodeServices.Node.Locality
	delete(meta, structs.MetaSegmentKey)    // Added later, not in config.
	delete(meta, structs.MetaConsulVersion) // Added later, not in config.
	require.Equal(t, a.Config.NodeID, id)
	require.Equal(t, a.Config.TaggedAddresses, addrs)
	require.Equal(t, a.Config.StructLocality(), nodeLocality)
	require.Equal(t, unNilMap(a.Config.NodeMeta), meta)

	// Blow away the catalog version of the node info
	if err := a.RPC(context.Background(), "Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the sync - this should have been a sync of just the node info
	if err := a.RPC(context.Background(), "Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	{
		id := services.NodeServices.Node.ID
		addrs := services.NodeServices.Node.TaggedAddresses
		meta := services.NodeServices.Node.Meta
		nodeLocality := services.NodeServices.Node.Locality
		delete(meta, structs.MetaSegmentKey)    // Added later, not in config.
		delete(meta, structs.MetaConsulVersion) // Added later, not in config.
		require.Equal(t, nodeID, id)
		require.Equal(t, a.Config.TaggedAddresses, addrs)
		require.Equal(t, a.Config.StructLocality(), nodeLocality)
		require.Equal(t, nodeMeta, meta)
	}
}

func TestState_ServiceTokens(t *testing.T) {
	tokens := new(token.Store)
	cfg := loadRuntimeConfig(t, `bind_addr = "127.0.0.1" data_dir = "dummy" node_name = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, tokens)
	l.TriggerSyncChanges = func() {}

	id := structs.NewServiceID("redis", nil)

	t.Run("defaults to empty string", func(t *testing.T) {
		require.Equal(t, "", l.ServiceToken(id))
	})

	t.Run("empty string when there is no token", func(t *testing.T) {
		err := l.AddServiceWithChecks(&structs.NodeService{ID: "redis"}, nil, "", false)
		require.NoError(t, err)

		require.Equal(t, "", l.ServiceToken(id))
	})

	t.Run("returns configured token", func(t *testing.T) {
		err := l.AddServiceWithChecks(&structs.NodeService{ID: "redis"}, nil, "abc123", false)
		require.NoError(t, err)

		require.Equal(t, "abc123", l.ServiceToken(id))
	})

	t.Run("RemoveCheck keeps token around for the delete", func(t *testing.T) {
		err := l.RemoveService(structs.NewServiceID("redis", nil))
		require.NoError(t, err)

		require.Equal(t, "abc123", l.ServiceToken(id))
	})
}

func loadRuntimeConfig(t *testing.T, hcl string) *config.RuntimeConfig {
	t.Helper()
	result, err := config.Load(config.LoadOpts{HCL: []string{hcl}})
	require.NoError(t, err)
	require.Len(t, result.Warnings, 0)
	return result.RuntimeConfig
}

func TestState_CheckTokens(t *testing.T) {
	tokens := new(token.Store)
	cfg := loadRuntimeConfig(t, `bind_addr = "127.0.0.1" data_dir = "dummy" node_name = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, tokens)
	l.TriggerSyncChanges = func() {}

	id := structs.NewCheckID("mem", nil)

	t.Run("defaults to empty string", func(t *testing.T) {
		require.Equal(t, "", l.CheckToken(id))
	})

	t.Run("empty string when there is no token", func(t *testing.T) {
		err := l.AddCheck(&structs.HealthCheck{CheckID: "mem"}, "", false)
		require.NoError(t, err)

		require.Equal(t, "", l.CheckToken(id))
	})

	t.Run("returns configured token", func(t *testing.T) {
		err := l.AddCheck(&structs.HealthCheck{CheckID: "mem"}, "abc123", false)
		require.NoError(t, err)

		require.Equal(t, "abc123", l.CheckToken(id))
	})

	t.Run("RemoveCheck keeps token around for the delete", func(t *testing.T) {
		err := l.RemoveCheck(structs.NewCheckID("mem", nil))
		require.NoError(t, err)

		require.Equal(t, "abc123", l.CheckToken(id))
	})
}

func TestAgent_CheckCriticalTime(t *testing.T) {
	t.Parallel()
	cfg := loadRuntimeConfig(t, `bind_addr = "127.0.0.1" data_dir = "dummy" node_name = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, new(token.Store))
	l.TriggerSyncChanges = func() {}

	svc := &structs.NodeService{ID: "redis", Service: "redis", Port: 8000}
	l.AddServiceWithChecks(svc, nil, "", false)

	// Add a passing check and make sure it's not critical.
	checkID := types.CheckID("redis:1")
	chk := &structs.HealthCheck{
		Node:      "node",
		CheckID:   checkID,
		Name:      "redis:1",
		ServiceID: "redis",
		Status:    api.HealthPassing,
	}
	l.AddCheck(chk, "", false)
	if checks := l.CriticalCheckStates(structs.DefaultEnterpriseMetaInDefaultPartition()); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Set it to warning and make sure that doesn't show up as critical.
	l.UpdateCheck(structs.NewCheckID(checkID, nil), api.HealthWarning, "")
	if checks := l.CriticalCheckStates(structs.DefaultEnterpriseMetaInDefaultPartition()); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Fail the check and make sure the time looks reasonable.
	l.UpdateCheck(structs.NewCheckID(checkID, nil), api.HealthCritical, "")
	if c, ok := l.CriticalCheckStates(structs.DefaultEnterpriseMetaInDefaultPartition())[structs.NewCheckID(checkID, nil)]; !ok {
		t.Fatalf("should have a critical check")
	} else if c.CriticalFor() > time.Millisecond {
		t.Fatalf("bad: %#v, check was critical for %v", c, c.CriticalFor())
	}

	// Wait a while, then fail it again and make sure the time keeps track
	// of the initial failure, and doesn't reset here. Since we are sleeping for
	// 50ms the check should not be any less than that.
	time.Sleep(50 * time.Millisecond)
	l.UpdateCheck(chk.CompoundCheckID(), api.HealthCritical, "")
	if c, ok := l.CriticalCheckStates(structs.DefaultEnterpriseMetaInDefaultPartition())[structs.NewCheckID(checkID, nil)]; !ok {
		t.Fatalf("should have a critical check")
	} else if c.CriticalFor() < 50*time.Millisecond {
		t.Fatalf("bad: %#v, check was critical for %v", c, c.CriticalFor())
	}

	// Set it passing again.
	l.UpdateCheck(structs.NewCheckID(checkID, nil), api.HealthPassing, "")
	if checks := l.CriticalCheckStates(structs.DefaultEnterpriseMetaInDefaultPartition()); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Fail the check and make sure the time looks like it started again
	// from the latest failure, not the original one.
	l.UpdateCheck(structs.NewCheckID(checkID, nil), api.HealthCritical, "")
	if c, ok := l.CriticalCheckStates(structs.DefaultEnterpriseMetaInDefaultPartition())[structs.NewCheckID(checkID, nil)]; !ok {
		t.Fatalf("should have a critical check")
	} else if c.CriticalFor() > time.Millisecond {
		t.Fatalf("bad: %#v, check was critical for %v", c, c.CriticalFor())
	}
}

func TestAgent_AddCheckFailure(t *testing.T) {
	t.Parallel()
	cfg := loadRuntimeConfig(t, `bind_addr = "127.0.0.1" data_dir = "dummy" node_name = "dummy"`)
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
	wantErr := errors.New(`Check ID "redis:1" refers to non-existent service ID "redis"`)

	got := l.AddCheck(chk, "", false)
	require.Equal(t, wantErr, got)
}

func TestAgent_AliasCheck(t *testing.T) {
	t.Parallel()

	cfg := loadRuntimeConfig(t, `bind_addr = "127.0.0.1" data_dir = "dummy" node_name = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, new(token.Store))
	l.TriggerSyncChanges = func() {}

	// Add checks
	require.NoError(t, l.AddServiceWithChecks(&structs.NodeService{Service: "s1"}, nil, "", false))
	require.NoError(t, l.AddServiceWithChecks(&structs.NodeService{Service: "s2"}, nil, "", false))
	require.NoError(t, l.AddCheck(&structs.HealthCheck{CheckID: types.CheckID("c1"), ServiceID: "s1"}, "", false))
	require.NoError(t, l.AddCheck(&structs.HealthCheck{CheckID: types.CheckID("c2"), ServiceID: "s2"}, "", false))

	// Add an alias
	notifyCh := make(chan struct{}, 1)
	require.NoError(t, l.AddAliasCheck(structs.NewCheckID(types.CheckID("a1"), nil), structs.NewServiceID("s1", nil), notifyCh))

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

func TestAgent_AliasCheck_ServiceNotification(t *testing.T) {
	t.Parallel()

	cfg := loadRuntimeConfig(t, `bind_addr = "127.0.0.1" data_dir = "dummy" node_name = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, new(token.Store))
	l.TriggerSyncChanges = func() {}

	// Add an alias check for service s1
	notifyCh := make(chan struct{}, 1)
	require.NoError(t, l.AddAliasCheck(structs.NewCheckID(types.CheckID("a1"), nil), structs.NewServiceID("s1", nil), notifyCh))

	// Add aliased service, s1, and verify we get notified
	require.NoError(t, l.AddServiceWithChecks(&structs.NodeService{Service: "s1"}, nil, "", false))
	select {
	case <-notifyCh:
	default:
		t.Fatal("notify not received")
	}

	// Re-adding same service should not lead to a notification
	require.NoError(t, l.AddServiceWithChecks(&structs.NodeService{Service: "s1"}, nil, "", false))
	select {
	case <-notifyCh:
		t.Fatal("notify received")
	default:
	}

	// Add different service and verify we do not get notified
	require.NoError(t, l.AddServiceWithChecks(&structs.NodeService{Service: "s2"}, nil, "", false))
	select {
	case <-notifyCh:
		t.Fatal("notify received")
	default:
	}

	// Delete service and verify we get notified
	require.NoError(t, l.RemoveService(structs.NewServiceID("s1", nil)))
	select {
	case <-notifyCh:
	default:
		t.Fatal("notify not received")
	}

	// Delete different service and verify we do not get notified
	require.NoError(t, l.RemoveService(structs.NewServiceID("s2", nil)))
	select {
	case <-notifyCh:
		t.Fatal("notify received")
	default:
	}
}

func TestAgent_sendCoordinate(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.StartTestAgent(t, agent.TestAgent{Overrides: `
		sync_coordinate_interval_min = "1ms"
		sync_coordinate_rate_target = 10.0
		consul = {
			coordinate = {
				update_period = "100ms"
				update_batch_size = 10
				update_max_batches = 1
			}
		}
	`})
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
		if err := a.RPC(context.Background(), "Coordinate.ListNodes", &req, &reply); err != nil {
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

func servicesInSync(state *local.State, wantServices int, entMeta *acl.EnterpriseMeta) error {
	services := state.ServiceStates(entMeta)
	if got, want := len(services), wantServices; got != want {
		return fmt.Errorf("got %d services want %d", got, want)
	}
	for id, s := range services {
		if !s.InSync {
			return fmt.Errorf("service ID %q should be in sync %+v", id.String(), s)
		}
	}
	return nil
}

func checksInSync(state *local.State, wantChecks int, entMeta *acl.EnterpriseMeta) error {
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

func TestState_RemoveServiceErrorMessages(t *testing.T) {
	state := local.NewState(local.Config{}, hclog.New(nil), &token.Store{})

	// Stub state syncing
	state.TriggerSyncChanges = func() {}

	// Add 1 service
	err := state.AddServiceWithChecks(&structs.NodeService{
		ID:      "web-id",
		Service: "web-name",
	}, nil, "", false)
	require.NoError(t, err)

	// Attempt to remove service that doesn't exist
	sid := structs.NewServiceID("db", nil)
	err = state.RemoveService(sid)
	require.Contains(t, err.Error(), fmt.Sprintf(`Unknown service ID %q`, sid))

	// Attempt to remove service by name (which isn't valid)
	sid2 := structs.NewServiceID("web-name", nil)
	err = state.RemoveService(sid2)
	require.Contains(t, err.Error(), fmt.Sprintf(`Unknown service ID %q`, sid2))

	// Attempt to remove service by id (valid)
	err = state.RemoveService(structs.NewServiceID("web-id", nil))
	require.NoError(t, err)
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

	// Register a notifier
	notifyCh := make(chan struct{}, 1)
	state.Notify(notifyCh)
	defer state.StopNotify(notifyCh)
	assert.Empty(t, notifyCh)
	drainCh(notifyCh)

	// Add a service
	err := state.AddServiceWithChecks(&structs.NodeService{
		Service: "web",
	}, nil, "fake-token-web", false)
	require.NoError(t, err)

	// Should have a notification
	assert.NotEmpty(t, notifyCh)
	drainCh(notifyCh)

	// Re-Add same service
	err = state.AddServiceWithChecks(&structs.NodeService{
		Service: "web",
		Port:    4444,
	}, nil, "fake-token-web", false)
	require.NoError(t, err)

	// Should have a notification
	assert.NotEmpty(t, notifyCh)
	drainCh(notifyCh)

	// Remove service
	require.NoError(t, state.RemoveService(structs.NewServiceID("web", nil)))

	// Should have a notification
	assert.NotEmpty(t, notifyCh)
	drainCh(notifyCh)

	// Stopping should... stop
	state.StopNotify(notifyCh)

	// Add a service
	err = state.AddServiceWithChecks(&structs.NodeService{
		Service: "web",
	}, nil, "fake-token-web", false)
	require.NoError(t, err)

	// Should NOT have a notification
	assert.Empty(t, notifyCh)
	drainCh(notifyCh)
}

// Test that alias check is updated after AddCheck, UpdateCheck, and RemoveCheck for the same service id
func TestAliasNotifications_local(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, "")
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
	a.State.AddServiceWithChecks(srv, nil, "", false)

	scID := "socat-sidecar-proxy"
	sc := &structs.NodeService{
		ID:      scID,
		Service: scID,
		Tags:    []string{},
		Address: "127.0.0.10",
		Port:    9090,
	}
	a.State.AddServiceWithChecks(sc, nil, "", false)

	tcpID := types.CheckID("service:socat-tcp")
	chk0 := &structs.HealthCheck{
		Node:      "",
		CheckID:   tcpID,
		Name:      "tcp check",
		Status:    api.HealthPassing,
		ServiceID: svcID,
	}
	a.State.AddCheck(chk0, "", false)

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
	require.NoError(t, a.AddCheck(chk1, chkt, true, "", agent.ConfigSourceLocal), false)

	// Add a failing check to the same service ID, alias should also fail
	maintID := types.CheckID("service:socat-maintenance")
	chk2 := &structs.HealthCheck{
		Node:      "",
		CheckID:   maintID,
		Name:      "socat:Service Maintenance Mode",
		Status:    api.HealthCritical,
		ServiceID: svcID,
	}
	a.State.AddCheck(chk2, "", false)

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

func TestState_SyncChanges_DuplicateAddServiceOnlySyncsOnce(t *testing.T) {
	state := local.NewState(local.Config{}, hclog.New(nil), new(token.Store))
	rpc := &fakeRPC{}
	state.Delegate = rpc
	state.TriggerSyncChanges = func() {}

	srv := &structs.NodeService{
		Kind:           structs.ServiceKindTypical,
		ID:             "the-service-id",
		Service:        "web",
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInDefaultPartition(),
	}
	checks := []*structs.HealthCheck{
		{Node: "this-node", CheckID: "the-id-1", Name: "check-healthy-1"},
		{Node: "this-node", CheckID: "the-id-2", Name: "check-healthy-2"},
	}
	tok := "the-token"
	err := state.AddServiceWithChecks(srv, checks, tok, false)
	require.NoError(t, err)
	require.NoError(t, state.SyncChanges())
	// 4 rpc calls, one node register, one service register, two checks
	require.Len(t, rpc.calls, 4)

	// adding the service again should not catalog register
	err = state.AddServiceWithChecks(srv, checks, tok, false)
	require.NoError(t, err)
	require.NoError(t, state.SyncChanges())
	require.Len(t, rpc.calls, 4)
}

type fakeRPC struct {
	calls []callRPC
}

type callRPC struct {
	method string
	args   interface{}
	reply  interface{}
}

func (f *fakeRPC) RPC(ctx context.Context, method string, args interface{}, reply interface{}) error {
	f.calls = append(f.calls, callRPC{method: method, args: args, reply: reply})
	return nil
}

func (f *fakeRPC) ResolveTokenAndDefaultMeta(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) (resolver.Result, error) {
	return resolver.Result{}, nil
}
