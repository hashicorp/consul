package agent

import (
	"github.com/hashicorp/consul/consul/structs"
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

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

	// Register info
	args := &structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       agent.config.NodeName,
		Address:    "127.0.0.1",
	}
	var out struct{}

	// Exists both, same (noop)
	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tag:     "master",
		Port:    5000,
	}
	agent.AddService(srv1)
	args.Service = srv1
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Exists both, different (update)
	srv2 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tag:     "",
		Port:    8000,
	}
	agent.AddService(srv2)

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
		Tag:     "",
		Port:    80,
	}
	agent.AddService(srv3)

	// Exists remote (delete)
	srv4 := &structs.NodeService{
		ID:      "lb",
		Service: "lb",
		Tag:     "",
		Port:    443,
	}
	args.Service = srv4
	if err := agent.RPC("Catalog.Register", args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Trigger anti-entropy run and wait
	agent.RegistrationDone()
	time.Sleep(100 * time.Millisecond)

	// Verify that we are in sync
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       agent.config.NodeName,
	}
	var services structs.NodeServices
	if err := agent.RPC("Catalog.NodeServices", &req, &services); err != nil {
		t.Fatalf("err: %v", err)
	}

	// We should have 4 services (consul included)
	if len(services.Services) != 4 {
		t.Fatalf("bad: %v", services.Services)
	}

	// All the services should match
	for id, serv := range services.Services {
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
