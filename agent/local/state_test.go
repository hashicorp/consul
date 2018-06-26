package local_test

import (
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/go-memdb"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/pascaldekloe/goe/verify"
	"github.com/stretchr/testify/assert"
)

func TestAgentAntiEntropy_Services(t *testing.T) {
	t.Parallel()
	a := &agent.TestAgent{Name: t.Name()}
	a.Start()
	defer a.Shutdown()

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
	}
	a.State.AddService(srv3, "")

	// Exists remote (delete)
	srv4 := &structs.NodeService{
		ID:      "lb",
		Service: "lb",
		Tags:    []string{},
		Port:    443,
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
	verify.Values(t, "node id", id, a.Config.NodeID)
	verify.Values(t, "tagged addrs", addrs, a.Config.TaggedAddresses)
	verify.Values(t, "node meta", meta, a.Config.NodeMeta)

	// We should have 6 services (consul included)
	if len(services.NodeServices.Services) != 6 {
		t.Fatalf("bad: %v", services.NodeServices.Services)
	}

	// All the services should match
	for id, serv := range services.NodeServices.Services {
		serv.CreateIndex, serv.ModifyIndex = 0, 0
		switch id {
		case "mysql":
			if !reflect.DeepEqual(serv, srv1) {
				t.Fatalf("bad: %v %v", serv, srv1)
			}
		case "redis":
			if !reflect.DeepEqual(serv, srv2) {
				t.Fatalf("bad: %#v %#v", serv, srv2)
			}
		case "web":
			if !reflect.DeepEqual(serv, srv3) {
				t.Fatalf("bad: %v %v", serv, srv3)
			}
		case "api":
			if !reflect.DeepEqual(serv, srv5) {
				t.Fatalf("bad: %v %v", serv, srv5)
			}
		case "cache":
			if !reflect.DeepEqual(serv, srv6) {
				t.Fatalf("bad: %v %v", serv, srv6)
			}
		case structs.ConsulServiceID:
			// ignore
		default:
			t.Fatalf("unexpected service: %v", id)
		}
	}

	if err := servicesInSync(a.State, 5); err != nil {
		t.Fatal(err)
	}

	// Remove one of the services
	a.State.RemoveService("api")

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
			if !reflect.DeepEqual(serv, srv1) {
				t.Fatalf("bad: %v %v", serv, srv1)
			}
		case "redis":
			if !reflect.DeepEqual(serv, srv2) {
				t.Fatalf("bad: %#v %#v", serv, srv2)
			}
		case "web":
			if !reflect.DeepEqual(serv, srv3) {
				t.Fatalf("bad: %v %v", serv, srv3)
			}
		case "cache":
			if !reflect.DeepEqual(serv, srv6) {
				t.Fatalf("bad: %v %v", serv, srv6)
			}
		case structs.ConsulServiceID:
			// ignore
		default:
			t.Fatalf("unexpected service: %v", id)
		}
	}

	if err := servicesInSync(a.State, 4); err != nil {
		t.Fatal(err)
	}
}

