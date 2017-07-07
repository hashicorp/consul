package agent

import (
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
)

func TestAgentAntiEntropy_Services(t *testing.T) {
	t.Parallel()
	a := &TestAgent{Name: t.Name(), NoInitialSync: true}
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
	a.state.AddService(srv1, "")
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
	a.state.AddService(srv2, "")

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
	a.state.AddService(srv3, "")

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
	a.state.AddService(srv5, "")

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
	a.state.AddService(srv6, "")

	// todo(fs): data race
	a.state.Lock()
	a.state.serviceStatus["cache"] = syncStatus{inSync: true}
	a.state.Unlock()

	// Trigger anti-entropy run and wait
	a.StartSync()

	var services structs.IndexedNodeServices
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}

	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			r.Fatalf("err: %v", err)
		}

		// Make sure we sent along our node info when we synced.
		id := services.NodeServices.Node.ID
		addrs := services.NodeServices.Node.TaggedAddresses
		meta := services.NodeServices.Node.Meta
		if id != a.Config.NodeID ||
			!reflect.DeepEqual(addrs, a.Config.TaggedAddresses) ||
			!reflect.DeepEqual(meta, a.Config.Meta) {
			r.Fatalf("bad: %v", services.NodeServices.Node)
		}

		// We should have 6 services (consul included)
		if len(services.NodeServices.Services) != 6 {
			r.Fatalf("bad: %v", services.NodeServices.Services)
		}

		// All the services should match
		for id, serv := range services.NodeServices.Services {
			serv.CreateIndex, serv.ModifyIndex = 0, 0
			switch id {
			case "mysql":
				if !reflect.DeepEqual(serv, srv1) {
					r.Fatalf("bad: %v %v", serv, srv1)
				}
			case "redis":
				if !reflect.DeepEqual(serv, srv2) {
					r.Fatalf("bad: %#v %#v", serv, srv2)
				}
			case "web":
				if !reflect.DeepEqual(serv, srv3) {
					r.Fatalf("bad: %v %v", serv, srv3)
				}
			case "api":
				if !reflect.DeepEqual(serv, srv5) {
					r.Fatalf("bad: %v %v", serv, srv5)
				}
			case "cache":
				if !reflect.DeepEqual(serv, srv6) {
					r.Fatalf("bad: %v %v", serv, srv6)
				}
			case "consul":
				// ignore
			default:
				r.Fatalf("unexpected service: %v", id)
			}
		}

		// todo(fs): data race
		a.state.RLock()
		defer a.state.RUnlock()

		// Check the local state
		if len(a.state.services) != 6 {
			r.Fatalf("bad: %v", a.state.services)
		}
		if len(a.state.serviceStatus) != 6 {
			r.Fatalf("bad: %v", a.state.serviceStatus)
		}
		for name, status := range a.state.serviceStatus {
			if !status.inSync {
				r.Fatalf("should be in sync: %v %v", name, status)
			}
		}
	})

	// Remove one of the services
	a.state.RemoveService("api")

	// Trigger anti-entropy run and wait
	a.StartSync()

	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			r.Fatalf("err: %v", err)
		}

		// We should have 5 services (consul included)
		if len(services.NodeServices.Services) != 5 {
			r.Fatalf("bad: %v", services.NodeServices.Services)
		}

		// All the services should match
		for id, serv := range services.NodeServices.Services {
			serv.CreateIndex, serv.ModifyIndex = 0, 0
			switch id {
			case "mysql":
				if !reflect.DeepEqual(serv, srv1) {
					r.Fatalf("bad: %v %v", serv, srv1)
				}
			case "redis":
				if !reflect.DeepEqual(serv, srv2) {
					r.Fatalf("bad: %#v %#v", serv, srv2)
				}
			case "web":
				if !reflect.DeepEqual(serv, srv3) {
					r.Fatalf("bad: %v %v", serv, srv3)
				}
			case "cache":
				if !reflect.DeepEqual(serv, srv6) {
					r.Fatalf("bad: %v %v", serv, srv6)
				}
			case "consul":
				// ignore
			default:
				r.Fatalf("unexpected service: %v", id)
			}
		}

		// todo(fs): data race
		a.state.RLock()
		defer a.state.RUnlock()

		// Check the local state
		if len(a.state.services) != 5 {
			r.Fatalf("bad: %v", a.state.services)
		}
		if len(a.state.serviceStatus) != 5 {
			r.Fatalf("bad: %v", a.state.serviceStatus)
		}
		for name, status := range a.state.serviceStatus {
			if !status.inSync {
				r.Fatalf("should be in sync: %v %v", name, status)
			}
		}
	})
}

