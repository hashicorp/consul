package agent

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
)

func TestAgentAntiEntropy_Services(t *testing.T) {
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	// Register info
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       agent.config.NodeName,
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
	agent.state.AddService(srv1)
	args.Service = srv1
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists both, different (update)
	srv2 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{},
		Port:    8000,
	}
	agent.state.AddService(srv2)

	srv2_mod := new(structs.NodeService)
	*srv2_mod = *srv2
	srv2_mod.Port = 9000
	args.Service = srv2_mod
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists local (create)
	srv3 := &structs.NodeService{
		ID:      "web",
		Service: "web",
		Tags:    []string{},
		Port:    80,
	}
	agent.state.AddService(srv3)

	// Exists remote (delete)
	srv4 := &structs.NodeService{
		ID:      "lb",
		Service: "lb",
		Tags:    []string{},
		Port:    443,
	}
	args.Service = srv4
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
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
	agent.state.AddService(srv5)

	srv5_mod := new(structs.NodeService)
	*srv5_mod = *srv5
	srv5_mod.Address = "127.0.0.1"
	args.Service = srv5_mod
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Trigger anti-entropy run and wait
	agent.StartSync()
	time.Sleep(200 * time.Millisecond)

	// Verify that we are in sync
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       agent.config.NodeName,
	}
	var services structs.IndexedNodeServices
	if err := agent.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should have 5 services (consul included)
	if len(services.NodeServices.Services) != 5 {
		t.Fatalf("bad: %v", services.NodeServices.Services)
	}

	// All the services should match
	for id, serv := range services.NodeServices.Services {
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
		case "consul":
			// ignore
		default:
			t.Fatalf("unexpected service: %v", id)
		}
	}

	// Check the local state
	if len(agent.state.services) != 5 {
		t.Fatalf("bad: %v", agent.state.services)
	}
	if len(agent.state.serviceStatus) != 5 {
		t.Fatalf("bad: %v", agent.state.serviceStatus)
	}
	for name, status := range agent.state.serviceStatus {
		if !status.inSync {
			t.Fatalf("should be in sync: %v %v", name, status)
		}
	}
}