func TestAgentAntiEntropy_Services_ConnectProxy(t *testing.T) {
	t.Parallel()

	assert := assert.New(t)
	a := &agent.TestAgent{Name: t.Name()}
	a.Start()
	defer a.Shutdown()

	// Register node info
	var out struct{}
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
	}

	// Exists both same (noop)
	srv1 := &structs.NodeService{
		Kind:             structs.ServiceKindConnectProxy,
		ID:               "mysql-proxy",
		Service:          "mysql-proxy",
		Port:             5000,
		ProxyDestination: "db",
	}
	a.State.AddService(srv1, "")
	args.Service = srv1
	assert.Nil(a.RPC("Catalog.Register", args, &out))

	// Exists both, different (update)
	srv2 := &structs.NodeService{
		ID:               "redis-proxy",
		Service:          "redis-proxy",
		Port:             8000,
		Kind:             structs.ServiceKindConnectProxy,
		ProxyDestination: "redis",
	}
	a.State.AddService(srv2, "")

	srv2_mod := new(structs.NodeService)
	*srv2_mod = *srv2
	srv2_mod.Port = 9000
	args.Service = srv2_mod
	assert.Nil(a.RPC("Catalog.Register", args, &out))

	// Exists local (create)
	srv3 := &structs.NodeService{
		ID:               "web-proxy",
		Service:          "web-proxy",
		Port:             80,
		Kind:             structs.ServiceKindConnectProxy,
		ProxyDestination: "web",
	}
	a.State.AddService(srv3, "")

	// Exists remote (delete)
	srv4 := &structs.NodeService{
		ID:               "lb-proxy",
		Service:          "lb-proxy",
		Port:             443,
		Kind:             structs.ServiceKindConnectProxy,
		ProxyDestination: "lb",
	}
	args.Service = srv4
	assert.Nil(a.RPC("Catalog.Register", args, &out))

	// Exists local, in sync, remote missing (create)
	srv5 := &structs.NodeService{
		ID:               "cache-proxy",
		Service:          "cache-proxy",
		Port:             11211,
		Kind:             structs.ServiceKindConnectProxy,
		ProxyDestination: "cache-proxy",
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

	assert.Nil(servicesInSync(a.State, 4))

	// Remove one of the services
	a.State.RemoveService("cache-proxy")
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

	assert.Nil(servicesInSync(a.State, 3))
}

func TestAgentAntiEntropy_EnableTagOverride(t *testing.T) {
	t.Parallel()
	a := &agent.TestAgent{Name: t.Name()}
	a.Start()
	defer a.Shutdown()

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
	}
	a.State.AddService(srv1, "")

	// register a local service with tag override disabled
	srv2 := &structs.NodeService{
		ID:                "svc_id2",
		Service:           "svc2",
		Tags:              []string{"tag2"},
		Port:              6200,
		EnableTagOverride: false,
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

	if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
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
			}
			if !verify.Values(t, "", got, want) {
				t.FailNow()
			}
		case "svc_id2":
			got, want := serv, srv2
			if !verify.Values(t, "", got, want) {
				t.FailNow()
			}
		case structs.ConsulServiceID:
			// ignore
		default:
			t.Fatalf("unexpected service: %v", id)
		}
	}

	if err := servicesInSync(a.State, 2); err != nil {
		t.Fatal(err)
	}
}