func TestAgentAntiEntropy_EnableTagOverride(t *testing.T) {
	t.Parallel()
	a := &TestAgent{Name: t.Name(), NoInitialSync: true}
	a.Start()
	defer a.Shutdown()

	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
		Address:    "127.0.0.1",
	}
	var out struct{}

	// EnableTagOverride = true
	srv1 := &structs.NodeService{
		ID:                "svc_id1",
		Service:           "svc1",
		Tags:              []string{"tag1"},
		Port:              6100,
		EnableTagOverride: true,
	}
	a.state.AddService(srv1, "")
	srv1_mod := new(structs.NodeService)
	*srv1_mod = *srv1
	srv1_mod.Port = 7100
	srv1_mod.Tags = []string{"tag1_mod"}
	args.Service = srv1_mod
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// EnableTagOverride = false
	srv2 := &structs.NodeService{
		ID:                "svc_id2",
		Service:           "svc2",
		Tags:              []string{"tag2"},
		Port:              6200,
		EnableTagOverride: false,
	}
	a.state.AddService(srv2, "")
	srv2_mod := new(structs.NodeService)
	*srv2_mod = *srv2
	srv2_mod.Port = 7200
	srv2_mod.Tags = []string{"tag2_mod"}
	args.Service = srv2_mod
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Trigger anti-entropy run and wait
	a.StartSync()

	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}
	var services structs.IndexedNodeServices

	retry.Run(t, func(r *retry.R) {
		//	runtime.Gosched()
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			r.Fatalf("err: %v", err)
		}

		a.state.RLock()
		defer a.state.RUnlock()

		// All the services should match
		for id, serv := range services.NodeServices.Services {
			serv.CreateIndex, serv.ModifyIndex = 0, 0
			switch id {
			case "svc_id1":
				if serv.ID != "svc_id1" ||
					serv.Service != "svc1" ||
					serv.Port != 6100 ||
					!reflect.DeepEqual(serv.Tags, []string{"tag1_mod"}) {
					r.Fatalf("bad: %v %v", serv, srv1)
				}
			case "svc_id2":
				if serv.ID != "svc_id2" ||
					serv.Service != "svc2" ||
					serv.Port != 6200 ||
					!reflect.DeepEqual(serv.Tags, []string{"tag2"}) {
					r.Fatalf("bad: %v %v", serv, srv2)
				}
			case "consul":
				// ignore
			default:
				r.Fatalf("unexpected service: %v", id)
			}
		}

		// todo(fs): data race
		a.state.RLock()
		defer a.state.RUnlock()

		for name, status := range a.state.serviceStatus {
			if !status.inSync {
				r.Fatalf("should be in sync: %v %v", name, status)
			}
		}
	})
}

