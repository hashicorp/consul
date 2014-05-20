package agent

import (
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"os"
	"reflect"
	"testing"
	"time"
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
		Tags:    nil,
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
		Tags:    nil,
		Port:    80,
	}
	agent.state.AddService(srv3)

	// Exists remote (delete)
	srv4 := &structs.NodeService{
		ID:      "lb",
		Service: "lb",
		Tags:    nil,
		Port:    443,
	}
	args.Service = srv4
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Trigger anti-entropy run and wait
	agent.StartSync()
	time.Sleep(100 * time.Millisecond)

	// Verify that we are in sync
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       agent.config.NodeName,
	}
	var services structs.IndexedNodeServices
	if err := agent.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should have 4 services (consul included)
	if len(services.NodeServices.Services) != 4 {
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
				t.Fatalf("bad: %v %v", serv, srv2)
			}
		case "web":
			if !reflect.DeepEqual(serv, srv3) {
				t.Fatalf("bad: %v %v", serv, srv3)
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
	chk2_mod.Status = structs.HealthUnknown
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
	time.Sleep(100 * time.Millisecond)

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