func TestAgentAntiEntropy_Services_WithChecks(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), "")
	defer a.Shutdown()

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
	a := &agent.TestAgent{Name: t.Name(), HCL: `
		acl_datacenter = "dc1"
		acl_master_token = "root"
		acl_default_policy = "deny"
		acl_enforce_version_8 = true`}
	a.Start()
	defer a.Shutdown()

	// Create the ACL
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTypeClient,
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
	}
	a.State.AddService(srv1, token)

	// Create service (allowed)
	srv2 := &structs.NodeService{
		ID:      "api",
		Service: "api",
		Tags:    []string{"foo"},
		Port:    5001,
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
				if !reflect.DeepEqual(serv, srv2) {
					t.Fatalf("bad: %#v %#v", serv, srv2)
				}
			case structs.ConsulServiceID:
				// ignore
			default:
				t.Fatalf("unexpected service: %v", id)
			}
		}

		if err := servicesInSync(a.State, 2); err != nil {
			t.Fatal(err)
		}
	}

	// Now remove the service and re-sync
	a.State.RemoveService("api")
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

		if err := servicesInSync(a.State, 1); err != nil {
			t.Fatal(err)
		}
	}

	// Make sure the token got cleaned up.
	if token := a.State.ServiceToken("api"); token != "" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgentAntiEntropy_Checks(t *testing.T) {
	t.Parallel()
	a := &agent.TestAgent{Name: t.Name()}
	a.Start()
	defer a.Shutdown()

	// Register info
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
	}

	// Exists both, same (noop)
	var out struct{}
	chk1 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "mysql",
		Name:    "mysql",
		Status:  api.HealthPassing,
	}
	a.State.AddCheck(chk1, "")
	args.Check = chk1
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists both, different (update)
	chk2 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "redis",
		Name:    "redis",
		Status:  api.HealthPassing,
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
		Node:    a.Config.NodeName,
		CheckID: "web",
		Name:    "web",
		Status:  api.HealthPassing,
	}
	a.State.AddCheck(chk3, "")

	// Exists remote (delete)
	chk4 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "lb",
		Name:    "lb",
		Status:  api.HealthPassing,
	}
	args.Check = chk4
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists local, in sync, remote missing (create)
	chk5 := &structs.HealthCheck{
		Node:    a.Config.NodeName,
		CheckID: "cache",
		Name:    "cache",
		Status:  api.HealthPassing,
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
			if !reflect.DeepEqual(chk, chk1) {
				t.Fatalf("bad: %v %v", chk, chk1)
			}
		case "redis":
			if !reflect.DeepEqual(chk, chk2) {
				t.Fatalf("bad: %v %v", chk, chk2)
			}
		case "web":
			if !reflect.DeepEqual(chk, chk3) {
				t.Fatalf("bad: %v %v", chk, chk3)
			}
		case "cache":
			if !reflect.DeepEqual(chk, chk5) {
				t.Fatalf("bad: %v %v", chk, chk5)
			}
		case "serfHealth":
			// ignore
		default:
			t.Fatalf("unexpected check: %v", chk)
		}
	}

	if err := checksInSync(a.State, 4); err != nil {
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
		verify.Values(t, "node id", id, a.Config.NodeID)
		verify.Values(t, "tagged addrs", addrs, a.Config.TaggedAddresses)
		verify.Values(t, "node meta", meta, a.Config.NodeMeta)
	}

	// Remove one of the checks
	a.State.RemoveCheck("redis")

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
			if !reflect.DeepEqual(chk, chk1) {
				t.Fatalf("bad: %v %v", chk, chk1)
			}
		case "web":
			if !reflect.DeepEqual(chk, chk3) {
				t.Fatalf("bad: %v %v", chk, chk3)
			}
		case "cache":
			if !reflect.DeepEqual(chk, chk5) {
				t.Fatalf("bad: %v %v", chk, chk5)
			}
		case "serfHealth":
			// ignore
		default:
			t.Fatalf("unexpected check: %v", chk)
		}
	}

	if err := checksInSync(a.State, 3); err != nil {
		t.Fatal(err)
	}
}

func TestAgentAntiEntropy_Checks_ACLDeny(t *testing.T) {
	t.Parallel()
	a := &agent.TestAgent{Name: t.Name(), HCL: `
		acl_datacenter = "dc1"
		acl_master_token = "root"
		acl_default_policy = "deny"
		acl_enforce_version_8 = true`}
	a.Start()
	defer a.Shutdown()

	// Create the ACL
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTypeClient,
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
	}
	a.State.AddService(srv1, "root")
	srv2 := &structs.NodeService{
		ID:      "api",
		Service: "api",
		Tags:    []string{"foo"},
		Port:    5001,
	}
	a.State.AddService(srv2, "root")

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

		// We should have 3 services (consul included)
		if len(services.NodeServices.Services) != 3 {
			t.Fatalf("bad: %v", services.NodeServices.Services)
		}

		// All the services should match
		for id, serv := range services.NodeServices.Services {
			serv.CreateIndex, serv.ModifyIndex = 0, 0
			switch id {
			case "mysql":
				if !reflect.DeepEqual(serv, srv1) {
					t.Fatalf("bad: %#v %#v", serv, srv1)
				}
			case "api":
				if !reflect.DeepEqual(serv, srv2) {
					t.Fatalf("bad: %#v %#v", serv, srv2)
				}
			case structs.ConsulServiceID:
				// ignore
			default:
				t.Fatalf("unexpected service: %v", id)
			}
		}

		if err := servicesInSync(a.State, 2); err != nil {
			t.Fatal(err)
		}
	}

	// This check won't be allowed.
	chk1 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		ServiceID:   "mysql",
		ServiceName: "mysql",
		ServiceTags: []string{"master"},
		CheckID:     "mysql-check",
		Name:        "mysql",
		Status:      api.HealthPassing,
	}
	a.State.AddCheck(chk1, token)

	// This one will be allowed.
	chk2 := &structs.HealthCheck{
		Node:        a.Config.NodeName,
		ServiceID:   "api",
		ServiceName: "api",
		ServiceTags: []string{"foo"},
		CheckID:     "api-check",
		Name:        "api",
		Status:      api.HealthPassing,
	}
	a.State.AddCheck(chk2, token)

	if err := a.State.SyncFull(); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify that we are in sync
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
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
			if !reflect.DeepEqual(chk, chk2) {
				t.Fatalf("bad: %v %v", chk, chk2)
			}
		case "serfHealth":
			// ignore
		default:
			t.Fatalf("unexpected check: %v", chk)
		}
	}

	if err := checksInSync(a.State, 2); err != nil {
		t.Fatal(err)
	}

	// Now delete the check and wait for sync.
	a.State.RemoveCheck("api-check")
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

	if err := checksInSync(a.State, 1); err != nil {
		t.Fatal(err)
	}

	// Make sure the token got cleaned up.
	if token := a.State.CheckToken("api-check"); token != "" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_UpdateCheck_DiscardOutput(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), `
		discard_check_output = true
		check_update_interval = "0s" # set to "0s" since otherwise output checks are deferred
	`)
	defer a.Shutdown()

	inSync := func(id string) bool {
		s := a.State.CheckState(types.CheckID(id))
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
	a.State.UpdateCheck(check.CheckID, api.HealthPassing, "second output")
	if !inSync("web") {
		t.Fatal("check should be in sync")
	}

	// disable discarding of check output and update the check again with different
	// output. Then the check should be out of sync.
	a.State.SetDiscardCheckOutput(false)
	a.State.UpdateCheck(check.CheckID, api.HealthPassing, "third output")
	if inSync("web") {
		t.Fatal("check should be out of sync")
	}
}