func TestAgentAntiEntropy_Services_WithChecks(t *testing.T) {
	t.Parallel()
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	{
		// Single check
		srv := &structs.NodeService{
			ID:      "mysql",
			Service: "mysql",
			Tags:    []string{"master"},
			Port:    5000,
		}
		a.state.AddService(srv, "")

		chk := &structs.HealthCheck{
			Node:      a.Config.NodeName,
			CheckID:   "mysql",
			Name:      "mysql",
			ServiceID: "mysql",
			Status:    api.HealthPassing,
		}
		a.state.AddCheck(chk, "")

		// todo(fs): data race
		func() {
			a.state.RLock()
			defer a.state.RUnlock()

			// Sync the service once
			if err := a.state.syncService("mysql"); err != nil {
				t.Fatalf("err: %s", err)
			}
		}()

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
		a.state.AddService(srv, "")

		chk1 := &structs.HealthCheck{
			Node:      a.Config.NodeName,
			CheckID:   "redis:1",
			Name:      "redis:1",
			ServiceID: "redis",
			Status:    api.HealthPassing,
		}
		a.state.AddCheck(chk1, "")

		chk2 := &structs.HealthCheck{
			Node:      a.Config.NodeName,
			CheckID:   "redis:2",
			Name:      "redis:2",
			ServiceID: "redis",
			Status:    api.HealthPassing,
		}
		a.state.AddCheck(chk2, "")

		// todo(fs): data race
		func() {
			a.state.RLock()
			defer a.state.RUnlock()

			// Sync the service once
			if err := a.state.syncService("redis"); err != nil {
				t.Fatalf("err: %s", err)
			}
		}()

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
	cfg := TestConfig()
	cfg.ACLDatacenter = "dc1"
	cfg.ACLMasterToken = "root"
	cfg.ACLDefaultPolicy = "deny"
	cfg.ACLEnforceVersion8 = Bool(true)
	a := &TestAgent{Name: t.Name(), Config: cfg, NoInitialSync: true}
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
	a.state.AddService(srv1, token)

	// Create service (allowed)
	srv2 := &structs.NodeService{
		ID:      "api",
		Service: "api",
		Tags:    []string{"foo"},
		Port:    5001,
	}
	a.state.AddService(srv2, token)

	// Trigger anti-entropy run and wait
	a.StartSync()
	time.Sleep(200 * time.Millisecond)

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
			case "consul":
				// ignore
			default:
				t.Fatalf("unexpected service: %v", id)
			}
		}

		// todo(fs): data race
		func() {
			a.state.RLock()
			defer a.state.RUnlock()

			// Check the local state
			if len(a.state.services) != 3 {
				t.Fatalf("bad: %v", a.state.services)
			}
			if len(a.state.serviceStatus) != 3 {
				t.Fatalf("bad: %v", a.state.serviceStatus)
			}
			for name, status := range a.state.serviceStatus {
				if !status.inSync {
					t.Fatalf("should be in sync: %v %v", name, status)
				}
			}
		}()
	}

	// Now remove the service and re-sync
	a.state.RemoveService("api")
	a.StartSync()
	time.Sleep(200 * time.Millisecond)

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
			case "consul":
				// ignore
			default:
				t.Fatalf("unexpected service: %v", id)
			}
		}

		// todo(fs): data race
		func() {
			a.state.RLock()
			defer a.state.RUnlock()

			// Check the local state
			if len(a.state.services) != 2 {
				t.Fatalf("bad: %v", a.state.services)
			}
			if len(a.state.serviceStatus) != 2 {
				t.Fatalf("bad: %v", a.state.serviceStatus)
			}
			for name, status := range a.state.serviceStatus {
				if !status.inSync {
					t.Fatalf("should be in sync: %v %v", name, status)
				}
			}
		}()
	}

	// Make sure the token got cleaned up.
	if token := a.state.ServiceToken("api"); token != "" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgentAntiEntropy_Checks(t *testing.T) {
	t.Parallel()
	a := &TestAgent{Name: t.Name(), NoInitialSync: true}
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
	a.state.AddCheck(chk1, "")
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
	a.state.AddCheck(chk2, "")

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
	a.state.AddCheck(chk3, "")

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
	a.state.AddCheck(chk5, "")

	// todo(fs): data race
	a.state.Lock()
	a.state.checkStatus["cache"] = syncStatus{inSync: true}
	a.state.Unlock()

	// Trigger anti-entropy run and wait
	a.StartSync()

	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}
	var checks structs.IndexedHealthChecks

	// Verify that we are in sync
	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
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
				if !reflect.DeepEqual(chk, chk1) {
					r.Fatalf("bad: %v %v", chk, chk1)
				}
			case "redis":
				if !reflect.DeepEqual(chk, chk2) {
					r.Fatalf("bad: %v %v", chk, chk2)
				}
			case "web":
				if !reflect.DeepEqual(chk, chk3) {
					r.Fatalf("bad: %v %v", chk, chk3)
				}
			case "cache":
				if !reflect.DeepEqual(chk, chk5) {
					r.Fatalf("bad: %v %v", chk, chk5)
				}
			case "serfHealth":
				// ignore
			default:
				r.Fatalf("unexpected check: %v", chk)
			}
		}
	})

	// todo(fs): data race
	func() {
		a.state.RLock()
		defer a.state.RUnlock()

		// Check the local state
		if len(a.state.checks) != 4 {
			t.Fatalf("bad: %v", a.state.checks)
		}
		if len(a.state.checkStatus) != 4 {
			t.Fatalf("bad: %v", a.state.checkStatus)
		}
		for name, status := range a.state.checkStatus {
			if !status.inSync {
				t.Fatalf("should be in sync: %v %v", name, status)
			}
		}
	}()

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
		if id != a.Config.NodeID ||
			!reflect.DeepEqual(addrs, a.Config.TaggedAddresses) ||
			!reflect.DeepEqual(meta, a.Config.Meta) {
			t.Fatalf("bad: %v", services.NodeServices.Node)
		}
	}

	// Remove one of the checks
	a.state.RemoveCheck("redis")

	// Trigger anti-entropy run and wait
	a.StartSync()

	// Verify that we are in sync
	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
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
				if !reflect.DeepEqual(chk, chk1) {
					r.Fatalf("bad: %v %v", chk, chk1)
				}
			case "web":
				if !reflect.DeepEqual(chk, chk3) {
					r.Fatalf("bad: %v %v", chk, chk3)
				}
			case "cache":
				if !reflect.DeepEqual(chk, chk5) {
					r.Fatalf("bad: %v %v", chk, chk5)
				}
			case "serfHealth":
				// ignore
			default:
				r.Fatalf("unexpected check: %v", chk)
			}
		}
	})

	// todo(fs): data race
	func() {
		a.state.RLock()
		defer a.state.RUnlock()

		// Check the local state
		if len(a.state.checks) != 3 {
			t.Fatalf("bad: %v", a.state.checks)
		}
		if len(a.state.checkStatus) != 3 {
			t.Fatalf("bad: %v", a.state.checkStatus)
		}
		for name, status := range a.state.checkStatus {
			if !status.inSync {
				t.Fatalf("should be in sync: %v %v", name, status)
			}
		}
	}()
}