func TestAgentAntiEntropy_Services_WithChecks(t *testing.T) {
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	{
		// Single check
		srv := &structs.NodeService{
			ID:      "mysql",
			Service: "mysql",
			Tags:    []string{"master"},
			Port:    5000,
		}
		agent.state.AddService(srv)

		chk := &structs.HealthCheck{
			Node:      agent.config.NodeName,
			CheckID:   "mysql",
			Name:      "mysql",
			ServiceID: "mysql",
			Status:    structs.HealthPassing,
		}
		agent.state.AddCheck(chk)

		// Sync the service once
		if err := agent.state.syncService("mysql"); err != nil {
			t.Fatalf("err: %s", err)
		}

		// We should have 2 services (consul included)
		svcReq := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       agent.config.NodeName,
		}
		var services structs.IndexedNodeServices
		if err := agent.RPC("Catalog.NodeServices", &svcReq, &services); err != nil {
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
		if err := agent.RPC("Health.ServiceChecks", &chkReq, &checks); err != nil {
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
		agent.state.AddService(srv)

		chk1 := &structs.HealthCheck{
			Node:      agent.config.NodeName,
			CheckID:   "redis:1",
			Name:      "redis:1",
			ServiceID: "redis",
			Status:    structs.HealthPassing,
		}
		agent.state.AddCheck(chk1)

		chk2 := &structs.HealthCheck{
			Node:      agent.config.NodeName,
			CheckID:   "redis:2",
			Name:      "redis:2",
			ServiceID: "redis",
			Status:    structs.HealthPassing,
		}
		agent.state.AddCheck(chk2)

		// Sync the service once
		if err := agent.state.syncService("redis"); err != nil {
			t.Fatalf("err: %s", err)
		}

		// We should have 3 services (consul included)
		svcReq := structs.NodeSpecificRequest{
			Datacenter: "dc1",
			Node:       agent.config.NodeName,
		}
		var services structs.IndexedNodeServices
		if err := agent.RPC("Catalog.NodeServices", &svcReq, &services); err != nil {
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
		if err := agent.RPC("Health.ServiceChecks", &chkReq, &checks); err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(checks.HealthChecks) != 2 {
			t.Fatalf("bad: %v", checks)
		}
	}
}

func TestAgentAntiEntropy_Services_ACLDeny(t *testing.T) {
	conf := nextConfig()
	conf.ACLDatacenter = "dc1"
	conf.ACLMasterToken = "root"
	conf.ACLDefaultPolicy = "deny"
	dir, agent := makeAgent(t, conf)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	// Create the ACL
	arg := structs.ACLRequest{
		Datacenter: "dc1",
		Op:         structs.ACLSet,
		ACL: structs.ACL{
			Name:  "User token",
			Type:  structs.ACLTypeClient,
			Rules: testRegisterRules,
		},
		WriteRequest: structs.WriteRequest{Token: "root"},
	}
	var out string
	if err := agent.RPC("ACL.Apply", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Update the agent ACL token, resume sync
	conf.ACLToken = out

	// Create service (Allowed)
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tags:    []string{"master"},
		Port:    5000,
	}
	agent.state.AddService(srv1)

	// Create service (Disallowed)
	srv2 := &structs.NodeService{
		ID:      "api",
		Service: "api",
		Tags:    []string{"foo"},
		Port:    5001,
	}
	agent.state.AddService(srv2)

	// Trigger anti-entropy run and wait
	agent.StartSync()
	time.Sleep(200 * time.Millisecond)

	// Verify that we are in sync
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       agent.config.NodeName,
	}
	var services structs.IndexedNodeServices
	if err := agent.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should have 2 services (consul included)
	if len(services.NodeServices.Services) != 2 {
		t.Fatalf("bad: %v", services.NodeServices.Services)
	}

	// All the services should match
	for id, serv := range services.NodeServices.Services {
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

	// Check the local state
	if len(agent.state.services) != 3 {
		t.Fatalf("bad: %v", agent.state.services)
	}
	if len(agent.state.serviceStatus) != 3 {
		t.Fatalf("bad: %v", agent.state.serviceStatus)
	}
	for name, status := range agent.state.serviceStatus {
		if !status.inSync {
			t.Fatalf("should be in sync: %v %v", name, status)
		}
	}
}

func TestAgentAntiEntropy_Checks(t *testing.T) {
	conf := nextConfig()
	dir, agent := makeAgent(t, conf)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	// Register info
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       agent.config.NodeName,
		Address:    "127.0.0.1",
	}

	// Exists both, same (noop)
	var out struct{}
	chk1 := &structs.HealthCheck{
		Node:    agent.config.NodeName,
		CheckID: "mysql",
		Name:    "mysql",
		Status:  structs.HealthPassing,
	}
	agent.state.AddCheck(chk1)
	args.Check = chk1
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists both, different (update)
	chk2 := &structs.HealthCheck{
		Node:    agent.config.NodeName,
		CheckID: "redis",
		Name:    "redis",
		Status:  structs.HealthPassing,
	}
	agent.state.AddCheck(chk2)

	chk2_mod := new(structs.HealthCheck)
	*chk2_mod = *chk2
	chk2_mod.Status = structs.HealthCritical
	args.Check = chk2_mod
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists local (create)
	chk3 := &structs.HealthCheck{
		Node:    agent.config.NodeName,
		CheckID: "web",
		Name:    "web",
		Status:  structs.HealthPassing,
	}
	agent.state.AddCheck(chk3)

	// Exists remote (delete)
	chk4 := &structs.HealthCheck{
		Node:    agent.config.NodeName,
		CheckID: "lb",
		Name:    "lb",
		Status:  structs.HealthPassing,
	}
	args.Check = chk4
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Trigger anti-entropy run and wait
	agent.StartSync()
	time.Sleep(200 * time.Millisecond)

	// Verify that we are in sync
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       agent.config.NodeName,
	}
	var checks structs.IndexedHealthChecks
	if err := agent.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should have 4 services (serf included)
	if len(checks.HealthChecks) != 4 {
		t.Fatalf("bad: %v", checks)
	}

	// All the checks should match
	for _, chk := range checks.HealthChecks {
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
		case "serfHealth":
			// ignore
		default:
			t.Fatalf("unexpected check: %v", chk)
		}
	}

	// Check the local state
	if len(agent.state.checks) != 3 {
		t.Fatalf("bad: %v", agent.state.checks)
	}
	if len(agent.state.checkStatus) != 3 {
		t.Fatalf("bad: %v", agent.state.checkStatus)
	}
	for name, status := range agent.state.checkStatus {
		if !status.inSync {
			t.Fatalf("should be in sync: %v %v", name, status)
		}
	}
}

func TestAgentAntiEntropy_Check_DeferSync(t *testing.T) {
	conf := nextConfig()
	conf.CheckUpdateInterval = 100 * time.Millisecond
	dir, agent := makeAgent(t, conf)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	// Create a check
	check := &structs.HealthCheck{
		Node:    agent.config.NodeName,
		CheckID: "web",
		Name:    "web",
		Status:  structs.HealthPassing,
		Output:  "",
	}
	agent.state.AddCheck(check)

	// Trigger anti-entropy run and wait
	agent.StartSync()
	time.Sleep(200 * time.Millisecond)

	// Verify that we are in sync
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       agent.config.NodeName,
	}
	var checks structs.IndexedHealthChecks
	if err := agent.RPC("Health.NodeChecks", &req, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Verify checks in place
	if len(checks.HealthChecks) != 2 {
		t.Fatalf("checks: %v", check)
	}

	// Update the check output! Should be deferred
	agent.state.UpdateCheck("web", structs.HealthPassing, "output")

	// Should not update for 100 milliseconds
	time.Sleep(50 * time.Millisecond)
	if err := agent.RPC("Health.NodeChecks", &req, &checks); err != nil {
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
	testutil.WaitForResult(func() (bool, error) {
		if err := agent.RPC("Health.NodeChecks", &req, &checks); err != nil {
			return false, err
		}

		// Verify updated
		for _, chk := range checks.HealthChecks {
			switch chk.CheckID {
			case "web":
				if chk.Output != "output" {
					return false, fmt.Errorf("no update: %v", chk)
				}
			}
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestAgentAntiEntory_deleteService_fails(t *testing.T) {
	l := new(localState)
	if err := l.deleteService(""); err == nil {
		t.Fatalf("should have failed")
	}
}

func TestAgentAntiEntropy_deleteCheck_fails(t *testing.T) {
	l := new(localState)
	if err := l.deleteCheck(""); err == nil {
		t.Fatalf("should have errored")
	}
}

var testRegisterRules = `
service "api" {
	policy = "write"
}
`