func TestAgentAntiEntropy_Check_DeferSync(t *testing.T) {
	t.Parallel()
	a := &agent.TestAgent{Name: t.Name(), HCL: `
		check_update_interval = "500ms"
	`}
	a.Start()
	defer a.Shutdown()

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
	a.State.UpdateCheck("web", api.HealthPassing, "output")

	// Should not update for 500 milliseconds
	time.Sleep(250 * time.Millisecond)
	if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify not updated
	for _, chk := range checks.HealthChecks {
		switch chk.CheckID {
		case "web":
			if chk.Output != "" {
				t.Fatalf("early update: %v", chk)
			}
		}
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
	timer := &retry.Timer{Timeout: 6 * time.Second, Wait: 100 * time.Millisecond}
	retry.RunWith(timer, t, func(r *retry.R) {
		if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
			r.Fatal(err)
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
	a.State.UpdateCheck("web", api.HealthPassing, "deferred")

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
	a.Start()
	defer a.Shutdown()

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
	if id != a.Config.NodeID ||
		!reflect.DeepEqual(addrs, a.Config.TaggedAddresses) ||
		!reflect.DeepEqual(meta, a.Config.NodeMeta) {
		t.Fatalf("bad: %v", services.NodeServices.Node)
	}

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
		if id != nodeID ||
			!reflect.DeepEqual(addrs, a.Config.TaggedAddresses) ||
			!reflect.DeepEqual(meta, nodeMeta) {
			t.Fatalf("bad: %v", services.NodeServices.Node)
		}
	}
}

func TestAgent_ServiceTokens(t *testing.T) {
	t.Parallel()

	tokens := new(token.Store)
	tokens.UpdateUserToken("default")
	cfg := config.DefaultRuntimeConfig(`bind_addr = "127.0.0.1" data_dir = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, tokens)
	l.TriggerSyncChanges = func() {}

	l.AddService(&structs.NodeService{ID: "redis"}, "")

	// Returns default when no token is set
	if token := l.ServiceToken("redis"); token != "default" {
		t.Fatalf("bad: %s", token)
	}

	// Returns configured token
	l.AddService(&structs.NodeService{ID: "redis"}, "abc123")
	if token := l.ServiceToken("redis"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}

	// Keeps token around for the delete
	l.RemoveService("redis")
	if token := l.ServiceToken("redis"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_CheckTokens(t *testing.T) {
	t.Parallel()

	tokens := new(token.Store)
	tokens.UpdateUserToken("default")
	cfg := config.DefaultRuntimeConfig(`bind_addr = "127.0.0.1" data_dir = "dummy"`)
	l := local.NewState(agent.LocalConfig(cfg), nil, tokens)
	l.TriggerSyncChanges = func() {}

	// Returns default when no token is set
	l.AddCheck(&structs.HealthCheck{CheckID: types.CheckID("mem")}, "")
	if token := l.CheckToken("mem"); token != "default" {
		t.Fatalf("bad: %s", token)
	}

	// Returns configured token
	l.AddCheck(&structs.HealthCheck{CheckID: types.CheckID("mem")}, "abc123")
	if token := l.CheckToken("mem"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}

	// Keeps token around for the delete
	l.RemoveCheck("mem")
	if token := l.CheckToken("mem"); token != "abc123" {
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
	if checks := l.CriticalCheckStates(); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Set it to warning and make sure that doesn't show up as critical.
	l.UpdateCheck(checkID, api.HealthWarning, "")
	if checks := l.CriticalCheckStates(); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Fail the check and make sure the time looks reasonable.
	l.UpdateCheck(checkID, api.HealthCritical, "")
	if c, ok := l.CriticalCheckStates()[checkID]; !ok {
		t.Fatalf("should have a critical check")
	} else if c.CriticalFor() > time.Millisecond {
		t.Fatalf("bad: %#v", c)
	}

	// Wait a while, then fail it again and make sure the time keeps track
	// of the initial failure, and doesn't reset here.
	time.Sleep(50 * time.Millisecond)
	l.UpdateCheck(chk.CheckID, api.HealthCritical, "")
	if c, ok := l.CriticalCheckStates()[checkID]; !ok {
		t.Fatalf("should have a critical check")
	} else if c.CriticalFor() < 25*time.Millisecond ||
		c.CriticalFor() > 75*time.Millisecond {
		t.Fatalf("bad: %#v", c)
	}

	// Set it passing again.
	l.UpdateCheck(checkID, api.HealthPassing, "")
	if checks := l.CriticalCheckStates(); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Fail the check and make sure the time looks like it started again
	// from the latest failure, not the original one.
	l.UpdateCheck(checkID, api.HealthCritical, "")
	if c, ok := l.CriticalCheckStates()[checkID]; !ok {
		t.Fatalf("should have a critical check")
	} else if c.CriticalFor() > time.Millisecond {
		t.Fatalf("bad: %#v", c)
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
	if got, want := l.AddCheck(chk, ""), wantErr; !reflect.DeepEqual(got, want) {
		t.Fatalf("got error %q want %q", got, want)
	}
}

func TestAgent_sendCoordinate(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), `
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

func servicesInSync(state *local.State, wantServices int) error {
	services := state.ServiceStates()
	if got, want := len(services), wantServices; got != want {
		return fmt.Errorf("got %d services want %d", got, want)
	}
	for id, s := range services {
		if !s.InSync {
			return fmt.Errorf("service %q should be in sync", id)
		}
	}
	return nil
}

func checksInSync(state *local.State, wantChecks int) error {
	checks := state.CheckStates()
	if got, want := len(checks), wantChecks; got != want {
		return fmt.Errorf("got %d checks want %d", got, want)
	}
	for id, c := range checks {
		if !c.InSync {
			return fmt.Errorf("check %q should be in sync", id)
		}
	}
	return nil
}

func TestStateProxyManagement(t *testing.T) {
	t.Parallel()

	state := local.NewState(local.Config{
		ProxyBindMinPort: 20000,
		ProxyBindMaxPort: 20001,
	}, log.New(os.Stderr, "", log.LstdFlags), &token.Store{})

	// Stub state syncing
	state.TriggerSyncChanges = func() {}

	p1 := structs.ConnectManagedProxy{
		ExecMode:        structs.ProxyExecModeDaemon,
		Command:         []string{"consul", "connect", "proxy"},
		TargetServiceID: "web",
	}

	require := require.New(t)
	assert := assert.New(t)

	_, err := state.AddProxy(&p1, "fake-token", "")
	require.Error(err, "should fail as the target service isn't registered")

	// Sanity check done, lets add a couple of target services to the state
	err = state.AddService(&structs.NodeService{
		Service: "web",
	}, "fake-token-web")
	require.NoError(err)
	err = state.AddService(&structs.NodeService{
		Service: "cache",
	}, "fake-token-cache")
	require.NoError(err)
	require.NoError(err)
	err = state.AddService(&structs.NodeService{
		Service: "db",
	}, "fake-token-db")
	require.NoError(err)

	// Should work now
	pstate, err := state.AddProxy(&p1, "fake-token", "")
	require.NoError(err)

	svc := pstate.Proxy.ProxyService
	assert.Equal("web-proxy", svc.ID)
	assert.Equal("web-proxy", svc.Service)
	assert.Equal(structs.ServiceKindConnectProxy, svc.Kind)
	assert.Equal("web", svc.ProxyDestination)
	assert.Equal("", svc.Address, "should have empty address by default")
	// Port is non-deterministic but could be either of 20000 or 20001
	assert.Contains([]int{20000, 20001}, svc.Port)

	{
		// Re-registering same proxy again should not pick a random port but re-use
		// the assigned one. It should also keep the same proxy token since we don't
		// want to force restart for config change.
		pstateDup, err := state.AddProxy(&p1, "fake-token", "")
		require.NoError(err)
		svcDup := pstateDup.Proxy.ProxyService

		assert.Equal("web-proxy", svcDup.ID)
		assert.Equal("web-proxy", svcDup.Service)
		assert.Equal(structs.ServiceKindConnectProxy, svcDup.Kind)
		assert.Equal("web", svcDup.ProxyDestination)
		assert.Equal("", svcDup.Address, "should have empty address by default")
		// Port must be same as before
		assert.Equal(svc.Port, svcDup.Port)
		// Same ProxyToken
		assert.Equal(pstate.ProxyToken, pstateDup.ProxyToken)
	}

	// Let's register a notifier now
	notifyCh := make(chan struct{}, 1)
	state.NotifyProxy(notifyCh)
	defer state.StopNotifyProxy(notifyCh)
	assert.Empty(notifyCh)
	drainCh(notifyCh)

	// Second proxy should claim other port
	p2 := p1
	p2.TargetServiceID = "cache"
	pstate2, err := state.AddProxy(&p2, "fake-token", "")
	require.NoError(err)
	svc2 := pstate2.Proxy.ProxyService
	assert.Contains([]int{20000, 20001}, svc2.Port)
	assert.NotEqual(svc.Port, svc2.Port)

	// Should have a notification
	assert.NotEmpty(notifyCh)
	drainCh(notifyCh)

	// Store this for later
	p2token := state.Proxy(svc2.ID).ProxyToken

	// Third proxy should fail as all ports are used
	p3 := p1
	p3.TargetServiceID = "db"
	_, err = state.AddProxy(&p3, "fake-token", "")
	require.Error(err)

	// Should have a notification but we'll do nothing so that the next
	// receive should block (we set cap == 1 above)

	// But if we set a port explicitly it should be OK
	p3.Config = map[string]interface{}{
		"bind_port":    1234,
		"bind_address": "0.0.0.0",
	}
	pstate3, err := state.AddProxy(&p3, "fake-token", "")
	require.NoError(err)
	svc3 := pstate3.Proxy.ProxyService
	require.Equal("0.0.0.0", svc3.Address)
	require.Equal(1234, svc3.Port)

	// Should have a notification
	assert.NotEmpty(notifyCh)
	drainCh(notifyCh)

	// Update config of an already registered proxy should work
	p3updated := p3
	p3updated.Config["foo"] = "bar"
	// Setup multiple watchers who should all witness the change
	gotP3 := state.Proxy(svc3.ID)
	require.NotNil(gotP3)
	var ws memdb.WatchSet
	ws.Add(gotP3.WatchCh)
	pstate3, err = state.AddProxy(&p3updated, "fake-token", "")
	require.NoError(err)
	svc3 = pstate3.Proxy.ProxyService
	require.Equal("0.0.0.0", svc3.Address)
	require.Equal(1234, svc3.Port)
	gotProxy3 := state.Proxy(svc3.ID)
	require.NotNil(gotProxy3)
	require.Equal(p3updated.Config, gotProxy3.Proxy.Config)
	assert.False(ws.Watch(time.After(500*time.Millisecond)),
		"watch should have fired so ws.Watch should not timeout")

	drainCh(notifyCh)

	// Remove one of the auto-assigned proxies
	_, err = state.RemoveProxy(svc2.ID)
	require.NoError(err)

	// Should have a notification
	assert.NotEmpty(notifyCh)
	drainCh(notifyCh)

	// Should be able to create a new proxy for that service with the port (it
	// should have been "freed").
	p4 := p2
	pstate4, err := state.AddProxy(&p4, "fake-token", "")
	require.NoError(err)
	svc4 := pstate4.Proxy.ProxyService
	assert.Contains([]int{20000, 20001}, svc2.Port)
	assert.Equal(svc4.Port, svc2.Port, "should get the same port back that we freed")

	// Remove a proxy that doesn't exist should error
	_, err = state.RemoveProxy("nope")
	require.Error(err)

	assert.Equal(&p4, state.Proxy(p4.ProxyService.ID).Proxy,
		"should fetch the right proxy details")
	assert.Nil(state.Proxy("nope"))

	proxies := state.Proxies()
	assert.Len(proxies, 3)
	assert.Equal(&p1, proxies[svc.ID].Proxy)
	assert.Equal(&p4, proxies[svc4.ID].Proxy)
	assert.Equal(&p3, proxies[svc3.ID].Proxy)

	tokens := make([]string, 4)
	tokens[0] = state.Proxy(svc.ID).ProxyToken
	// p2 not registered anymore but lets make sure p4 got a new token when it
	// re-registered with same ID.
	tokens[1] = p2token
	tokens[2] = state.Proxy(svc2.ID).ProxyToken
	tokens[3] = state.Proxy(svc3.ID).ProxyToken

	// Quick check all are distinct
	for i := 0; i < len(tokens)-1; i++ {
		assert.Len(tokens[i], 36) // Sanity check for UUIDish thing.
		for j := i + 1; j < len(tokens); j++ {
			assert.NotEqual(tokens[i], tokens[j], "tokens for proxy %d and %d match",
				i+1, j+1)
		}
	}
}

// Tests the logic for retaining tokens and ports through restore (i.e.
// proxy-service already restored and token passed in externally)
func TestStateProxyRestore(t *testing.T) {
	t.Parallel()

	state := local.NewState(local.Config{
		// Wide random range to make it very unlikely to pass by chance
		ProxyBindMinPort: 10000,
		ProxyBindMaxPort: 20000,
	}, log.New(os.Stderr, "", log.LstdFlags), &token.Store{})

	// Stub state syncing
	state.TriggerSyncChanges = func() {}

	webSvc := structs.NodeService{
		Service: "web",
	}

	p1 := structs.ConnectManagedProxy{
		ExecMode:        structs.ProxyExecModeDaemon,
		Command:         []string{"consul", "connect", "proxy"},
		TargetServiceID: "web",
	}

	p2 := p1

	require := require.New(t)
	assert := assert.New(t)

	// Add a target service
	require.NoError(state.AddService(&webSvc, "fake-token-web"))

	// Add the proxy for first time to get the proper service definition to
	// register
	pstate, err := state.AddProxy(&p1, "fake-token", "")
	require.NoError(err)

	// Now start again with a brand new state
	state2 := local.NewState(local.Config{
		// Wide random range to make it very unlikely to pass by chance
		ProxyBindMinPort: 10000,
		ProxyBindMaxPort: 20000,
	}, log.New(os.Stderr, "", log.LstdFlags), &token.Store{})

	// Stub state syncing
	state2.TriggerSyncChanges = func() {}

	// Register the target service
	require.NoError(state2.AddService(&webSvc, "fake-token-web"))

	// "Restore" the proxy service
	require.NoError(state.AddService(p1.ProxyService, "fake-token-web"))

	// Now we can AddProxy with the "restored" token
	pstate2, err := state.AddProxy(&p2, "fake-token", pstate.ProxyToken)
	require.NoError(err)

	// Check it still has the same port and token as before
	assert.Equal(pstate.ProxyToken, pstate2.ProxyToken)
	assert.Equal(p1.ProxyService.Port, p2.ProxyService.Port)
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