func TestAgentAntiEntropy_Checks_ACLDeny(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.ACLDatacenter = "dc1"
	cfg.ACLMasterToken = "root"
	cfg.ACLDefaultPolicy = "deny"
	cfg.ACLEnforceVersion8 = Bool(true)
	a := &TestAgent{Name: t.Name(), Config: cfg, NoInitialSync: true}
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
	a.state.AddService(srv1, "root")
	srv2 := &structs.NodeService{
		ID:      "api",
		Service: "api",
		Tags:    []string{"foo"},
		Port:    5001,
	}
	a.state.AddService(srv2, "root")

	// Trigger anti-entropy run and wait
	a.StartSync()
	time.Sleep(200 * time.Millisecond)

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
			case "consul":
				// ignore
			default:
				t.Fatalf("unexpected service: %v", id)
			}
		}

		// todo(fs): data race
		func() {
			a.state.RLock()
			defer a.state.RUnlock()

			// Check the local state
			if len(a.state.services) != 3 {
				t.Fatalf("bad: %v", a.state.services)
			}
			if len(a.state.serviceStatus) != 3 {
				t.Fatalf("bad: %v", a.state.serviceStatus)
			}
			for name, status := range a.state.serviceStatus {
				if !status.inSync {
					t.Fatalf("should be in sync: %v %v", name, status)
				}
			}
		}()
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
	a.state.AddCheck(chk1, token)

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
	a.state.AddCheck(chk2, token)

	// Trigger anti-entropy run and wait.
	a.StartSync()
	time.Sleep(200 * time.Millisecond)

	// Verify that we are in sync
	retry.Run(t, func(r *retry.R) {
		req := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
			QueryOptions: structs.QueryOptions{
				Token: "root",
			},
		}
		var checks structs.IndexedHealthChecks
		if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
			r.Fatalf("err: %v", err)
		}

		// We should have 2 checks (serf included)
		if len(checks.HealthChecks) != 2 {
			r.Fatalf("bad: %v", checks)
		}

		// All the checks should match
		for _, chk := range checks.HealthChecks {
			chk.CreateIndex, chk.ModifyIndex = 0, 0
			switch chk.CheckID {
			case "mysql-check":
				t.Fatalf("should not be permitted")
			case "api-check":
				if !reflect.DeepEqual(chk, chk2) {
					r.Fatalf("bad: %v %v", chk, chk2)
				}
			case "serfHealth":
				// ignore
			default:
				r.Fatalf("unexpected check: %v", chk)
			}
		}
	})

	// todo(fs): data race
	func() {
		a.state.RLock()
		defer a.state.RUnlock()

		// Check the local state.
		if len(a.state.checks) != 2 {
			t.Fatalf("bad: %v", a.state.checks)
		}
		if len(a.state.checkStatus) != 2 {
			t.Fatalf("bad: %v", a.state.checkStatus)
		}
		for name, status := range a.state.checkStatus {
			if !status.inSync {
				t.Fatalf("should be in sync: %v %v", name, status)
			}
		}
	}()

	// Now delete the check and wait for sync.
	a.state.RemoveCheck("api-check")
	a.StartSync()
	time.Sleep(200 * time.Millisecond)
	// Verify that we are in sync
	retry.Run(t, func(r *retry.R) {
		req := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       a.Config.NodeName,
			QueryOptions: structs.QueryOptions{
				Token: "root",
			},
		}
		var checks structs.IndexedHealthChecks
		if err := a.RPC("Health.NodeChecks", &req, &checks); err != nil {
			r.Fatalf("err: %v", err)
		}

		// We should have 1 check (just serf)
		if len(checks.HealthChecks) != 1 {
			r.Fatalf("bad: %v", checks)
		}

		// All the checks should match
		for _, chk := range checks.HealthChecks {
			chk.CreateIndex, chk.ModifyIndex = 0, 0
			switch chk.CheckID {
			case "mysql-check":
				r.Fatalf("should not be permitted")
			case "api-check":
				r.Fatalf("should be deleted")
			case "serfHealth":
				// ignore
			default:
				r.Fatalf("unexpected check: %v", chk)
			}
		}
	})

	// todo(fs): data race
	func() {
		a.state.RLock()
		defer a.state.RUnlock()

		// Check the local state.
		if len(a.state.checks) != 1 {
			t.Fatalf("bad: %v", a.state.checks)
		}
		if len(a.state.checkStatus) != 1 {
			t.Fatalf("bad: %v", a.state.checkStatus)
		}
		for name, status := range a.state.checkStatus {
			if !status.inSync {
				t.Fatalf("should be in sync: %v %v", name, status)
			}
		}
	}()

	// Make sure the token got cleaned up.
	if token := a.state.CheckToken("api-check"); token != "" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgentAntiEntropy_Check_DeferSync(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.CheckUpdateInterval = 500 * time.Millisecond
	a := &TestAgent{Name: t.Name(), Config: cfg, NoInitialSync: true}
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
	a.state.AddCheck(check, "")

	// Trigger anti-entropy run and wait
	a.StartSync()

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
	a.state.UpdateCheck("web", api.HealthPassing, "output")

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
	// Wait for a deferred update
	retry.Run(t, func(r *retry.R) {
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
		Address:         a.Config.AdvertiseAddr,
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

	// Trigger anti-entropy run and wait.
	a.StartSync()
	time.Sleep(200 * time.Millisecond)

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
	a.state.UpdateCheck("web", api.HealthPassing, "deferred")

	// Trigger anti-entropy run and wait.
	a.StartSync()
	time.Sleep(200 * time.Millisecond)

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
	cfg := TestConfig()
	cfg.NodeID = types.NodeID("40e4a748-2192-161a-0510-9bf59fe950b5")
	cfg.Meta["somekey"] = "somevalue"
	a := &TestAgent{Name: t.Name(), Config: cfg, NoInitialSync: true}
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

	// Trigger anti-entropy run and wait
	a.StartSync()

	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       a.Config.NodeName,
	}
	var services structs.IndexedNodeServices
	// Wait for the sync
	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			r.Fatalf("err: %v", err)
		}

		// Make sure we synced our node info - this should have ridden on the
		// "consul" service sync
		id := services.NodeServices.Node.ID
		addrs := services.NodeServices.Node.TaggedAddresses
		meta := services.NodeServices.Node.Meta
		if id != cfg.NodeID ||
			!reflect.DeepEqual(addrs, cfg.TaggedAddresses) ||
			!reflect.DeepEqual(meta, cfg.Meta) {
			r.Fatalf("bad: %v", services.NodeServices.Node)
		}
	})

	// Blow away the catalog version of the node info
	if err := a.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Trigger anti-entropy run and wait
	a.StartSync()
	// Wait for the sync - this should have been a sync of just the node info
	retry.Run(t, func(r *retry.R) {
		if err := a.RPC("Catalog.NodeServices", &req, &services); err != nil {
			r.Fatalf("err: %v", err)
		}

		id := services.NodeServices.Node.ID
		addrs := services.NodeServices.Node.TaggedAddresses
		meta := services.NodeServices.Node.Meta
		if id != cfg.NodeID ||
			!reflect.DeepEqual(addrs, cfg.TaggedAddresses) ||
			!reflect.DeepEqual(meta, cfg.Meta) {
			r.Fatalf("bad: %v", services.NodeServices.Node)
		}
	})
}

func TestAgentAntiEntropy_deleteService_fails(t *testing.T) {
	t.Parallel()
	l := new(localState)

	// todo(fs): data race
	l.Lock()
	defer l.Unlock()
	if err := l.deleteService(""); err == nil {
		t.Fatalf("should have failed")
	}
}

func TestAgentAntiEntropy_deleteCheck_fails(t *testing.T) {
	t.Parallel()
	l := new(localState)

	// todo(fs): data race
	l.Lock()
	defer l.Unlock()
	if err := l.deleteCheck(""); err == nil {
		t.Fatalf("should have errored")
	}
}

func TestAgent_serviceTokens(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.ACLToken = "default"
	l := NewLocalState(cfg, nil)

	l.AddService(&structs.NodeService{
		ID: "redis",
	}, "")

	// Returns default when no token is set
	if token := l.ServiceToken("redis"); token != "default" {
		t.Fatalf("bad: %s", token)
	}

	// Returns configured token
	l.serviceTokens["redis"] = "abc123"
	if token := l.ServiceToken("redis"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}

	// Keeps token around for the delete
	l.RemoveService("redis")
	if token := l.ServiceToken("redis"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_checkTokens(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.ACLToken = "default"
	l := NewLocalState(cfg, nil)

	// Returns default when no token is set
	if token := l.CheckToken("mem"); token != "default" {
		t.Fatalf("bad: %s", token)
	}

	// Returns configured token
	l.checkTokens["mem"] = "abc123"
	if token := l.CheckToken("mem"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}

	// Keeps token around for the delete
	l.RemoveCheck("mem")
	if token := l.CheckToken("mem"); token != "abc123" {
		t.Fatalf("bad: %s", token)
	}
}

func TestAgent_checkCriticalTime(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	l := NewLocalState(cfg, nil)

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
	if checks := l.CriticalChecks(); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Set it to warning and make sure that doesn't show up as critical.
	l.UpdateCheck(checkID, api.HealthWarning, "")
	if checks := l.CriticalChecks(); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Fail the check and make sure the time looks reasonable.
	l.UpdateCheck(checkID, api.HealthCritical, "")
	if crit, ok := l.CriticalChecks()[checkID]; !ok {
		t.Fatalf("should have a critical check")
	} else if crit.CriticalFor > time.Millisecond {
		t.Fatalf("bad: %#v", crit)
	}

	// Wait a while, then fail it again and make sure the time keeps track
	// of the initial failure, and doesn't reset here.
	time.Sleep(50 * time.Millisecond)
	l.UpdateCheck(chk.CheckID, api.HealthCritical, "")
	if crit, ok := l.CriticalChecks()[checkID]; !ok {
		t.Fatalf("should have a critical check")
	} else if crit.CriticalFor < 25*time.Millisecond ||
		crit.CriticalFor > 75*time.Millisecond {
		t.Fatalf("bad: %#v", crit)
	}

	// Set it passing again.
	l.UpdateCheck(checkID, api.HealthPassing, "")
	if checks := l.CriticalChecks(); len(checks) > 0 {
		t.Fatalf("should not have any critical checks")
	}

	// Fail the check and make sure the time looks like it started again
	// from the latest failure, not the original one.
	l.UpdateCheck(checkID, api.HealthCritical, "")
	if crit, ok := l.CriticalChecks()[checkID]; !ok {
		t.Fatalf("should have a critical check")
	} else if crit.CriticalFor > time.Millisecond {
		t.Fatalf("bad: %#v", crit)
	}
}

func TestAgent_nestedPauseResume(t *testing.T) {
	t.Parallel()
	l := new(localState)
	if l.isPaused() != false {
		t.Fatal("localState should be unPaused after init")
	}
	l.Pause()
	if l.isPaused() != true {
		t.Fatal("localState should be Paused after first call to Pause()")
	}
	l.Pause()
	if l.isPaused() != true {
		t.Fatal("localState should STILL be Paused after second call to Pause()")
	}
	l.Resume()
	if l.isPaused() != true {
		t.Fatal("localState should STILL be Paused after FIRST call to Resume()")
	}
	l.Resume()
	if l.isPaused() != false {
		t.Fatal("localState should NOT be Paused after SECOND call to Resume()")
	}

	defer func() {
		err := recover()
		if err == nil {
			t.Fatal("unbalanced Resume() should cause a panic()")
		}
	}()
	l.Resume()
}

func TestAgent_sendCoordinate(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.SyncCoordinateRateTarget = 10.0 // updates/sec
	cfg.SyncCoordinateIntervalMin = 1 * time.Millisecond
	cfg.ConsulConfig.CoordinateUpdatePeriod = 100 * time.Millisecond
	cfg.ConsulConfig.CoordinateUpdateBatchSize = 10
	cfg.ConsulConfig.CoordinateUpdateMaxBatches = 1
	a := NewTestAgent(t.Name(), cfg)
	defer a.Shutdown()

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
